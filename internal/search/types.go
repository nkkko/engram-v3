package search

import (
	"context"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// EngineType represents the type of search engine
type EngineType string

const (
	// BadgerEngine is the default search engine using BadgerDB
	BadgerEngine EngineType = "badger"

	// VectorEngine uses vector embeddings for semantic search
	VectorEngine EngineType = "vector"

	// WeaviateEngine uses Weaviate for vector search
	WeaviateEngine EngineType = "weaviate"
)

// SearchOptions contains options for searching
type SearchOptions struct {
	// Query text to search for
	Query string

	// Context ID to filter by
	ContextID string

	// Agent IDs to filter by
	AgentIDs []string

	// Types to filter by
	Types []proto.WorkUnitType

	// Metadata filters
	MetaFilters map[string]string

	// Pagination
	Limit  int
	Offset int

	// Include full work units in results (not just IDs)
	IncludeFull bool

	// Semantic search options
	SemanticSearch bool
	Similarity     float64 // Minimum similarity score (0-1)
}

// SearchResults contains search results
type SearchResults struct {
	// Work unit IDs
	IDs []string

	// Full work units (if requested)
	WorkUnits []*proto.WorkUnit

	// Total count of matching items
	TotalCount int

	// Similarity scores (for semantic search)
	Scores map[string]float64
}

// Indexer handles indexing of work units
type Indexer interface {
	// Index indexes a work unit
	Index(ctx context.Context, unit *proto.WorkUnit) error

	// IndexBatch indexes a batch of work units
	IndexBatch(ctx context.Context, units []*proto.WorkUnit) error
}

// SearchProvider handles search operations
type SearchProvider interface {
	// Search performs a search with the given options
	Search(ctx context.Context, options SearchOptions) (*SearchResults, error)
}

// RelationshipFinder handles relationship queries
type RelationshipFinder interface {
	// FindByRelationship finds work units by relationship
	FindByRelationship(ctx context.Context, relType string, targetID string, options SearchOptions) (*SearchResults, error)
}

// SearchBackend combines indexing and search capabilities
type SearchBackend interface {
	Indexer
	SearchProvider
	RelationshipFinder

	// Start initializes the backend
	Start(ctx context.Context) error

	// Shutdown cleans up resources
	Shutdown(ctx context.Context) error
}