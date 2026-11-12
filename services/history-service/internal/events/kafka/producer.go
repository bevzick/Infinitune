package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/events"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(15*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

func (producer *Producer) Publish(ctx context.Context, message events.Message) error {
	record := &kgo.Record{
		Topic: message.Topic,
		Key:   []byte(message.Key),
		Value: message.Payload,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(message.ID.String())},
			{Key: "event_type", Value: []byte(message.Type)},
			{Key: "content_type", Value: []byte("application/json")},
		},
	}
	if err := producer.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish %s: %w", message.Type, err)
	}
	return nil
}

func (producer *Producer) Close() {
	producer.client.Close()
}
