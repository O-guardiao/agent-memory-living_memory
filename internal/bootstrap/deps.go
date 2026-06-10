package bootstrap

// Dependencies are composed in app.go. Keep this file for production wiring variants.

import (
	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	localembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/local"
	openaiembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/openai"
	voyageembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/voyage"
	"github.com/agent-memory/agent-memory/internal/adapters/http/middleware"
	anthropicllm "github.com/agent-memory/agent-memory/internal/adapters/llm/anthropic"
	localllm "github.com/agent-memory/agent-memory/internal/adapters/llm/local"
	openaillm "github.com/agent-memory/agent-memory/internal/adapters/llm/openai"
	cohererank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/cohere"
	localrerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/local"
	simplerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/simple"
	voyagerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/voyage"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/services/distillation"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

// embedderFor selects the embedding provider. The default "hash" keeps the
// deterministic local embedder; remote providers are wrapped in a Batcher.
// When a remote provider is chosen, MEMORY_QDRANT_VECTOR_SIZE must match
// the provider dimension.
func embedderFor(cfg config.Config) ports.Embedder {
	var remote ports.Embedder
	switch cfg.EmbeddingProvider {
	case "openai":
		remote = openaiembed.New(cfg.OpenAIEndpoint, cfg.OpenAIAPIKey, cfg.OpenAIEmbedModel, cfg.QdrantVectorSize, nil)
	case "voyage":
		remote = voyageembed.New(cfg.VoyageAPIKey, cfg.VoyageEmbedModel, cfg.QdrantVectorSize, nil)
	case "local":
		remote = localembed.New(cfg.LocalEmbedEndpoint, cfg.LocalEmbedModel, cfg.QdrantVectorSize, nil)
	default:
		return hashembed.New(cfg.QdrantVectorSize)
	}
	return embedding.NewBatcher(remote, cfg.EmbedBatchSize, cfg.EmbedMaxRetries)
}

// rerankerFor selects the reranker. The default "simple" keeps the lexical
// overlap reranker.
func rerankerFor(cfg config.Config) ports.Reranker {
	switch cfg.RerankerProvider {
	case "cohere":
		return cohererank.New(cfg.CohereAPIKey, cfg.CohereRerankModel, nil)
	case "voyage":
		return voyagerank.New(cfg.VoyageAPIKey, cfg.VoyageRerankModel, nil)
	case "local":
		return localrerank.New()
	default:
		return simplerank.New()
	}
}

// llmFor selects the LLM provider; nil keeps LLM-dependent features off.
func llmFor(cfg config.Config) ports.LLM {
	switch cfg.LLMProvider {
	case "anthropic":
		return anthropicllm.New(cfg.AnthropicEndpoint, cfg.AnthropicAPIKey, cfg.AnthropicModel, nil)
	case "openai":
		return openaillm.New(cfg.OpenAIEndpoint, cfg.OpenAIAPIKey, cfg.OpenAIModel, nil)
	case "local":
		return localllm.New(cfg.LocalLLMEndpoint, cfg.LocalLLMModel, nil)
	default:
		return nil
	}
}

// distillerFor returns the LLM-backed distiller when a provider is set,
// otherwise the heuristic one (current default behavior).
func distillerFor(cfg config.Config, idgen ports.IDGenerator, clock ports.Clock) *distillation.Service {
	if llm := llmFor(cfg); llm != nil {
		return distillation.NewLLMService(llm, idgen, clock)
	}
	return distillation.NewService(idgen, clock)
}

// limiterFor selects the rate limiter: disabled unless MEMORY_RATE_LIMIT_RPS
// is set. The Redis-backed variant is chosen when MEMORY_REDIS_ADDR is set.
func limiterFor(cfg config.Config) middleware.RateLimiter {
	if cfg.RateLimitRPS <= 0 {
		return nil
	}
	return middleware.NewTokenBucket(cfg.RateLimitRPS, cfg.RateLimitBurst, nil)
}
