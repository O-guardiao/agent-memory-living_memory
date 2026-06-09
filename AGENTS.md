# Agent Workspace Guide

This file marks the workspace root for tooling (the Living Memory v2
benchmarks resolve document paths relative to the directory that contains
`AGENTS.md`).

## Components

- `cmd/`, `internal/`, `sdk/` — Go control plane: ingestion, retrieval,
  context assembly, agentic control plane, HTTP API, workers, scheduler,
  CLI, and storage adapters (Postgres, Qdrant, Neo4j, memory).
- `engines/living_memoryv2/` — Python local-first memory engine with the
  MCP facade (`scripts/run_mcp_server.py`).
- `migrations/` — Postgres SQL and Neo4j Cypher migrations.
- `deploy/` — Docker, Kubernetes, and Helm manifests.
- `api/` — OpenAPI and protobuf definitions.
- `test/` — golden cases, k6 load scripts, and Go integration tests.

## Build and test

```bash
go build ./... && go vet ./... && go test ./...
cd engines/living_memoryv2/tests && python -m pytest -q
```

## Publication boundary

Never commit vault data, credentials, or private reports. See
`PUBLICATION_BOUNDARY.md` and `SECURITY.md` before adding files.
