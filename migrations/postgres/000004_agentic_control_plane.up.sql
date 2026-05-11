CREATE INDEX IF NOT EXISTS idx_memories_agentic_kind
    ON memories ((metadata->>'agentic_kind'))
    WHERE metadata ? 'agentic_kind';

CREATE INDEX IF NOT EXISTS idx_memories_agentic_spec
    ON memories ((metadata->>'spec_id'))
    WHERE metadata ? 'spec_id';
