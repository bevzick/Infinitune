package processing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/google/uuid"
)

func TestWorkerProcessesAudioVariants(t *testing.T) {
	job := processingJob(1)
	queue := &queueStub{job: job}
	objects := &objectsStub{}
	transcoder := &transcoderStub{result: transcodeResult()}
	processed, err := newTestWorker(queue, objects, transcoder).ProcessNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessNext() = %v, %v", processed, err)
	}
	if queue.completedJob.ID != job.ID || queue.durationMS != 1250 ||
		len(queue.variants) != 2 || len(objects.puts) != 2 {
		t.Fatalf("completed = %+v, duration = %d, variants = %d, puts = %d",
			queue.completedJob, queue.durationMS, len(queue.variants), len(objects.puts))
	}
	if queue.variants[0].ObjectKey != variantObjectKey(job.Asset.ID, 128) ||
		queue.variants[1].ObjectKey != variantObjectKey(job.Asset.ID, 320) {
		t.Fatalf("variant keys = %q, %q",
			queue.variants[0].ObjectKey, queue.variants[1].ObjectKey)
	}
}

func TestWorkerSchedulesRetryAfterTranscodingFailure(t *testing.T) {
	job := processingJob(1)
	queue := &queueStub{job: job}
	processed, err := newTestWorker(queue, &objectsStub{},
		&transcoderStub{err: errors.New("invalid audio")}).ProcessNext(context.Background())
	if !processed || err == nil || queue.failedJob.ID != job.ID || queue.terminal {
		t.Fatalf("ProcessNext() = %v, %v; failed = %+v, terminal = %v",
			processed, err, queue.failedJob, queue.terminal)
	}
	if queue.retryAt.Before(time.Now().Add(9 * time.Second)) {
		t.Fatalf("retryAt = %v", queue.retryAt)
	}
}

func TestWorkerMarksFinalAttemptTerminal(t *testing.T) {
	job := processingJob(3)
	queue := &queueStub{job: job}
	_, err := newTestWorker(queue, &objectsStub{},
		&transcoderStub{err: errors.New("invalid audio")}).ProcessNext(context.Background())
	if err == nil || !queue.terminal {
		t.Fatalf("ProcessNext() error = %v, terminal = %v", err, queue.terminal)
	}
}

func TestWorkerDeletesUploadedVariantWhenLaterUploadFails(t *testing.T) {
	queue := &queueStub{job: processingJob(1)}
	objects := &objectsStub{putErrAt: 2}
	_, err := newTestWorker(queue, objects,
		&transcoderStub{result: transcodeResult()}).ProcessNext(context.Background())
	if err == nil || len(objects.deleted) != 1 || objects.deleted[0] != objects.puts[0] {
		t.Fatalf("ProcessNext() error = %v, puts = %v, deleted = %v",
			err, objects.puts, objects.deleted)
	}
}

func TestWorkerReturnsIdleWhenNoJobExists(t *testing.T) {
	processed, err := newTestWorker(&queueStub{claimErr: ErrNoPendingJob},
		&objectsStub{}, &transcoderStub{}).ProcessNext(context.Background())
	if err != nil || processed {
		t.Fatalf("ProcessNext() = %v, %v", processed, err)
	}
}

func newTestWorker(queue *queueStub, objects *objectsStub, transcoder *transcoderStub) *Worker {
	return NewWorker(queue, objects, transcoder, Config{
		PollInterval: time.Millisecond,
		RetryDelay:   10 * time.Second,
		StaleAfter:   time.Minute,
		JobTimeout:   time.Minute,
		MaxAttempts:  3,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func processingJob(attempts int) Job {
	return Job{
		ID: uuid.New(), Attempts: attempts,
		Asset: media.Asset{
			ID: uuid.New(), OriginalObjectKey: "creator/source.flac",
		},
	}
}

func transcodeResult() TranscodeResult {
	return TranscodeResult{
		DurationMS: 1250,
		Variants: []GeneratedVariant{
			{Body: io.NopCloser(bytes.NewBufferString("low")), Codec: "mp3",
				Container: "mp3", ContentType: "audio/mpeg", BitrateKbps: 128,
				SampleRate: 48000, Channels: 2, SizeBytes: 3},
			{Body: io.NopCloser(bytes.NewBufferString("high")), Codec: "mp3",
				Container: "mp3", ContentType: "audio/mpeg", BitrateKbps: 320,
				SampleRate: 48000, Channels: 2, SizeBytes: 4},
		},
	}
}

type queueStub struct {
	job          Job
	claimErr     error
	completedJob Job
	failedJob    Job
	durationMS   int
	variants     []media.AudioVariant
	retryAt      time.Time
	terminal     bool
}

func (*queueStub) EnqueueProcessing(context.Context, uuid.UUID) error { return nil }

func (stub *queueStub) ClaimProcessing(context.Context) (Job, error) {
	return stub.job, stub.claimErr
}

func (stub *queueStub) CompleteProcessing(
	_ context.Context,
	job Job,
	durationMS int,
	variants []media.AudioVariant,
) error {
	stub.completedJob, stub.durationMS, stub.variants = job, durationMS, variants
	return nil
}

func (stub *queueStub) FailProcessing(
	_ context.Context,
	job Job,
	_ string,
	retryAt time.Time,
	maxAttempts int,
) (bool, error) {
	stub.failedJob, stub.retryAt = job, retryAt
	stub.terminal = job.Attempts >= maxAttempts
	return stub.terminal, nil
}

func (*queueStub) RecoverStaleProcessing(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type objectsStub struct {
	puts     []string
	deleted  []string
	putErrAt int
}

func (*objectsStub) Open(context.Context, string) (storage.Object, error) {
	return storage.Object{
		Body: &readSeekCloser{Reader: bytes.NewReader([]byte("source"))}, Size: 6,
	}, nil
}

func (stub *objectsStub) Put(
	_ context.Context,
	key string,
	reader io.Reader,
	_ int64,
	_ string,
) error {
	stub.puts = append(stub.puts, key)
	_, _ = io.Copy(io.Discard, reader)
	if stub.putErrAt == len(stub.puts) {
		return errors.New("storage unavailable")
	}
	return nil
}

func (stub *objectsStub) Delete(_ context.Context, key string) error {
	stub.deleted = append(stub.deleted, key)
	return nil
}

type readSeekCloser struct{ *bytes.Reader }

func (*readSeekCloser) Close() error { return nil }

type transcoderStub struct {
	result TranscodeResult
	err    error
}

func (stub *transcoderStub) Transcode(context.Context, io.Reader) (TranscodeResult, error) {
	return stub.result, stub.err
}
