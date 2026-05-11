CREATE TABLE IF NOT EXISTS memory_jobs (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    msg_key TEXT NOT NULL DEFAULT '',
    body BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_memory_jobs_topic_status_available
    ON memory_jobs (topic, status, available_at, created_at);

CREATE INDEX IF NOT EXISTS idx_memory_jobs_processing_stale
    ON memory_jobs (topic, locked_at)
    WHERE status = 'processing';
