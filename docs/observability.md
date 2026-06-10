# Observability

Métricas recomendadas:

- memory_ingestion_total
- memory_extraction_latency_ms
- memory_retrieval_latency_ms
- memory_context_assembly_latency_ms
- memory_candidates_total
- memory_used_total
- memory_deleted_total
- llm_tokens_input_total
- embedding_cost_total

Traces devem explicar: candidatos, selecionados, rejeitados, filtros, scores e latência.

Implementado:

- `GET /metrics` em formato Prometheus: `http_requests_total`,
  `memory_retrieval_latency_ms`, `memory_context_tokens`;
- logging estruturado JSON (`slog`) com nível via `MEMORY_LOG_LEVEL`;
- spans em ingestão, destilação, busca vetorial, rerank e context
  assembly, exportados via OTLP/HTTP quando `MEMORY_OTLP_ENDPOINT` é
  configurado.
