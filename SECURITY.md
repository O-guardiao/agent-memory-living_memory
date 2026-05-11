# Security

Do not commit secrets.

Use environment variables, CI/CD secrets, or a managed secret store for production values:

- `MEMORY_POSTGRES_DSN`
- `MEMORY_QDRANT_ENDPOINT`
- `MEMORY_QDRANT_API_KEY`
- `MEMORY_NEO4J_ENDPOINT`
- `MEMORY_NEO4J_BASIC_AUTH`
- provider API keys

If a secret is committed accidentally, revoke and rotate it first, then remove it from history.

