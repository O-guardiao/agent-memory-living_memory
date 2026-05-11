package postgresqueue

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type Queue struct {
	db           *sql.DB
	pollInterval time.Duration
}

func New(db *sql.DB) *Queue {
	return &Queue{db: db, pollInterval: 500 * time.Millisecond}
}

func (q *Queue) Publish(ctx context.Context, msg ports.QueueMessage) error {
	_, err := q.db.ExecContext(
		ctx,
		`INSERT INTO memory_jobs (id, topic, msg_key, body, status, attempts, available_at, created_at)
		 VALUES ($1, $2, $3, $4, 'queued', 0, now(), now())`,
		newJobID(),
		msg.Topic,
		msg.Key,
		msg.Body,
	)
	return err
}

func (q *Queue) Subscribe(ctx context.Context, topic string, handler func(context.Context, ports.QueueMessage) error) error {
	for {
		handled, err := q.processNext(ctx, topic, handler)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(q.pollInterval):
				continue
			}
		}
		if handled {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(q.pollInterval):
		}
	}
}

func (q *Queue) processNext(ctx context.Context, topic string, handler func(context.Context, ports.QueueMessage) error) (bool, error) {
	var id string
	var msg ports.QueueMessage
	row := q.db.QueryRowContext(
		ctx,
		`WITH next_job AS (
			SELECT id
			FROM memory_jobs
			WHERE topic = $1
			  AND attempts < max_attempts
			  AND (
				(status IN ('queued', 'retry') AND available_at <= now())
				OR (status = 'processing' AND locked_at < now() - interval '5 minutes')
			  )
			ORDER BY available_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE memory_jobs AS j
		SET status = 'processing',
			attempts = attempts + 1,
			locked_at = now()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.topic, j.msg_key, j.body`,
		topic,
	)
	if err := row.Scan(&id, &msg.Topic, &msg.Key, &msg.Body); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if err := handler(ctx, msg); err != nil {
		if failErr := q.markFailed(ctx, id, err); failErr != nil {
			return true, fmt.Errorf("handler failed: %v; marking failed: %w", err, failErr)
		}
		return true, nil
	}
	if err := q.markDone(ctx, id); err != nil {
		return true, err
	}
	return true, nil
}

func (q *Queue) markDone(ctx context.Context, id string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE memory_jobs SET status = 'done', completed_at = now() WHERE id = $1`, id)
	return err
}

func (q *Queue) markFailed(ctx context.Context, id string, handlerErr error) error {
	_, err := q.db.ExecContext(
		ctx,
		`UPDATE memory_jobs
		 SET status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'retry' END,
			 last_error = $2,
			 available_at = now() + interval '5 seconds'
		 WHERE id = $1`,
		id,
		handlerErr.Error(),
	)
	return err
}

func newJobID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	return "job_" + hex.EncodeToString(raw)
}
