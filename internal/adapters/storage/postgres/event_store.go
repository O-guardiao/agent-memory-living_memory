package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

type EventStore struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{db: db}
}

func (s *EventStore) Append(ctx context.Context, event memory.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO events (
			id, tenant_id, user_id, agent_id, session_id, project_id, role,
			content, tool_name, tool_result, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			agent_id = EXCLUDED.agent_id,
			session_id = EXCLUDED.session_id,
			project_id = EXCLUDED.project_id,
			role = EXCLUDED.role,
			content = EXCLUDED.content,
			tool_name = EXCLUDED.tool_name,
			tool_result = EXCLUDED.tool_result,
			metadata = EXCLUDED.metadata`,
		event.ID,
		event.TenantID,
		event.UserID,
		event.AgentID,
		event.SessionID,
		event.ProjectID,
		string(event.Role),
		event.Content,
		event.ToolName,
		event.ToolResult,
		metadata,
		event.CreatedAt,
	)
	return err
}

func (s *EventStore) Get(ctx context.Context, tenantID, eventID string) (memory.Event, error) {
	row := s.db.QueryRowContext(ctx, eventSelectSQL+" WHERE tenant_id = $1 AND id = $2", tenantID, eventID)
	return scanEvent(row)
}

func (s *EventStore) ListBySession(ctx context.Context, tenantID, sessionID string, limit int) ([]memory.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(
		ctx,
		eventSelectSQL+" WHERE tenant_id = $1 AND session_id = $2 ORDER BY created_at ASC LIMIT $3",
		tenantID,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []memory.Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

const eventSelectSQL = `SELECT id, tenant_id, user_id, agent_id, session_id, project_id, role,
	content, tool_name, tool_result, metadata, created_at FROM events`

func scanEvent(scanner memoryScanner) (memory.Event, error) {
	var event memory.Event
	var role string
	var metadata []byte
	err := scanner.Scan(
		&event.ID,
		&event.TenantID,
		&event.UserID,
		&event.AgentID,
		&event.SessionID,
		&event.ProjectID,
		&role,
		&event.Content,
		&event.ToolName,
		&event.ToolResult,
		&metadata,
		&event.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return memory.Event{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.Event{}, err
	}
	event.Role = memory.EventRole(role)
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &event.Metadata)
	}
	return event, nil
}
