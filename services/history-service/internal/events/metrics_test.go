package events

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposePrometheusCountersAndPreserveApplicationRoutes(t *testing.T) {
	metrics := NewMetrics("history")
	at := time.Unix(1_800_000_000, 0)
	metrics.OutboxClaimed(2)
	metrics.OutboxPublished(at)
	metrics.OutboxPublishFailed()
	metrics.OutboxOperationFailed()
	metrics.ConsumerProcessed(at)
	metrics.ConsumerHandlerFailed()
	metrics.ConsumerOperationFailed()
	metrics.ConsumerDeadLettered()
	metrics.ConsumerDeadLetterPublishFailed()
	handler := metrics.Wrap(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status = %d, content type = %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	for _, metric := range []string{
		`infinitune_kafka_outbox_claimed_total{service="history"} 2`,
		`infinitune_kafka_outbox_published_total{service="history"} 1`,
		`infinitune_kafka_outbox_publish_failures_total{service="history"} 1`,
		`infinitune_kafka_outbox_operation_failures_total{service="history"} 1`,
		`infinitune_kafka_outbox_last_published_timestamp_seconds{service="history"} 1800000000`,
		`infinitune_kafka_consumer_processed_total{service="history"} 1`,
		`infinitune_kafka_consumer_handler_failures_total{service="history"} 1`,
		`infinitune_kafka_consumer_operation_failures_total{service="history"} 1`,
		`infinitune_kafka_consumer_dead_lettered_total{service="history"} 1`,
		`infinitune_kafka_consumer_dead_letter_publish_failures_total{service="history"} 1`,
		`infinitune_kafka_consumer_last_processed_timestamp_seconds{service="history"} 1800000000`,
	} {
		if !strings.Contains(recorder.Body.String(), metric) {
			t.Errorf("response does not contain %q", metric)
		}
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("wrapped route status = %d", recorder.Code)
	}
}
