# Agent Memory + Living Memory v2

Public-safe release of the Agent Memory control plane and the Living Memory v2 local engine.

This repository contains:

- Go control plane for memory ingestion, retrieval, context assembly, traces, async workers, migrations, and storage adapters.
- Living Memory v2 Python engine for local-first recall, SIF/context delivery, graph relations, vault boxes, governance, and capture flows.
- Production-oriented adapters for Postgres, Qdrant, and Neo4j.
- Local development adapters and deterministic tests.

## Publication Boundary

This public tree intentionally excludes private vault data, local reports, research notes, private go-to-market material, private references, PDFs, generated caches, and any real credentials.

See `PUBLICATION_BOUNDARY.md` before adding new files.

## Quick Checks

```powershell
go test ./...
```

```powershell
cd engines/living_memoryv2/tests
python -m pytest -q
```

## CLI

```bash
go run ./cmd/memory-cli search --query "preferência de linguagem" --tenant tenant_demo --user user_123
go run ./cmd/memory-cli trace --id trace_xxx --tenant tenant_demo
go run ./cmd/memory-cli export --tenant tenant_demo --user user_123 --out memories.jsonl
go run ./cmd/memory-cli eval --queries queries.jsonl --out report.jsonl
```

Flags globais: `--base-url` (env `MEMORY_BASE_URL`), `--api-key`
(env `MEMORY_API_KEY`), `--tenant` (env `MEMORY_TENANT`).

## Binaries

- `memory-api` — HTTP API.
- `memory-worker` — consumidor assíncrono de ingestão.
- `memory-scheduler` — retenção, manutenção de fila, reindexação e compactação.
- `memory-migrate` — migrações Postgres/Neo4j.
- `memory-cli` — search, trace, export, eval.

Integração opcional via env (defaults preservam o comportamento local):
provedores LLM/embedding/reranker (`MEMORY_LLM_PROVIDER`,
`MEMORY_EMBEDDING_PROVIDER`, `MEMORY_RERANKER_PROVIDER`), filas Kafka/NATS
(`MEMORY_QUEUE_PROVIDER`), Redis (`MEMORY_REDIS_ADDR`), S3
(`MEMORY_S3_BUCKET`), criptografia (`MEMORY_ENCRYPTION_KEY`), auth
(`MEMORY_API_KEYS`), rate limit (`MEMORY_RATE_LIMIT_RPS`) e flags
`MEMORY_CONSOLIDATION_ENABLED` / `MEMORY_PRIVACY_GATES_ENABLED` /
`MEMORY_EVAL_RECORDER_ENABLED`.

## Integration Tests

```bash
docker compose -f deploy/docker/docker-compose.yaml up -d postgres qdrant neo4j
MEMORY_TEST_POSTGRES_DSN="postgres://memory:change-me-local@localhost:5432/memory?sslmode=disable" \
MEMORY_TEST_QDRANT_ENDPOINT=http://localhost:6333 \
MEMORY_TEST_NEO4J_ENDPOINT=http://localhost:7474 \
go test ./test/integration/...
```

Sem as variáveis, os testes de integração passam com skip.

## Local Stack

The Docker compose file is configured for local development placeholders. Set real secrets through environment variables or a private secret manager, not in git.

```powershell
docker compose -f deploy/docker/docker-compose.yaml up --build
```
