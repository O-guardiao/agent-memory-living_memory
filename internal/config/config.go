package config

import "time"

type Config struct {
	Env           string
	Port          string
	LogLevel      string
	DefaultTenant string
	StorageMode   string
	IngestionMode string

	PostgresDSN string

	QdrantEndpoint   string
	QdrantCollection string
	QdrantVectorSize int

	Neo4jEndpoint  string
	Neo4jDatabase  string
	Neo4jBasicAuth string

	// LLM providers. Empty LLMProvider keeps the heuristic distiller.
	LLMProvider       string
	AnthropicAPIKey   string
	AnthropicModel    string
	AnthropicEndpoint string
	OpenAIAPIKey      string
	OpenAIModel       string
	OpenAIEndpoint    string
	LocalLLMEndpoint  string
	LocalLLMModel     string

	// Embedding providers. "hash" keeps the deterministic local embedder.
	EmbeddingProvider  string
	OpenAIEmbedModel   string
	VoyageAPIKey       string
	VoyageEmbedModel   string
	LocalEmbedEndpoint string
	LocalEmbedModel    string
	EmbedBatchSize     int
	EmbedMaxRetries    int

	// Reranker providers. "simple" keeps the lexical-overlap reranker.
	RerankerProvider  string
	CohereAPIKey      string
	CohereRerankModel string
	VoyageRerankModel string

	// Queue providers. Empty derives from StorageMode (postgres or memory).
	QueueProvider string
	KafkaBrokers  string
	NATSURL       string

	// Redis cache and rate limiting. Empty RedisAddr disables Redis.
	RedisAddr     string
	RedisPassword string

	// S3-compatible object store. Empty S3Bucket disables it.
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3PathStyle bool

	// Base64-encoded 32-byte key for envelope encryption. Empty disables it.
	EncryptionKey string

	// Comma-separated "key" or "key:tenant_id" pairs. Empty disables auth.
	APIKeys string
	// Requests per second per tenant/client. Zero disables rate limiting.
	RateLimitRPS   int
	RateLimitBurst int

	// Feature flags, all default to current behavior (off).
	ConsolidationEnabled bool
	PrivacyGatesEnabled  bool
	EvalRecorderEnabled  bool

	// Scheduler intervals.
	SchedulerRetentionInterval time.Duration
	SchedulerReindexInterval   time.Duration
	SchedulerCompactInterval   time.Duration
	SchedulerJobsRetention     time.Duration

	// OTLP/HTTP endpoint for span export. Empty disables export.
	OTLPEndpoint string
}
