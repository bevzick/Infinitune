package kafka

import (
	"testing"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestRecordCorrelationIDPrefersCorrelationHeader(t *testing.T) {
	record := &kgo.Record{Headers: []kgo.RecordHeader{
		{Key: "event_id", Value: []byte(uuid.NewString())},
		{Key: "correlation_id", Value: []byte("request-42")},
	}}
	if got := recordCorrelationID(record); got != "request-42" {
		t.Fatalf("recordCorrelationID() = %q, want request-42", got)
	}
}

func TestRecordCorrelationIDFallsBackToEventID(t *testing.T) {
	eventID := uuid.NewString()
	record := &kgo.Record{Headers: []kgo.RecordHeader{
		{Key: "correlation_id", Value: []byte("unsafe correlation id")},
		{Key: "event_id", Value: []byte(eventID)},
	}}
	if got := recordCorrelationID(record); got != eventID {
		t.Fatalf("recordCorrelationID() = %q, want %q", got, eventID)
	}
}
