CREATE INDEX memory_tenant IF NOT EXISTS FOR (m:Memory) ON (m.tenant_id);
CREATE INDEX memory_type IF NOT EXISTS FOR (m:Memory) ON (m.type);
