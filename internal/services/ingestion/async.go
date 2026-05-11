package ingestion

import (
	"context"
	"encoding/json"

	"github.com/agent-memory/agent-memory/internal/ports"
)

const TopicIngestEvents = "memory.events.ingest"

type AsyncService struct {
	queue ports.Queue
	sync  *Service
}

func NewAsyncService(queue ports.Queue, sync *Service) *AsyncService {
	return &AsyncService{queue: queue, sync: sync}
}

func (s *AsyncService) Enqueue(ctx context.Context, req IngestRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return s.queue.Publish(ctx, ports.QueueMessage{
		Topic: TopicIngestEvents,
		Key:   req.TenantID + ":" + req.UserID,
		Body:  body,
	})
}

func (s *AsyncService) StartWorker(ctx context.Context) error {
	return s.queue.Subscribe(ctx, TopicIngestEvents, func(ctx context.Context, msg ports.QueueMessage) error {
		var req IngestRequest
		if err := json.Unmarshal(msg.Body, &req); err != nil {
			return err
		}
		_, err := s.sync.Ingest(ctx, req)
		return err
	})
}
