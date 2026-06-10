# Evals

Compare contra:

- no memory;
- full context;
- vector RAG;
- hybrid search;
- graph memory;
- concorrentes externos quando possível.

Categorias essenciais:

- recall factual;
- preferência;
- raciocínio temporal;
- atualização de conhecimento;
- contradição;
- abstenção;
- custo;
- latência p95/p99.

Ferramentas disponíveis:

- `MEMORY_EVAL_RECORDER_ENABLED=true` grava cada retrieval; exporte com
  `GET /v1/evals/export` ou `memory-cli eval`;
- `memory-cli eval --queries queries.jsonl --out report.jsonl` calcula
  hit-rate e latência p50/p95 por consulta.
