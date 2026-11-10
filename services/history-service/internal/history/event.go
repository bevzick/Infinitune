package history

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	MinimumQualifiedListen     = 30 * time.Second
	CompletionThresholdPercent = 90
	RetentionMonths            = 24
)

var ErrEventConflict = errors.New("listening event id already belongs to different data")

type Event struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	TrackID     uuid.UUID
	StartedAt   time.Time
	FinishedAt  *time.Time
	ListenedMS  int
	Completed   bool
	ContextType string
	ContextID   *uuid.UUID
	DeviceType  string
}

// Qualifies reports whether validated playback counts as a listen. Short tracks
// qualify at half their duration; longer tracks qualify after 30 seconds.
func Qualifies(listenedMS, durationMS int) bool {
	if listenedMS < 0 || durationMS <= 0 {
		return false
	}
	threshold := int(MinimumQualifiedListen / time.Millisecond)
	if half := (durationMS + 1) / 2; half < threshold {
		threshold = half
	}
	return listenedMS >= threshold
}

func IsCompleted(listenedMS, durationMS int) bool {
	if listenedMS < 0 || durationMS <= 0 {
		return false
	}
	return int64(listenedMS)*100 >= int64(durationMS)*CompletionThresholdPercent
}
