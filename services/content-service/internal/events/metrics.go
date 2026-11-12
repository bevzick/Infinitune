package events

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type Metrics struct {
	service                         string
	outboxClaimed                   atomic.Uint64
	outboxPublished                 atomic.Uint64
	outboxPublishFailures           atomic.Uint64
	outboxOperationFailures         atomic.Uint64
	outboxLastPublishedUnix         atomic.Int64
	consumerProcessed               atomic.Uint64
	consumerHandlerFailures         atomic.Uint64
	consumerOperationFailures       atomic.Uint64
	consumerDeadLettered            atomic.Uint64
	consumerDeadLetterPublishErrors atomic.Uint64
	consumerLastProcessedUnix       atomic.Int64
}

func NewMetrics(service string) *Metrics { return &Metrics{service: service} }

func (metrics *Metrics) OutboxClaimed(count int) {
	if count > 0 {
		metrics.outboxClaimed.Add(uint64(count))
	}
}

func (metrics *Metrics) OutboxPublished(at time.Time) {
	metrics.outboxPublished.Add(1)
	metrics.outboxLastPublishedUnix.Store(at.Unix())
}

func (metrics *Metrics) OutboxPublishFailed()   { metrics.outboxPublishFailures.Add(1) }
func (metrics *Metrics) OutboxOperationFailed() { metrics.outboxOperationFailures.Add(1) }

func (metrics *Metrics) ConsumerProcessed(at time.Time) {
	metrics.consumerProcessed.Add(1)
	metrics.consumerLastProcessedUnix.Store(at.Unix())
}

func (metrics *Metrics) ConsumerHandlerFailed()   { metrics.consumerHandlerFailures.Add(1) }
func (metrics *Metrics) ConsumerOperationFailed() { metrics.consumerOperationFailures.Add(1) }
func (metrics *Metrics) ConsumerDeadLettered()    { metrics.consumerDeadLettered.Add(1) }
func (metrics *Metrics) ConsumerDeadLetterPublishFailed() {
	metrics.consumerDeadLetterPublishErrors.Add(1)
}

func (metrics *Metrics) Wrap(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics)
	mux.Handle("/", next)
	return mux
}

func (metrics *Metrics) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	label := fmt.Sprintf("{service=%q}", metrics.service)
	writeMetric(response, "infinitune_kafka_outbox_claimed_total", "counter", label, metrics.outboxClaimed.Load())
	writeMetric(response, "infinitune_kafka_outbox_published_total", "counter", label, metrics.outboxPublished.Load())
	writeMetric(response, "infinitune_kafka_outbox_publish_failures_total", "counter", label, metrics.outboxPublishFailures.Load())
	writeMetric(response, "infinitune_kafka_outbox_operation_failures_total", "counter", label, metrics.outboxOperationFailures.Load())
	writeMetric(response, "infinitune_kafka_outbox_last_published_timestamp_seconds", "gauge", label, uint64(metrics.outboxLastPublishedUnix.Load()))
	writeMetric(response, "infinitune_kafka_consumer_processed_total", "counter", label, metrics.consumerProcessed.Load())
	writeMetric(response, "infinitune_kafka_consumer_handler_failures_total", "counter", label, metrics.consumerHandlerFailures.Load())
	writeMetric(response, "infinitune_kafka_consumer_operation_failures_total", "counter", label, metrics.consumerOperationFailures.Load())
	writeMetric(response, "infinitune_kafka_consumer_dead_lettered_total", "counter", label, metrics.consumerDeadLettered.Load())
	writeMetric(response, "infinitune_kafka_consumer_dead_letter_publish_failures_total", "counter", label, metrics.consumerDeadLetterPublishErrors.Load())
	writeMetric(response, "infinitune_kafka_consumer_last_processed_timestamp_seconds", "gauge", label, uint64(metrics.consumerLastProcessedUnix.Load()))
}

func writeMetric(response http.ResponseWriter, name, metricType, label string, value uint64) {
	fmt.Fprintf(response, "# TYPE %s %s\n%s%s %d\n", name, metricType, name, label, value)
}
