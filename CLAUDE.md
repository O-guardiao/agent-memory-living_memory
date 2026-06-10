# CLAUDE.md

Concise guide for Claude Code sessions in this repository. Read this
instead of re-exploring the tree — it is kept accurate on purpose.

## What this repo is

Agent Memory: a Go memory control plane (ingestion → distillation →
retrieval → context assembly, plus an agentic control plane) and the
Living Memory v2 Python engine (`engines/living_memoryv2`, local-first,
stdlib-only, exposes 31 tools over MCP).

## Commands

```bash
go build ./... && go vet ./... && go test ./...      # Go: all green, no infra needed
cd engines/living_memoryv2 && python3 -m unittest discover -s tests -p "test_*.py"  # 109 tests
go run ./cmd/memory-api                              # HTTP API on :8080 (in-memory mode)
go run ./cmd/memory-cli search --query "..."         # CLI: search|trace|export|eval
```

Integration tests skip without infra; gate them with
`MEMORY_TEST_POSTGRES_DSN`, `MEMORY_TEST_QDRANT_ENDPOINT`,
`MEMORY_TEST_NEO4J_ENDPOINT`.

## Map

- `cmd/` — memory-api, memory-worker, memory-scheduler, memory-migrate, memory-cli.
- `internal/ports/` — interfaces; `internal/adapters/` — implementations
  (postgres/qdrant/neo4j/redis/s3, llm/embeddings/rerankers, kafka/nats,
  HTTP server + middleware). Remote adapters are stdlib `net/http`,
  tested with `httptest`.
- `internal/services/` — ingestion, distillation, retrieval, context,
  consolidation, forgetting, evaluation, audit, agentic, scheduler.
- `internal/bootstrap/` — all wiring (`app.go`, `deps.go`, `scheduler.go`).
- `internal/config/` — env-var config (`MEMORY_*`); every option defaults
  to current behavior. Full list in `.env.example`.
- `engines/living_memoryv2/` — Python engine + MCP server.

## Conventions

- New features must keep defaults backward compatible (empty env = old
  behavior) and never delete intent comments.
- Validate with the commands above before committing; `gofmt` clean.
- Respect `PUBLICATION_BOUNDARY.md` / `SECURITY.md`: no vault data,
  credentials or private notes in commits.

## Using the engine as memory for Claude Code (token saver)

Register the MCP server so sessions can store/recall context instead of
re-reading files:

```bash
claude mcp add living-memory \
  --env LIVING_MEMORY_ROOT="$HOME/.living_memory_vault" \
  --env LIVING_MEMORY_COMPACT_TOOLS=1 \
  -- python3 engines/living_memoryv2/scripts/run_mcp_server.py
```

`LIVING_MEMORY_COMPACT_TOOLS=1` emits compact tool descriptors (fewer
tokens per session). Key tools: `remember_text`, `remember_markdown`,
`recall_context`, `recall_sif`, `query_graph`, `fact_status`.
