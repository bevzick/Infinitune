package processing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
)

const (
	failureTimeout    = 5 * time.Second
	maximumSourceSize = 500 * 1024 * 1024
)

type Config struct {
	PollInterval time.Duration
	RetryDelay   time.Duration
	StaleAfter   time.Duration
	JobTimeout   time.Duration
	MaxAttempts  int
}

type Worker struct {
	queue      Queue
	objects    ObjectStore
	transcoder Transcoder
	config     Config
	logger     *slog.Logger
}

func NewWorker(
	queue Queue,
	objects ObjectStore,
	transcoder Transcoder,
	config Config,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		queue: queue, objects: objects, transcoder: transcoder,
		config: config, logger: logger,
	}
}

func (worker *Worker) Run(ctx context.Context) {
	recovered, err := worker.queue.RecoverStaleProcessing(
		ctx, time.Now().Add(-worker.config.StaleAfter),
	)
	if err != nil {
		worker.logger.Error("failed to recover stale media processing jobs", "error", err)
	} else if recovered > 0 {
		worker.logger.Info("recovered stale media processing jobs", "count", recovered)
	}
	for {
		processed, processErr := worker.ProcessNext(ctx)
		if processErr != nil && !errors.Is(processErr, context.Canceled) {
			worker.logger.Error("media processing job failed", "error", processErr)
		}
		if ctx.Err() != nil {
			return
		}
		if processed {
			continue
		}
		timer := time.NewTimer(worker.config.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (worker *Worker) ProcessNext(ctx context.Context) (bool, error) {
	job, err := worker.queue.ClaimProcessing(ctx)
	if errors.Is(err, ErrNoPendingJob) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim media processing job: %w", err)
	}
	jobCtx, cancelJob := context.WithTimeout(ctx, worker.config.JobTimeout)
	err = worker.process(jobCtx, job)
	cancelJob()
	if err == nil {
		return true, nil
	}
	failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	retryAt := time.Now().Add(worker.retryDelay(job.Attempts))
	terminal, failErr := worker.queue.FailProcessing(
		failureCtx, job, boundedError(err), retryAt, worker.config.MaxAttempts,
	)
	if failErr != nil {
		return true, errors.Join(err, fmt.Errorf("record processing failure: %w", failErr))
	}
	if terminal {
		worker.logger.Warn("media asset processing exhausted retries",
			"media_asset_id", job.Asset.ID, "attempts", job.Attempts)
	}
	return true, err
}

func (worker *Worker) process(ctx context.Context, job Job) error {
	original, err := worker.objects.Open(ctx, job.Asset.OriginalObjectKey)
	if err != nil {
		return fmt.Errorf("open original audio: %w", err)
	}
	if original.Size < 1 || original.Size > maximumSourceSize {
		_ = original.Body.Close()
		return fmt.Errorf("original audio size %d is outside processing limits", original.Size)
	}
	result, transcodeErr := worker.transcoder.Transcode(ctx, original.Body)
	closeErr := original.Body.Close()
	if transcodeErr != nil {
		return errors.Join(fmt.Errorf("transcode original audio: %w", transcodeErr), closeErr)
	}
	defer result.Close()
	if closeErr != nil {
		return fmt.Errorf("close original audio: %w", closeErr)
	}
	if result.DurationMS <= 0 || len(result.Variants) == 0 {
		return fmt.Errorf("transcoder returned incomplete audio metadata")
	}

	variants := make([]media.AudioVariant, 0, len(result.Variants))
	uploadedKeys := make([]string, 0, len(result.Variants))
	for _, generated := range result.Variants {
		variantID := uuid.New()
		objectKey := variantObjectKey(job.Asset.ID, generated.BitrateKbps)
		if err = worker.objects.Put(ctx, objectKey, generated.Body,
			generated.SizeBytes, generated.ContentType); err != nil {
			cleanupErr := worker.deleteObjects(ctx, uploadedKeys)
			return errors.Join(fmt.Errorf("store audio variant: %w", err), cleanupErr)
		}
		uploadedKeys = append(uploadedKeys, objectKey)
		variants = append(variants, media.AudioVariant{
			ID: variantID, ObjectKey: objectKey, Codec: generated.Codec,
			Container: generated.Container, BitrateKbps: generated.BitrateKbps,
			SampleRate: generated.SampleRate, Channels: generated.Channels,
			SizeBytes: generated.SizeBytes,
		})
	}
	if err = worker.queue.CompleteProcessing(ctx, job, result.DurationMS, variants); err != nil {
		cleanupErr := worker.deleteObjects(ctx, uploadedKeys)
		return errors.Join(fmt.Errorf("complete media processing: %w", err), cleanupErr)
	}
	return nil
}

func (worker *Worker) deleteObjects(ctx context.Context, keys []string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	var result error
	for _, key := range keys {
		result = errors.Join(result, worker.objects.Delete(cleanupCtx, key))
	}
	return result
}

func (worker *Worker) retryDelay(attempt int) time.Duration {
	delay := worker.config.RetryDelay
	for current := 1; current < attempt && current < 5; current++ {
		delay *= 2
	}
	return delay
}

func variantObjectKey(assetID uuid.UUID, bitrateKbps int) string {
	return fmt.Sprintf("media/%s/variants/mp3-%d.mp3", assetID, bitrateKbps)
}

func boundedError(err error) string {
	const maximumLength = 2000
	message := err.Error()
	if len(message) > maximumLength {
		return message[:maximumLength]
	}
	return message
}
