CREATE TABLE IF NOT EXISTS retrieval_traces (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    query TEXT NOT NULL,
    candidate_ids TEXT[] NOT NULL DEFAULT '{}',
    selected_ids TEXT[] NOT NULL DEFAULT '{}',
    rejected_ids TEXT[] NOT NULL DEFAULT '{}',
    scores JSONB NOT NULL DEFAULT '{}',
    filters JSONB NOT NULL DEFAULT '{}',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    token_estimate INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deletion_receipts (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    reason TEXT,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
