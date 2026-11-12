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
		Topic: "history.listening.recorded.v1", Partition: 0, Offset: 9,
		Key: []byte("user-1"), Value: []byte("{invalid"),
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("history.listening.recorded.v1")}},
	}

	record := deadLetterRecord(original, "platform.events.dead-letter.v1", "recommendation-history-v1", errors.New("invalid payload"), failedAt)

	if record.Topic != "platform.events.dead-letter.v1" || string(record.Key) != string(original.Key) || string(record.Value) != string(original.Value) {
		t.Fatalf("record = %+v", record)
	}
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	want := map[string]string{
		"event_type": "history.listening.recorded.v1", "dlq_original_topic": original.Topic,
		"dlq_original_partition": "0", "dlq_original_offset": "9",
		"dlq_consumer_group": "recommendation-history-v1", "dlq_error": "invalid payload",
		"dlq_failed_at": failedAt.Format(time.RFC3339Nano),
	}
	for key, value := range want {
		if headers[key] != value {
			t.Errorf("header %s = %q, want %q", key, headers[key], value)
		}
	}
}
