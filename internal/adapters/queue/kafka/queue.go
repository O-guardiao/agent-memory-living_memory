package kafka

// Placeholder for Kafka queue adapter.

import (
	"context"
	"errors"
	"sync"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/agent-memory/agent-memory/internal/ports"
)

// Queue implements ports.Queue on Kafka: lazily-created writers per topic
// and a consumer-group reader that commits only after the handler succeeds,
// so failed messages are redelivered.
type Queue struct {
	brokers []string
	groupID string

	mu      sync.Mutex
	writers map[string]*kafkago.Writer
}

func New(brokers []string, groupID string) *Queue {
	if groupID == "" {
		groupID = "memory-workers"
	}
	return &Queue{
		brokers: brokers,
		groupID: groupID,
		writers: make(map[string]*kafkago.Writer),
	}
}

func (q *Queue) Publish(ctx context.Context, msg ports.QueueMessage) error {
	return q.writerFor(msg.Topic).WriteMessages(ctx, kafkago.Message{
		Key:   []byte(msg.Key),
		Value: msg.Body,
	})
}

func (q *Queue) Subscribe(ctx context.Context, topic string, handler func(context.Context, ports.QueueMessage) error) error {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: q.brokers,
		GroupID: q.groupID,
		Topic:   topic,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := handler(ctx, ports.QueueMessage{Topic: topic, Key: string(msg.Key), Body: msg.Value}); err != nil {
			// No commit: the message stays pending and is redelivered.
			continue
		}
		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			return err
		}
	}
}

func (q *Queue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	var errs []error
	for _, writer := range q.writers {
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	q.writers = make(map[string]*kafkago.Writer)
	return errors.Join(errs...)
}

func (q *Queue) writerFor(topic string) *kafkago.Writer {
	q.mu.Lock()
	defer q.mu.Unlock()
	if writer, ok := q.writers[topic]; ok {
		return writer
	}
	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(q.brokers...),
		Topic:                  topic,
		AllowAutoTopicCreation: true,
		Balancer:               &kafkago.Hash{},
	}
	q.writers[topic] = writer
	return writer
}
