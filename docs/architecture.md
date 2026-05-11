# Architecture

O sistema segue portas e adaptadores:

```txt
HTTP/gRPC -> services -> ports <- adapters
```

O domínio não conhece bancos, LLM providers ou frameworks de agente.

Fluxo principal de memória:

```txt
Event ingestion
  -> Distillation
  -> Memory upsert
  -> Embedding/indexing
  -> Retrieval
  -> Context assembly
  -> Audit trace
```

Fluxo opcional de Agentic Control Plane:

```txt
Task
  -> /v1/agentic/decide
  -> research | specify | plan | execute | verify | recover
  -> SPEC/PLAN stored as procedural memories
  -> /v1/agentic/context
  -> compact control context injected into ContextPack.control
```

A regra de arquitetura é que o control plane não substitui a memória. Ele apenas decide quando uma tarefa precisa de trilhos adicionais e injeta contexto curto quando isso muda a próxima decisão.
