CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    agent_id TEXT,
    project_id TEXT,
    session_id TEXT,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    summary TEXT,
    source_event_ids TEXT[] NOT NULL DEFAULT '{}',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    importance DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active',
    scope TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    safety_labels TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memories_tenant_user ON memories (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_memories_status ON memories (status);
CREATE INDEX IF NOT EXISTS idx_memories_type ON memories (type);
