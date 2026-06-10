# Security

Requisitos de produção:

- autenticação por tenant;
- autorização por escopo;
- isolamento tenant/user/project;
- deleção verificável;
- PII redaction;
- criptografia de memórias sensíveis;
- proteção contra prompt injection persistido em memória.

Implementado:

- API keys por tenant: `MEMORY_API_KEYS="chave"` ou `"chave:tenant_id"`
  (vazio = auth desligada, comportamento MVP);
- rate limiting: `MEMORY_RATE_LIMIT_RPS` (token bucket em memória ou
  janela fixa no Redis com `MEMORY_REDIS_ADDR`);
- privacy gates de retrieval: `MEMORY_PRIVACY_GATES_ENABLED=true` filtra
  memórias com rótulos credential/PII;
- redação de PII: `POST /v1/memories/{id}/redact`;
- recibos de auditoria hash-encadeados em deleções, persistidos no
  object store (S3) com criptografia de envelope AES-256-GCM quando
  `MEMORY_ENCRYPTION_KEY` (base64, 32 bytes) é configurada.
