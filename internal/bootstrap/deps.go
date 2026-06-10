package bootstrap

// Dependencies are composed in app.go. Keep this file for production wiring variants.

import (
	"context"
	"strings"

	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	localembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/local"
	openaiembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/openai"
	voyageembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/voyage"
	"github.com/agent-memory/agent-memory/internal/adapters/http/middleware"
	anthropicllm "github.com/agent-memory/agent-memory/internal/adapters/llm/anthropic"
	localllm "github.com/agent-memory/agent-memory/internal/adapters/llm/local"
	openaillm "github.com/agent-memory/agent-memory/internal/adapters/llm/openai"
	kafkaqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/kafka"
	natsqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/nats"
	cohererank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/cohere"
	localrerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/local"
	simplerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/simple"
	voyagerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/voyage"
	redisstore "github.com/agent-memory/agent-memory/internal/adapters/storage/redis"
	s3store "github.com/agent-memory/agent-memory/internal/adapters/storage/s3"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/security"
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
	if cfg.RedisAddr != "" {
		client := redisstore.NewClient(cfg.RedisAddr, cfg.RedisPassword)
		return redisstore.NewRateLimiter(client, cfg.RateLimitRPS, cfg.RateLimitBurst)
	}
	return middleware.NewTokenBucket(cfg.RateLimitRPS, cfg.RateLimitBurst, nil)
}

// cacheFor returns the embedding cache: Redis when configured, else nil
// (no caching, current behavior).
func cacheFor(cfg config.Config, shutdown *Shutdown) ports.Cache {
	if cfg.RedisAddr == "" {
		return nil
	}
	client := redisstore.NewClient(cfg.RedisAddr, cfg.RedisPassword)
	if shutdown != nil {
		shutdown.Register("redis", func(context.Context) error { return client.Close() })
	}
	return redisstore.NewCache(client)
}

// queueFor overrides the storage-derived queue when MEMORY_QUEUE_PROVIDER
// is set to kafka or nats; otherwise the bundle's queue is kept.
func queueFor(cfg config.Config, fallback ports.Queue, shutdown *Shutdown) ports.Queue {
	switch cfg.QueueProvider {
	case "kafka":
		queue := kafkaqueue.New(splitList(cfg.KafkaBrokers), "memory-workers")
		if shutdown != nil {
			shutdown.Register("kafka", func(context.Context) error { return queue.Close() })
		}
		return queue
	case "nats":
		queue, err := natsqueue.New(cfg.NATSURL)
		if err != nil {
			panic(err)
		}
		if shutdown != nil {
			shutdown.Register("nats", func(context.Context) error { return queue.Close() })
		}
		return queue
	default:
		return fallback
	}
}

// objectStoreFor returns the configured object store (nil when no bucket),
// wrapped with envelope encryption when MEMORY_ENCRYPTION_KEY is set.
func objectStoreFor(cfg config.Config) ports.ObjectStore {
	if cfg.S3Bucket == "" {
		return nil
	}
	var store ports.ObjectStore = s3store.New(s3store.Config{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		PathStyle: cfg.S3PathStyle,
	}, nil)
	if cfg.EncryptionKey != "" {
		enc, err := security.NewEncryptor(cfg.EncryptionKey)
		if err != nil {
			panic(err)
		}
		store = security.NewEncryptingObjectStore(store, enc)
	}
	return store
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
