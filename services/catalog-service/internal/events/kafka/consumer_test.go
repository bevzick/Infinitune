package kafka

import (
	"errors"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestDeadLetterRecordPreservesSourceEventAndAddsFailureContext(t *testing.T) {
	failedAt := time.Date(2026, time.November, 12, 10, 30, 0, 123, time.UTC)
	original := &kgo.Record{
		Topic: "content.release.approved.v1", Partition: 1, Offset: 17,
		Key: []byte("release-1"), Value: []byte("{invalid"),
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("content.release.approved.v1")}},
	}

	record := deadLetterRecord(original, "platform.events.dead-letter.v1", "catalog-content-v1", errors.New("invalid payload"), failedAt)

	if record.Topic != "platform.events.dead-letter.v1" || string(record.Key) != string(original.Key) || string(record.Value) != string(original.Value) {
		t.Fatalf("record = %+v", record)
	}
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	want := map[string]string{
		"event_type": "content.release.approved.v1", "dlq_original_topic": original.Topic,
		"dlq_original_partition": "1", "dlq_original_offset": "17",
		"dlq_consumer_group": "catalog-content-v1", "dlq_error": "invalid payload",
		"dlq_failed_at": failedAt.Format(time.RFC3339Nano),
	}
	for key, value := range want {
		if headers[key] != value {
			t.Errorf("header %s = %q, want %q", key, headers[key], value)
		}
	}
}
