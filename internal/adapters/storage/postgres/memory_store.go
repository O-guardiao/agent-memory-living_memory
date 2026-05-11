package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/lib/pq"
)

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

func (s *MemoryStore) Upsert(ctx context.Context, mem memory.Memory) error {
	metadata, err := json.Marshal(mem.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO memories (
			id, tenant_id, user_id, agent_id, project_id, session_id, type, content, summary,
			source_event_ids, confidence, importance, valid_from, valid_until, status, scope,
			version, safety_labels, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21
		)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			agent_id = EXCLUDED.agent_id,
			project_id = EXCLUDED.project_id,
			session_id = EXCLUDED.session_id,
			type = EXCLUDED.type,
			content = EXCLUDED.content,
			summary = EXCLUDED.summary,
			source_event_ids = EXCLUDED.source_event_ids,
			confidence = EXCLUDED.confidence,
			importance = EXCLUDED.importance,
			valid_from = EXCLUDED.valid_from,
			valid_until = EXCLUDED.valid_until,
			status = EXCLUDED.status,
			scope = EXCLUDED.scope,
			version = EXCLUDED.version,
			safety_labels = EXCLUDED.safety_labels,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at`,
		mem.ID,
		mem.TenantID,
		mem.UserID,
		mem.AgentID,
		mem.ProjectID,
		mem.SessionID,
		string(mem.Type),
		mem.Content,
		mem.Summary,
		pq.Array(mem.SourceEventIDs),
		mem.Confidence,
		mem.Importance,
		mem.ValidFrom,
		mem.ValidUntil,
		string(mem.Status),
		string(mem.Scope),
		mem.Version,
		pq.Array(safetyLabelsToStrings(mem.SafetyLabels)),
		metadata,
		mem.CreatedAt,
		mem.UpdatedAt,
	)
	return err
}

func (s *MemoryStore) Get(ctx context.Context, tenantID, memoryID string) (memory.Memory, error) {
	row := s.db.QueryRowContext(ctx, memorySelectSQL+" WHERE tenant_id = $1 AND id = $2 AND status <> 'deleted'", tenantID, memoryID)
	return scanMemory(row)
}

func (s *MemoryStore) Delete(ctx context.Context, tenantID, memoryID string) error {
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE memories SET status = $1, updated_at = $2 WHERE tenant_id = $3 AND id = $4",
		string(memory.StatusDeleted),
		time.Now().UTC(),
		tenantID,
		memoryID,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return memory.ErrNotFound
	}
	return nil
}

func (s *MemoryStore) List(ctx context.Context, q retrieval.Query) ([]memory.Memory, error) {
	sqlText, args := buildMemoryListQuery(q, false)
	return s.queryMemories(ctx, sqlText, args)
}

func (s *MemoryStore) SearchByText(ctx context.Context, q retrieval.Query) ([]memory.Memory, error) {
	sqlText, args := buildMemoryListQuery(q, true)
	return s.queryMemories(ctx, sqlText, args)
}

func (s *MemoryStore) queryMemories(ctx context.Context, sqlText string, args []any) ([]memory.Memory, error) {
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []memory.Memory{}
	for rows.Next() {
		mem, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, rows.Err()
}

const memorySelectSQL = `SELECT id, tenant_id, user_id, agent_id, project_id, session_id, type, content, summary,
	source_event_ids, confidence, importance, valid_from, valid_until, status, scope, version,
	safety_labels, metadata, created_at, updated_at FROM memories`

func buildMemoryListQuery(q retrieval.Query, textSearch bool) (string, []any) {
	args := []any{}
	clauses := []string{}
	add := func(fragment string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(fragment, len(args)))
	}
	if q.TenantID != "" {
		add("tenant_id = $%d", q.TenantID)
	}
	if q.UserID != "" {
		add("user_id = $%d", q.UserID)
	}
	if q.ProjectID != "" {
		add("project_id = $%d", q.ProjectID)
	}
	status := q.Filters["status"]
	if status == "" {
		status = string(memory.StatusActive)
	}
	add("status = $%d", status)
	if typ := q.Filters["type"]; typ != "" {
		add("type = $%d", typ)
	}
	if after := q.Filters["created_after"]; after != "" {
		add("created_at >= $%d", after)
	}
	if before := q.Filters["created_before"]; before != "" {
		add("created_at <= $%d", before)
	}
	if textSearch && q.NormalizedText() != "" {
		args = append(args, "%"+q.NormalizedText()+"%")
		clauses = append(clauses, fmt.Sprintf("(content ILIKE $%d OR summary ILIKE $%d)", len(args), len(args)))
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	limit := q.NormalizedLimit()
	args = append(args, limit)
	return memorySelectSQL + where + " ORDER BY importance DESC, updated_at DESC LIMIT $" + strconv.Itoa(len(args)), args
}

type memoryScanner interface {
	Scan(dest ...any) error
}

func scanMemory(scanner memoryScanner) (memory.Memory, error) {
	var mem memory.Memory
	var typ, status, scope string
	var sourceEventIDs []string
	var safetyLabels []string
	var metadata []byte
	err := scanner.Scan(
		&mem.ID,
		&mem.TenantID,
		&mem.UserID,
		&mem.AgentID,
		&mem.ProjectID,
		&mem.SessionID,
		&typ,
		&mem.Content,
		&mem.Summary,
		pq.Array(&sourceEventIDs),
		&mem.Confidence,
		&mem.Importance,
		&mem.ValidFrom,
		&mem.ValidUntil,
		&status,
		&scope,
		&mem.Version,
		pq.Array(&safetyLabels),
		&metadata,
		&mem.CreatedAt,
		&mem.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return memory.Memory{}, memory.ErrNotFound
	}
	if err != nil {
		return memory.Memory{}, err
	}
	mem.Type = memory.Type(typ)
	mem.Status = memory.Status(status)
	mem.Scope = memory.Scope(scope)
	mem.SourceEventIDs = sourceEventIDs
	mem.SafetyLabels = stringsToSafetyLabels(safetyLabels)
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &mem.Metadata)
	}
	return mem, nil
}

func safetyLabelsToStrings(labels []memory.SafetyLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, string(label))
	}
	return out
}

func stringsToSafetyLabels(labels []string) []memory.SafetyLabel {
	out := make([]memory.SafetyLabel, 0, len(labels))
	for _, label := range labels {
		out = append(out, memory.SafetyLabel(label))
	}
	return out
}
