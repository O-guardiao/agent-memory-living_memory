# API

Endpoints principais de memória:

- `POST /v1/events`
- `POST /v1/memories/search`
- `POST /v1/context/assemble`
- `GET /v1/memories/{id}`
- `DELETE /v1/memories/{id}`
- `GET /v1/traces/{id}`

Endpoints do Agentic Control Plane:

- `GET /v1/agentic/skills`
- `POST /v1/agentic/decide`
- `POST /v1/agentic/specs`
- `GET /v1/agentic/specs/{id}`
- `POST /v1/agentic/plans`
- `GET /v1/agentic/plans/{id}`
- `POST /v1/agentic/context`

Veja `api/openapi/memory.v1.yaml`.

## Decision request

```json
{
  "tenant_id": "tenant_demo",
  "user_id": "user_123",
  "task": "Analisar links atuais e gerar arquitetura com zip pronto.",
  "token_budget": 900
}
```

Retorna uma rota como `answer_directly`, `research`, `specify`, `plan`, `execute`, `verify` ou `recover`.

## Control context

```json
{
  "tenant_id": "tenant_demo",
  "user_id": "user_123",
  "project_id": "agent_memory",
  "query": "implementar próximo passo",
  "token_budget": 1200
}
```

Retorna um pacote compacto com active spec, active plan, current step, rails, skill cards e instruções curtas.
