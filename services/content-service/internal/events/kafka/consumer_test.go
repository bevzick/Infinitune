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
		Topic: "media.asset.ready.v1", Partition: 2, Offset: 42,
		Key: []byte("asset-1"), Value: []byte("{invalid"),
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("media.asset.ready.v1")}},
	}

	record := deadLetterRecord(original, "platform.events.dead-letter.v1", "content-media-v1", errors.New("invalid payload"), failedAt)

	assertDeadLetterRecord(t, record, original, "content-media-v1", "invalid payload", failedAt)
}

func assertDeadLetterRecord(t *testing.T, record, original *kgo.Record, groupID, failure string, failedAt time.Time) {
	t.Helper()
	if record.Topic != "platform.events.dead-letter.v1" || string(record.Key) != string(original.Key) || string(record.Value) != string(original.Value) {
		t.Fatalf("record = %+v", record)
	}
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	want := map[string]string{
		"event_type": "media.asset.ready.v1", "dlq_original_topic": original.Topic,
		"dlq_original_partition": "2", "dlq_original_offset": "42",
		"dlq_consumer_group": groupID, "dlq_error": failure,
		"dlq_failed_at": failedAt.Format(time.RFC3339Nano),
	}
	for key, value := range want {
		if headers[key] != value {
			t.Errorf("header %s = %q, want %q", key, headers[key], value)
		}
	}
	record.Key[0] = 'X'
	record.Value[0] = 'X'
	if string(original.Key) != "asset-1" || string(original.Value) != "{invalid" {
		t.Fatal("dead-letter record aliases source key or value")
	}
}
