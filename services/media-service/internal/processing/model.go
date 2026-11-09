package processing

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/google/uuid"
)

var ErrNoPendingJob = errors.New("no pending media processing job")

const JobTypeAudioVariants = "audio_variants"

type Job struct {
	ID       uuid.UUID
	Asset    media.Asset
	Attempts int
}

type GeneratedVariant struct {
	Body        io.ReadCloser
	Codec       string
	Container   string
	ContentType string
	BitrateKbps int
	SampleRate  int
	Channels    int
	SizeBytes   int64
}

type TranscodeResult struct {
	DurationMS int
	Variants   []GeneratedVariant
	close      func() error
}

func (result *TranscodeResult) Close() error {
	var err error
	for index := range result.Variants {
		err = errors.Join(err, result.Variants[index].Body.Close())
	}
	if result.close != nil {
		err = errors.Join(err, result.close())
	}
	return err
}

type Queue interface {
	EnqueueProcessing(context.Context, uuid.UUID) error
	ClaimProcessing(context.Context) (Job, error)
	CompleteProcessing(context.Context, Job, int, []media.AudioVariant) error
	FailProcessing(context.Context, Job, string, time.Time, int) (bool, error)
	RecoverStaleProcessing(context.Context, time.Time) (int64, error)
}

type ObjectStore interface {
	Open(context.Context, string) (storage.Object, error)
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Transcoder interface {
	Transcode(context.Context, io.Reader) (TranscodeResult, error)
}
