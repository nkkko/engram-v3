package search

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// skipVectorTests returns true if Weaviate is not available
func skipVectorTests(t *testing.T) bool {
	// Skip if WEAVIATE_URL is not set
	weaviateURL := os.Getenv("WEAVIATE_URL")
	if weaviateURL == "" {
		t.Skip("Skipping Weaviate tests, WEAVIATE_URL not set")
		return true
	}
	return false
}

// TestVectorConfig tests vector configuration
func TestVectorConfig(t *testing.T) {
	config := DefaultVectorConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, "http://localhost:8080", config.WeaviateURL)
	assert.Equal(t, "WorkUnit", config.ClassName)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, "openai/text-embedding-3-small", config.EmbeddingModel)
}

// TestEngineFactory tests the search engine factory
func TestEngineFactory(t *testing.T) {
	// Create factory with default config
	factory := NewEngineFactory(DefaultConfig(), DefaultVectorConfig())
	
	// Create BadgerDB engine
	badgerEngine, err := factory.CreateEngine(BadgerEngine)
	require.NoError(t, err)
	require.NotNil(t, badgerEngine)
	
	// Vector search is disabled in default config
	_, err = factory.CreateEngine(WeaviateEngine)
	assert.Error(t, err)
	
	// Enable vector search
	vectorConfig := DefaultVectorConfig()
	vectorConfig.Enabled = true
	
	// Skip actual Weaviate tests if not available
	if skipVectorTests(t) {
		return
	}
	
	// Update with Weaviate URL from environment
	vectorConfig.WeaviateURL = os.Getenv("WEAVIATE_URL")
	if apiKey := os.Getenv("WEAVIATE_API_KEY"); apiKey != "" {
		vectorConfig.WeaviateAPIKey = apiKey
	}
	
	// Create factory with vector config
	factory = NewEngineFactory(DefaultConfig(), vectorConfig)
	
	// Create Weaviate engine
	weaviateEngine, err := factory.CreateEngine(WeaviateEngine)
	require.NoError(t, err)
	require.NotNil(t, weaviateEngine)
}

// TestWeaviateBackend tests the Weaviate search backend
func TestWeaviateBackend(t *testing.T) {
	if skipVectorTests(t) {
		return
	}
	
	// Create config from environment
	config := DefaultVectorConfig()
	config.Enabled = true
	config.WeaviateURL = os.Getenv("WEAVIATE_URL")
	if apiKey := os.Getenv("WEAVIATE_API_KEY"); apiKey != "" {
		config.WeaviateAPIKey = apiKey
	}
	
	// Create test class name to avoid conflicts
	config.ClassName = "TestWorkUnit"
	
	// Create backend
	backend, err := NewWeaviateBackend(config)
	require.NoError(t, err)
	
	// Start backend
	ctx := context.Background()
	err = backend.Start(ctx)
	require.NoError(t, err)
	
	// Create test work units
	units := []*proto.WorkUnit{
		{
			Id:        "test-vector-1",
			ContextId: "test-context",
			AgentId:   "agent-1",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte("This is a test message about artificial intelligence"),
			Meta: map[string]string{
				"category": "ai",
				"priority": "high",
			},
			Ts: timestamppb.New(time.Now()),
		},
		{
			Id:        "test-vector-2",
			ContextId: "test-context",
			AgentId:   "agent-2",
			Type:      proto.WorkUnitType_SYSTEM,
			Payload:   []byte("System notification about database maintenance"),
			Meta: map[string]string{
				"category": "system",
				"priority": "medium",
			},
			Ts: timestamppb.New(time.Now()),
		},
		{
			Id:        "test-vector-3",
			ContextId: "test-context-2",
			AgentId:   "agent-1",
			Type:      proto.WorkUnitType_COMMAND,
			Payload:   []byte("Execute machine learning training job on dataset"),
			Meta: map[string]string{
				"category": "ml",
				"priority": "high",
				"relation_parent": "test-vector-1",
			},
			Ts: timestamppb.New(time.Now()),
		},
	}
	
	// Index work units
	err = backend.IndexBatch(ctx, units)
	require.NoError(t, err)
	
	// Wait for indexing to complete
	time.Sleep(2 * time.Second)
	
	// Test basic search
	results, err := backend.Search(ctx, SearchOptions{
		Query:     "",
		ContextID: "test-context",
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, results.TotalCount)
	
	// Test text search
	results, err = backend.Search(ctx, SearchOptions{
		Query: "artificial intelligence",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, results.TotalCount, 1)
	
	// Test semantic search
	results, err = backend.Search(ctx, SearchOptions{
		Query:         "AI and ML concepts",
		SemanticSearch: true,
		Similarity:    0.6,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, results.TotalCount, 1)
	
	// Test metadata filters
	results, err = backend.Search(ctx, SearchOptions{
		MetaFilters: map[string]string{
			"category": "ai",
		},
		Limit: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, results.TotalCount, 1)
	
	// Test relationship search
	results, err = backend.FindByRelationship(ctx, "parent", "test-vector-1", SearchOptions{
		Limit: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, results.TotalCount, 1)
	
	// Shutdown
	err = backend.Shutdown(ctx)
	require.NoError(t, err)
}