package config

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
}
