package memoryqueue

import (
	"context"
	"sync"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type Queue struct {
	mu       sync.RWMutex
	handlers map[string][]func(context.Context, ports.QueueMessage) error
}

func New() *Queue {
	return &Queue{handlers: map[string][]func(context.Context, ports.QueueMessage) error{}}
}

func (q *Queue) Publish(ctx context.Context, msg ports.QueueMessage) error {
	q.mu.RLock()
	handlers := append([]func(context.Context, ports.QueueMessage) error(nil), q.handlers[msg.Topic]...)
	q.mu.RUnlock()
	for _, h := range handlers {
		if err := h(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) Subscribe(ctx context.Context, topic string, handler func(context.Context, ports.QueueMessage) error) error {
	_ = ctx
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[topic] = append(q.handlers[topic], handler)
	return nil
}
