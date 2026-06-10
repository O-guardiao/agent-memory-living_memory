# Retrieval

O MVP combina:

1. Busca textual simples.
2. Busca vetorial hash local.
3. Score por importância, confiança e recência.
4. Reranking lexical simples.
5. Trace auditável.

A evolução natural é plugar BM25 real, Qdrant/Weaviate/Milvus, reranker cross-encoder e grafo temporal.

Disponível além do MVP:

- canal keyword dedicado (modo memória) e BM25 local como reranker
  (`MEMORY_RERANKER_PROVIDER=local`); rerankers Cohere/Voyage;
- embeddings OpenAI/Voyage/locais (`MEMORY_EMBEDDING_PROVIDER`) com
  batching e retry; ajuste `MEMORY_QDRANT_VECTOR_SIZE` à dimensão;
- filtros de escopo agent/project propagados ao índice vetorial,
  pós-filtro de sessão (`filters.session_only=true`), profundidade do
  grafo via `filters.graph_depth` (1-4) e `filters.graph=off`;
- consolidação (merge/supersede/contradição) com
  `MEMORY_CONSOLIDATION_ENABLED=true`;
- orçamento de tokens por consulta (`token_budget`) com compressão e
  citações de proveniência no context pack.
