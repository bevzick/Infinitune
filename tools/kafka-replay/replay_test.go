package main

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestReplayRecordRestoresOriginalRecord(t *testing.T) {
	originalHeaders := []kgo.RecordHeader{
		{Key: "event_id", Value: []byte("event-1")},
		{Key: "dlq_error", Value: []byte("original-header")},
	}
	source := deadLetterRecord(originalHeaders)

	replayed, err := replayRecord(source, defaultDeadLetterTopic)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Topic != "history.listening.recorded.v1" || replayed.Partition != 2 || string(replayed.Key) != "user-1" || string(replayed.Value) != "payload" {
		t.Fatalf("replayed record = %+v", replayed)
	}
	if len(replayed.Headers) != len(originalHeaders) || replayed.Headers[1].Key != "dlq_error" || string(replayed.Headers[1].Value) != "original-header" {
		t.Fatalf("headers = %+v", replayed.Headers)
	}
	replayed.Key[0] = 'X'
	replayed.Value[0] = 'X'
	replayed.Headers[0].Value[0] = 'X'
	if string(source.Key) != "user-1" || string(source.Value) != "payload" || string(source.Headers[0].Value) != "event-1" {
		t.Fatal("replayed record aliases dead-letter buffers")
	}
}

func TestReplayRecordRejectsMalformedMetadataSuffix(t *testing.T) {
	source := deadLetterRecord(nil)
	source.Headers[len(source.Headers)-1].Key = "unexpected"
	if _, err := replayRecord(source, defaultDeadLetterTopic); err == nil {
		t.Fatal("expected malformed metadata error")
	}
}

func TestReplayRecordRejectsInvalidPartition(t *testing.T) {
	source := deadLetterRecord(nil)
	source.Headers[len(source.Headers)-5].Value = []byte("invalid")
	if _, err := replayRecord(source, defaultDeadLetterTopic); err == nil {
		t.Fatal("expected invalid partition error")
	}
}

func deadLetterRecord(originalHeaders []kgo.RecordHeader) *kgo.Record {
	headers := append([]kgo.RecordHeader(nil), originalHeaders...)
	headers = append(headers,
		kgo.RecordHeader{Key: "dlq_original_topic", Value: []byte("history.listening.recorded.v1")},
		kgo.RecordHeader{Key: "dlq_original_partition", Value: []byte("2")},
		kgo.RecordHeader{Key: "dlq_original_offset", Value: []byte("42")},
		kgo.RecordHeader{Key: "dlq_consumer_group", Value: []byte("recommendation-history-v1")},
		kgo.RecordHeader{Key: "dlq_error", Value: []byte("invalid event")},
		kgo.RecordHeader{Key: "dlq_failed_at", Value: []byte("2026-11-12T12:00:00Z")},
	)
	return &kgo.Record{
		Topic: defaultDeadLetterTopic, Partition: 1, Offset: 9,
		Key: []byte("user-1"), Value: []byte("payload"), Headers: headers,
	}
}
