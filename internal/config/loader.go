package config

import (
	"os"
	"strconv"
	"time"
)

func Load() Config {
	return Config{
		Env:           env("MEMORY_ENV", "dev"),
		Port:          env("MEMORY_PORT", "8080"),
		LogLevel:      env("MEMORY_LOG_LEVEL", "debug"),
		DefaultTenant: env("MEMORY_DEFAULT_TENANT", "tenant_demo"),
		StorageMode:   env("MEMORY_STORAGE_MODE", "memory"),
		IngestionMode: env("MEMORY_INGESTION_MODE", "sync"),
		PostgresDSN:   env("MEMORY_POSTGRES_DSN", ""),
		QdrantEndpoint: env(
			"MEMORY_QDRANT_ENDPOINT",
			"http://localhost:6333",
		),
		QdrantCollection: env("MEMORY_QDRANT_COLLECTION", "agent_memory"),
		QdrantVectorSize: intEnv("MEMORY_QDRANT_VECTOR_SIZE", 64),
		Neo4jEndpoint:    env("MEMORY_NEO4J_ENDPOINT", "http://localhost:7474"),
		Neo4jDatabase:    env("MEMORY_NEO4J_DATABASE", "neo4j"),
		Neo4jBasicAuth:   env("MEMORY_NEO4J_BASIC_AUTH", ""),

		LLMProvider:       env("MEMORY_LLM_PROVIDER", ""),
		AnthropicAPIKey:   env("MEMORY_ANTHROPIC_API_KEY", ""),
		AnthropicModel:    env("MEMORY_ANTHROPIC_MODEL", "claude-opus-4-8"),
		AnthropicEndpoint: env("MEMORY_ANTHROPIC_ENDPOINT", "https://api.anthropic.com"),
		OpenAIAPIKey:      env("MEMORY_OPENAI_API_KEY", ""),
		OpenAIModel:       env("MEMORY_OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIEndpoint:    env("MEMORY_OPENAI_ENDPOINT", "https://api.openai.com"),
		LocalLLMEndpoint:  env("MEMORY_LOCAL_LLM_ENDPOINT", "http://localhost:11434/v1"),
		LocalLLMModel:     env("MEMORY_LOCAL_LLM_MODEL", "llama3.1"),

		EmbeddingProvider:  env("MEMORY_EMBEDDING_PROVIDER", "hash"),
		OpenAIEmbedModel:   env("MEMORY_OPENAI_EMBED_MODEL", "text-embedding-3-small"),
		VoyageAPIKey:       env("MEMORY_VOYAGE_API_KEY", ""),
		VoyageEmbedModel:   env("MEMORY_VOYAGE_EMBED_MODEL", "voyage-3.5-lite"),
		LocalEmbedEndpoint: env("MEMORY_LOCAL_EMBED_ENDPOINT", "http://localhost:11434/v1"),
		LocalEmbedModel:    env("MEMORY_LOCAL_EMBED_MODEL", "nomic-embed-text"),
		EmbedBatchSize:     intEnv("MEMORY_EMBED_BATCH_SIZE", 64),
		EmbedMaxRetries:    intEnv("MEMORY_EMBED_MAX_RETRIES", 3),

		RerankerProvider:  env("MEMORY_RERANKER_PROVIDER", "simple"),
		CohereAPIKey:      env("MEMORY_COHERE_API_KEY", ""),
		CohereRerankModel: env("MEMORY_COHERE_RERANK_MODEL", "rerank-v3.5"),
		VoyageRerankModel: env("MEMORY_VOYAGE_RERANK_MODEL", "rerank-2.5-lite"),

		QueueProvider: env("MEMORY_QUEUE_PROVIDER", ""),
		KafkaBrokers:  env("MEMORY_KAFKA_BROKERS", ""),
		NATSURL:       env("MEMORY_NATS_URL", "nats://localhost:4222"),

		RedisAddr:     env("MEMORY_REDIS_ADDR", ""),
		RedisPassword: env("MEMORY_REDIS_PASSWORD", ""),

		S3Endpoint:  env("MEMORY_S3_ENDPOINT", ""),
		S3Region:    env("MEMORY_S3_REGION", "us-east-1"),
		S3Bucket:    env("MEMORY_S3_BUCKET", ""),
		S3AccessKey: env("MEMORY_S3_ACCESS_KEY", ""),
		S3SecretKey: env("MEMORY_S3_SECRET_KEY", ""),
		S3PathStyle: boolEnv("MEMORY_S3_PATH_STYLE", true),

		EncryptionKey: env("MEMORY_ENCRYPTION_KEY", ""),

		APIKeys:        env("MEMORY_API_KEYS", ""),
		RateLimitRPS:   intEnv("MEMORY_RATE_LIMIT_RPS", 0),
		RateLimitBurst: intEnv("MEMORY_RATE_LIMIT_BURST", 0),

		ConsolidationEnabled: boolEnv("MEMORY_CONSOLIDATION_ENABLED", false),
		PrivacyGatesEnabled:  boolEnv("MEMORY_PRIVACY_GATES_ENABLED", false),
		EvalRecorderEnabled:  boolEnv("MEMORY_EVAL_RECORDER_ENABLED", false),

		SchedulerRetentionInterval: durationEnv("MEMORY_SCHEDULER_RETENTION_INTERVAL", time.Minute),
		SchedulerReindexInterval:   durationEnv("MEMORY_SCHEDULER_REINDEX_INTERVAL", 10*time.Minute),
		SchedulerCompactInterval:   durationEnv("MEMORY_SCHEDULER_COMPACT_INTERVAL", 5*time.Minute),
		SchedulerJobsRetention:     durationEnv("MEMORY_SCHEDULER_JOBS_RETENTION", 168*time.Hour),

		OTLPEndpoint: env("MEMORY_OTLP_ENDPOINT", ""),
	}
}

func intEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
