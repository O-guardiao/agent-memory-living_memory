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

## Local Stack

The Docker compose file is configured for local development placeholders. Set real secrets through environment variables or a private secret manager, not in git.

```powershell
docker compose -f deploy/docker/docker-compose.yaml up --build
```
