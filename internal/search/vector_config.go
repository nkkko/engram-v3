package search

// VectorConfig contains configuration for vector search
type VectorConfig struct {
	// Enable vector search
	Enabled bool

	// URL of the Weaviate instance
	WeaviateURL string

	// Authentication token for Weaviate
	WeaviateAPIKey string

	// Name of the class to use in Weaviate
	ClassName string

	// Batch size for indexing operations
	BatchSize int

	// Embedding model configuration
	EmbeddingModel string
	EmbeddingURL   string
	EmbeddingKey   string
}

// DefaultVectorConfig returns default vector search configuration
func DefaultVectorConfig() VectorConfig {
	return VectorConfig{
		Enabled:        false,
		WeaviateURL:    "http://localhost:8080",
		ClassName:      "WorkUnit",
		BatchSize:      100,
		EmbeddingModel: "openai/text-embedding-3-small",
	}
}