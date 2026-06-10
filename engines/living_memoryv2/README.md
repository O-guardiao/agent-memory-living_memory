# Living Memory v2

Local-first memory engine for agent context retrieval and capture.

Public-safe scope:

- Canonical memory envelopes and modality references.
- Temporal store, graph store, knowledge graph, Mobius index, SIF/context assembly, governance, vault boxes, proposal flow, task state, and MCP facade.
- Deterministic tests and small examples.

Excluded from this public copy:

- Private vault data.
- Internal analysis docs.
- Generated validation reports.
- Reference folders copied from other local experiments.

## Installation

The engine is standard-library only (Python 3.11+). Install editable for
development:

```bash
pip install -e .
```

Or run directly from the tree by adding `src` to `PYTHONPATH` — the MCP
entry point (`scripts/run_mcp_server.py`) does this automatically.

## Module map

All modules live under `src/living_memoryv2/`:

| Module | Purpose |
| --- | --- |
| `schema` | Canonical `MemoryEnvelope` and `ModalityRef` types. |
| `store` | Temporal store (SQLite-backed) for envelopes. |
| `graph` | Relation graph store and traversal. |
| `knowledge` | Knowledge graph extraction and queries. |
| `mobius` | Mobius index for associative recall. |
| `sif`, `context` | SIF/context assembly (v1 and v2). |
| `governance`, `lifecycle` | Retention, scope, and lifecycle policies. |
| `vault` | Vault boxes for protected payloads. |
| `agents` | Proposal flow between agents. |
| `task_state` | Durable task state tracking. |
| `recall`, `scoring`, `answerability`, `calibration` | Retrieval, ranking, and abstention. |
| `ingest`, `extraction`, `connectors`, `facts`, `consolidation`, `engram`, `tool_curriculum` | Capture and distillation flows. |
| `mcp_server` | MCP facade exposing 31 tools (`LivingMemoryTools`). |
| `benchmarks`, `benchmarks_external` | Local validation and replay suites. |

## MCP server

```bash
python scripts/run_mcp_server.py
# or, after `pip install -e .`:
living-memory-mcp
```

The facade registers tools such as `living_memory`, `remember_text`,
`recall_context`, `inspect_memory`, `query_graph`, and `add_relation`.

Environment variables:

- `LIVING_MEMORY_ROOT` — storage root for the vault (default
  `.living_memory_vault`; keep it out of git).
- `LIVING_MEMORY_COMPACT_TOOLS=1` — compact tool descriptors, reducing
  the tokens each MCP session spends on tool schemas.

### Using with Claude Code

Register the engine as persistent memory so sessions recall context
instead of re-reading files:

```bash
claude mcp add living-memory \
  --env LIVING_MEMORY_ROOT="$HOME/.living_memory_vault" \
  --env LIVING_MEMORY_COMPACT_TOOLS=1 \
  -- python3 /path/to/engines/living_memoryv2/scripts/run_mcp_server.py
```

Typical loop: `remember_text` / `remember_markdown` to capture durable
facts and decisions, `recall_context` or `recall_sif` at session start,
`fact_status` to check what is already known before re-deriving it.

## Benchmarks

```bash
python -c "import sys; sys.path.insert(0, 'src'); from living_memoryv2 import benchmarks; print(benchmarks.run_local_validation())"
python -c "import sys; sys.path.insert(0, 'src'); from living_memoryv2 import benchmarks; print(benchmarks.run_hard_validation())"
```

The replay suites resolve repository documents relative to the workspace
root, marked by `AGENTS.md` at the repository top level.

Run tests:

```powershell
cd tests
python -m pytest -q
```
