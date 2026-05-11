package ports

import "context"

type QueueMessage struct {
	Topic string
	Key   string
	Body  []byte
}

type Queue interface {
	Publish(ctx context.Context, msg QueueMessage) error
	Subscribe(ctx context.Context, topic string, handler func(context.Context, QueueMessage) error) error
}
