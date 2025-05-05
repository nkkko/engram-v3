package search

import (
	"context"
	"fmt"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// EngineFactory creates search engine instances
type EngineFactory struct {
	// Configuration
	config Config
	
	// Vector search configuration
	vectorConfig VectorConfig
}

// NewEngineFactory creates a new search engine factory
func NewEngineFactory(config Config, vectorConfig VectorConfig) *EngineFactory {
	return &EngineFactory{
		config:       config,
		vectorConfig: vectorConfig,
	}
}

// CreateEngine creates a search engine of the specified type
func (f *EngineFactory) CreateEngine(engineType EngineType) (SearchBackend, error) {
	logger := log.With().Str("component", "search-factory").Logger()
	logger.Info().Str("engine", string(engineType)).Msg("Creating search engine")

	switch engineType {
	case BadgerEngine:
		// Create BadgerDB-based search engine
		return NewBadgerBackend(f.config)
	case WeaviateEngine:
		// Check if vector search is enabled
		if !f.vectorConfig.Enabled {
			return nil, fmt.Errorf("vector search is not enabled in configuration")
		}
		// Create Weaviate-based search engine
		return NewWeaviateBackend(f.vectorConfig)
	default:
		return nil, fmt.Errorf("unsupported search engine type: %s", engineType)
	}
}

// NewBadgerBackend creates a new BadgerDB search backend
func NewBadgerBackend(config Config) (*BadgerBackend, error) {
	logger := log.With().Str("component", "badger-search").Logger()
	logger.Info().Msg("Creating BadgerDB search backend")

	return &BadgerBackend{
		config: config,
		logger: logger,
	}, nil
}

// BadgerBackend implements SearchBackend using BadgerDB
type BadgerBackend struct {
	// Original search implementation
	search *Search
	
	// Configuration
	config Config
	
	// Logger
	logger zerolog.Logger
}

// Start initializes the backend
func (b *BadgerBackend) Start(ctx context.Context) error {
	var err error
	
	// Create the search instance
	b.search, err = NewSearch(b.config)
	if err != nil {
		return fmt.Errorf("failed to create search: %w", err)
	}
	
	b.logger.Info().Msg("BadgerDB search backend started")
	return nil
}

// Index indexes a work unit
func (b *BadgerBackend) Index(ctx context.Context, unit *proto.WorkUnit) error {
	// Use the existing implementation
	batch := []*proto.WorkUnit{unit}
	b.search.indexBatch(batch)
	return nil
}

// IndexBatch indexes a batch of work units
func (b *BadgerBackend) IndexBatch(ctx context.Context, units []*proto.WorkUnit) error {
	// Use the existing implementation
	b.search.indexBatch(units)
	return nil
}

// Search performs a search with the given options
func (b *BadgerBackend) Search(ctx context.Context, options SearchOptions) (*SearchResults, error) {
	// Use the existing implementation
	ids, total, err := b.search.Search(
		ctx,
		options.Query,
		options.ContextID,
		options.AgentIDs,
		options.Types,
		options.MetaFilters,
		options.Limit,
		options.Offset,
	)
	
	if err != nil {
		return nil, err
	}
	
	results := &SearchResults{
		IDs:        ids,
		TotalCount: total,
		WorkUnits:  make([]*proto.WorkUnit, 0),
		Scores:     make(map[string]float64),
	}
	
	return results, nil
}

// FindByRelationship finds work units by relationship
func (b *BadgerBackend) FindByRelationship(ctx context.Context, relType string, targetID string, options SearchOptions) (*SearchResults, error) {
	// For BadgerDB, we implement relationships as metadata filters
	if options.MetaFilters == nil {
		options.MetaFilters = make(map[string]string)
	}
	
	// Add relationship filter
	options.MetaFilters["rel_"+relType] = targetID
	
	// Use regular search
	return b.Search(ctx, options)
}

// Shutdown cleans up resources
func (b *BadgerBackend) Shutdown(ctx context.Context) error {
	if b.search != nil {
		return b.search.Shutdown(ctx)
	}
	return nil
}