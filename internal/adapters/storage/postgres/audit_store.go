package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/lib/pq"
)

type TraceStore struct {
	db *sql.DB
}

func NewTraceStore(db *sql.DB) *TraceStore {
	return &TraceStore{db: db}
}

func (s *TraceStore) SaveTrace(ctx context.Context, trace retrieval.Trace) error {
	scores, err := json.Marshal(trace.Scores)
	if err != nil {
		return err
	}
	filters, err := json.Marshal(trace.Filters)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO retrieval_traces (
			id, tenant_id, user_id, query, candidate_ids, selected_ids, rejected_ids,
			scores, filters, latency_ms, token_estimate, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			candidate_ids = EXCLUDED.candidate_ids,
			selected_ids = EXCLUDED.selected_ids,
			rejected_ids = EXCLUDED.rejected_ids,
			scores = EXCLUDED.scores,
			filters = EXCLUDED.filters,
			latency_ms = EXCLUDED.latency_ms,
			token_estimate = EXCLUDED.token_estimate`,
		trace.ID,
		trace.TenantID,
		trace.UserID,
		trace.Query,
		pq.Array(trace.CandidateIDs),
		pq.Array(trace.SelectedIDs),
		pq.Array(trace.RejectedIDs),
		scores,
		filters,
		trace.LatencyMS,
		trace.TokenEstimate,
		trace.CreatedAt,
	)
	return err
}

func (s *TraceStore) GetTrace(ctx context.Context, tenantID, traceID string) (retrieval.Trace, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, user_id, query, candidate_ids, selected_ids, rejected_ids,
			scores, filters, latency_ms, token_estimate, created_at
		FROM retrieval_traces WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		traceID,
	)
	return scanTrace(row)
}

func (s *TraceStore) DeleteByMemoryID(ctx context.Context, tenantID, memoryID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE retrieval_traces
		SET candidate_ids = array_remove(candidate_ids, $2),
			selected_ids = array_remove(selected_ids, $2),
			rejected_ids = array_remove(rejected_ids, $2),
			scores = scores - $2
		WHERE tenant_id = $1`,
		tenantID,
		memoryID,
	)
	return err
}

func scanTrace(scanner memoryScanner) (retrieval.Trace, error) {
	var trace retrieval.Trace
	var candidateIDs, selectedIDs, rejectedIDs []string
	var scores, filters []byte
	err := scanner.Scan(
		&trace.ID,
		&trace.TenantID,
		&trace.UserID,
		&trace.Query,
		pq.Array(&candidateIDs),
		pq.Array(&selectedIDs),
		pq.Array(&rejectedIDs),
		&scores,
		&filters,
		&trace.LatencyMS,
		&trace.TokenEstimate,
		&trace.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return retrieval.Trace{}, memory.ErrNotFound
	}
	if err != nil {
		return retrieval.Trace{}, err
	}
	trace.CandidateIDs = candidateIDs
	trace.SelectedIDs = selectedIDs
	trace.RejectedIDs = rejectedIDs
	if len(scores) > 0 {
		_ = json.Unmarshal(scores, &trace.Scores)
	}
	if len(filters) > 0 {
		_ = json.Unmarshal(filters, &trace.Filters)
	}
	return trace, nil
}
