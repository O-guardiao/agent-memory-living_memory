# Retrieval

O MVP combina:

1. Busca textual simples.
2. Busca vetorial hash local.
3. Score por importância, confiança e recência.
4. Reranking lexical simples.
5. Trace auditável.

A evolução natural é plugar BM25 real, Qdrant/Weaviate/Milvus, reranker cross-encoder e grafo temporal.
