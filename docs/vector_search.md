# Vector Search in Engram v3

This document describes the vector search capabilities in Engram v3, including the Weaviate integration for semantic search.

## Overview

Engram v3 supports both traditional text search and semantic vector search. Vector search enables finding content based on meaning rather than exact keyword matches, providing more intuitive and powerful search capabilities.

The vector search functionality is implemented using:
- **Weaviate** as the vector database backend
- **OpenAI compatible embedding services** for generating vector embeddings
- **Pluggable architecture** that allows swapping implementation details

## Architecture

The vector search implementation consists of several key components:

```
┌─────────────────────────────────────────────┐
│                Search Interface              │
└───────────────────┬─────────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
┌───────▼───────┐       ┌───────▼───────┐
│ BadgerEngine   │       │ WeaviateEngine│
│ (Text Search)  │       │(Vector Search)│
└───────┬───────┘       └───────┬───────┘
        │                       │
┌───────▼───────┐       ┌───────▼───────┐
│   BadgerDB     │       │   Weaviate    │
└───────────────┘       └───────┬───────┘
                                │
                        ┌───────▼───────┐
                        │  Embedding    │
                        │   Service     │
                        └───────────────┘
```

### Components

1. **Search Interface**: Common API for all search operations
2. **BadgerEngine**: Text-based search using BadgerDB
3. **WeaviateEngine**: Vector-based search using Weaviate
4. **Embedding Service**: Generates vector embeddings for text

## Configuration

Vector search is disabled by default and can be enabled using the following configuration options:

```yaml
vector_search:
  enabled: true
  weaviate_url: "http://localhost:8080"
  weaviate_api_key: "your-api-key" # Optional
  class_name: "WorkUnit"
  batch_size: 10
  embedding_model: "openai/text-embedding-ada-002"
  embedding_url: "https://api.openai.com/v1"
  embedding_key: "your-openai-key"
```

Environment variables:
```
VECTOR_SEARCH_ENABLED=true
WEAVIATE_URL=http://localhost:8080
WEAVIATE_API_KEY=your-api-key
EMBEDDING_MODEL=openai/text-embedding-ada-002
EMBEDDING_URL=https://api.openai.com/v1
EMBEDDING_KEY=your-openai-key
```

Command-line arguments:
```
--vector-search=true
--weaviate-url=http://localhost:8080
--weaviate-api-key=your-api-key
--embedding-model=openai/text-embedding-ada-002
--embedding-url=https://api.openai.com/v1
--embedding-key=your-openai-key
```

## Weaviate Integration

### Schema Configuration

Engram v3 automatically creates the necessary schema in Weaviate:

```json
{
  "class": "WorkUnit",
  "description": "An Engram work unit entity with vector embeddings",
  "properties": [
    {
      "name": "content",
      "description": "The content of the work unit",
      "dataType": ["text"]
    },
    {
      "name": "contextId",
      "description": "The context ID",
      "dataType": ["string"],
      "index": true
    },
    {
      "name": "agentId",
      "description": "The agent ID",
      "dataType": ["string"],
      "index": true
    },
    {
      "name": "workUnitId",
      "description": "The work unit ID",
      "dataType": ["string"],
      "index": true
    },
    {
      "name": "unitType",
      "description": "The work unit type",
      "dataType": ["int"],
      "index": true
    },
    {
      "name": "timestamp",
      "description": "The timestamp",
      "dataType": ["date"],
      "index": true
    },
    {
      "name": "metadata",
      "description": "Additional metadata",
      "dataType": ["text"]
    }
  ],
  "vectorIndexConfig": {
    "distance": "cosine"
  }
}
```

### Embedding Generation

When a new work unit is created, its content is automatically sent to the embedding service to generate a vector representation:

1. The work unit content is extracted
2. The text is sent to the embedding service (e.g., OpenAI)
3. The resulting vector embedding is stored in Weaviate along with metadata
4. This process happens asynchronously to avoid slowing down work unit creation

### Batching

For efficiency, the embedding process uses batching:

```go
type EmbeddingBatcher struct {
    queue        chan *workUnitIndexRequest
    batchSize    int
    flushTimeout time.Duration
    // ...
}
```

Multiple work units are collected in a batch and processed together when:
- The batch size threshold is reached
- The flush timeout is triggered
- The system is shutting down

This approach significantly reduces API calls to the embedding service and improves throughput.

## API Usage

### Vector Search Endpoint

```
POST /search/vector
```

Request:
```json
{
  "query": "What is the purpose of distributed tracing?",
  "context_id": "ctx-123",
  "agent_ids": ["agent-1", "agent-2"],
  "types": [1, 2, 3],
  "meta_filters": {
    "category": "monitoring"
  },
  "limit": 5,
  "offset": 0,
  "min_similarity": 0.7
}
```

Response:
```json
{
  "results": [
    {
      "work_unit": {
        "id": "wu-456",
        "context_id": "ctx-123",
        "agent_id": "agent-1",
        "type": 1,
        "ts": "2023-09-01T12:34:56Z",
        "meta": {
          "category": "monitoring"
        },
        "payload": "base64-encoded-content"
      },
      "similarity": 0.92
    },
    {
      "work_unit": {
        "id": "wu-789",
        "context_id": "ctx-123",
        "agent_id": "agent-2",
        "type": 2,
        "ts": "2023-09-01T12:40:00Z",
        "meta": {
          "category": "monitoring"
        },
        "payload": "base64-encoded-content"
      },
      "similarity": 0.85
    }
  ],
  "total_count": 2,
  "limit": 5,
  "offset": 0
}
```

### Query Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| query | Natural language query text | (required) |
| context_id | Filter by context ID | (optional) |
| agent_ids | Filter by agent IDs | (optional) |
| types | Filter by work unit types | (optional) |
| meta_filters | Filter by metadata fields | (optional) |
| limit | Maximum number of results | 10 |
| offset | Pagination offset | 0 |
| min_similarity | Minimum similarity threshold (0.0-1.0) | 0.7 |

## Embedding Service

The embedding service supports any API-compatible with OpenAI's embedding endpoint:

```go
type EmbeddingService struct {
    model    string
    url      string
    apiKey   string
    cache    map[string][]float32
    cacheMu  sync.RWMutex
    client   *http.Client
    logger   zerolog.Logger
    disabled bool
}
```

### Supported Models

The default model is `openai/text-embedding-ada-002`, but any compatible model can be used:

- **OpenAI models**: Via OpenAI API
- **Open-source models**: Via compatible APIs (e.g., on Hugging Face)
- **Self-hosted models**: Via local compatible APIs

### Caching

The embedding service implements a memory cache to avoid redundant embedding requests:

```go
func (s *EmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
    // Check cache first
    s.cacheMu.RLock()
    embedding, found := s.cache[text]
    s.cacheMu.RUnlock()
    
    if found {
        // Cache hit
        return embedding, nil
    }
    
    // Cache miss, generate embedding
    // ...
    
    // Store in cache
    s.cacheMu.Lock()
    s.cache[text] = embedding
    s.cacheMu.Unlock()
    
    return embedding, nil
}
```

## Performance Considerations

### Embedding Service Performance

- Batch size affects latency and throughput
  - Larger batches reduce API calls but increase latency
  - Default batch size is 10
- Cache hit ratio significantly impacts performance
  - Frequently repeated content benefits more from caching
  - Default cache size is 10,000 entries

### Weaviate Performance

- Vector index performance scales with vector count
  - Up to millions of vectors with proper configuration
- Query performance depends on:
  - Vector dimension (default: 1536 for OpenAI embeddings)
  - Number of vectors in the index
  - Search filters applied

### Metrics

Key metrics for monitoring vector search performance:

- `engram_vector_search_queries_total`: Total number of vector search queries
- `engram_vector_search_query_duration_seconds`: Histogram of vector search query durations
- `engram_vector_index_operations_total`: Count of vector index operations
- `engram_embedding_requests_total`: Count of embedding requests
- `engram_embedding_cache_hits_total`: Count of embedding cache hits

## Example Usage

### Go Client Example

```go
// Create a new client with vector search capabilities
client := engram.NewClient("http://localhost:8080", "my-agent-id")

// Configure vector search
client.SetVectorSearchOptions(engram.VectorSearchOptions{
    MinSimilarity: 0.8,
    Limit: 5,
})

// Perform a vector search
results, err := client.VectorSearch("What is the purpose of distributed tracing?", "my-context-id")
if err != nil {
    log.Fatalf("Error during vector search: %v", err)
}

// Process results
for _, result := range results {
    fmt.Printf("Match (%.2f similarity): %s\n", result.Similarity, result.Content)
}
```

### cURL Example

```bash
curl -X POST http://localhost:8080/search/vector \
  -H "Content-Type: application/json" \
  -H "X-Agent-ID: my-agent-id" \
  -d '{
    "query": "What is the purpose of distributed tracing?",
    "context_id": "my-context-id",
    "limit": 5,
    "min_similarity": 0.8
  }'
```

## Troubleshooting

### Common Issues

1. **Embedding Service Connection Errors**:
   - Check API key and URL configuration
   - Verify network connectivity to the embedding service
   - Check rate limits on the embedding API

2. **Weaviate Connection Issues**:
   - Verify Weaviate URL is correct and accessible
   - Check Weaviate logs for errors
   - Ensure Weaviate has sufficient resources

3. **No Results Returned**:
   - Lower the `min_similarity` threshold
   - Verify content was properly indexed
   - Check that filters aren't too restrictive

### Logs and Debugging

Vector search components log detailed information with the following components:

- `component=search.vector`: Vector search operations
- `component=search.embedding`: Embedding service operations
- `component=search.weaviate`: Weaviate operations

Example log:
```json
{
  "level": "debug",
  "time": "2023-09-01T12:34:56Z",
  "component": "search.vector",
  "message": "Vector search query processed",
  "query": "distributed tracing",
  "results_count": 5,
  "duration_ms": 120.5,
  "min_similarity": 0.8
}
```

## Conclusion

Vector search in Engram v3 provides powerful semantic search capabilities, allowing users to find content based on meaning rather than exact keyword matches. By integrating with Weaviate and supporting flexible embedding services, Engram delivers efficient and accurate semantic search that enhances the overall user experience.