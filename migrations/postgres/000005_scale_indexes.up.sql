CREATE INDEX IF NOT EXISTS idx_events_tenant_user_project_created
    ON events (tenant_id, user_id, project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_events_tenant_session_created
    ON events (tenant_id, session_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_memories_tenant_user_project_status_type_updated
    ON memories (tenant_id, user_id, project_id, status, type, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_memories_tenant_project_status_type_valid
    ON memories (tenant_id, project_id, status, type, valid_from DESC, valid_until);

CREATE INDEX IF NOT EXISTS idx_memories_tenant_status_created
    ON memories (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_user_created
    ON retrieval_traces (tenant_id, user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_latency_created
    ON retrieval_traces (tenant_id, latency_ms DESC, created_at DESC);
