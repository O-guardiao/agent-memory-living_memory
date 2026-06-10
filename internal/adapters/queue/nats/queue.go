package nats

// Placeholder for NATS queue adapter.

import (
	"context"
	"time"

	natsgo "github.com/nats-io/nats.go"

	"github.com/agent-memory/agent-memory/internal/ports"
)

const (
	queueGroup = "memory-workers"
	keyHeader  = "Nats-Msg-Key"
)

// Queue implements ports.Queue on core NATS. Delivery is at-most-once;
// the Postgres queue remains the durable default. A JetStream variant can
// upgrade this to at-least-once when needed.
type Queue struct {
	conn *natsgo.Conn
}

func New(url string) (*Queue, error) {
	conn, err := natsgo.Connect(
		url,
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &Queue{conn: conn}, nil
}

func (q *Queue) Publish(ctx context.Context, msg ports.QueueMessage) error {
	_ = ctx
	natsMsg := natsgo.NewMsg(msg.Topic)
	natsMsg.Data = msg.Body
	if msg.Key != "" {
		natsMsg.Header.Set(keyHeader, msg.Key)
	}
	return q.conn.PublishMsg(natsMsg)
}

func (q *Queue) Subscribe(ctx context.Context, topic string, handler func(context.Context, ports.QueueMessage) error) error {
	sub, err := q.conn.QueueSubscribe(topic, queueGroup, func(natsMsg *natsgo.Msg) {
		_ = handler(ctx, ports.QueueMessage{
			Topic: topic,
			Key:   natsMsg.Header.Get(keyHeader),
			Body:  natsMsg.Data,
		})
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
	return nil
}

func (q *Queue) Close() error {
	q.conn.Close()
	return nil
}
