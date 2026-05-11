package config

import (
	"os"
	"strconv"
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

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
