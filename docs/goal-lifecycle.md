# Goal Lifecycle

How the agentic control plane moves a goal from intake to verified
delivery. The flow is implemented by `internal/services/agentic` and
exposed through `/v1/agentic/*`.

## Phases

A goal advances through up to seven phases (`internal/domain/agentic`):

```
triage → specify → research → plan → execute → verify
                     ↘ recover (on failure, from any phase)
```

| Phase | Purpose |
| --- | --- |
| `triage` | Classify the request: risk, horizon, whether it can be answered directly. |
| `specify` | Write a spec (`POST /v1/agentic/specs`) with scope, files and verification criteria. |
| `research` | Ground claims in evidence before committing them to memory. |
| `plan` | Break the spec into steps with checkpoints (`POST /v1/agentic/plans`). |
| `execute` | Carry out plan steps; memory context is assembled per step. |
| `verify` | Check expected evidence against verification criteria. |
| `recover` | Diagnose and reroute after a failed execution or verification. |

## Decision routing

`POST /v1/agentic/decide` receives the task plus state flags
(`has_approved_spec`, `has_plan`, `has_evidence`, `user_asked_for_code`)
and returns one route:

`answer_directly` | `clarify` | `specify` | `research` | `plan` |
`execute` | `verify` | `recover`

The advice also carries rails (e.g. `needs_spec`, `needs_evidence`),
recommended skills, required actions and the booleans
`should_write_spec` / `should_research` / `should_execute` /
`should_verify`, so an agent can drive the lifecycle without guessing.

## State and memory

- Specs and plans are persisted as memories in the scoped store and
  fetched back by ID (`GET /v1/agentic/specs/{id}`, `/plans/{id}`).
- `POST /v1/agentic/context` assembles a compact control context —
  active spec, active plan, current step, rails and skill cards —
  bounded by `token_budget`.
- Regular context assembly (`POST /v1/context/assemble`) attaches the
  same control block when relevant, so a single call gives an agent
  both knowledge memory and goal state.

## Lifecycle of the memories themselves

Independent of goals, each memory follows: distilled from events →
(optionally) consolidated (merge / supersede / contradiction links) →
retrievable while `active` and within its validity window → `expired`
by retention sweeps or `superseded` by newer facts → deleted or
redacted with an auditable receipt. See `docs/memory-model.md` and
`docs/retrieval.md`.
