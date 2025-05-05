package search

import (
	"context"
	"fmt"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Engine is the main search engine coordinator
type Engine struct {
	backend  SearchBackend
	config   Config
	vecConfig VectorConfig
	logger   zerolog.Logger
}

// NewEngine creates a new search engine
func NewEngine(config Config, vecConfig VectorConfig) (*Engine, error) {
	logger := log.With().Str("component", "search-engine").Logger()

	// Create factory
	factory := NewEngineFactory(config, vecConfig)
	
	// Choose engine type
	engineType := BadgerEngine
	if vecConfig.Enabled {
		engineType = WeaviateEngine
		logger.Info().Msg("Vector search enabled, using Weaviate backend")
	} else {
		logger.Info().Msg("Vector search disabled, using BadgerDB backend")
	}
	
	// Create backend
	backend, err := factory.CreateEngine(engineType)
	if err != nil {
		return nil, fmt.Errorf("failed to create search backend: %w", err)
	}
	
	return &Engine{
		backend:  backend,
		config:   config,
		vecConfig: vecConfig,
		logger:   logger,
	}, nil
}

// Start initializes the search engine
func (e *Engine) Start(ctx context.Context, events <-chan *proto.WorkUnit) error {
	e.logger.Info().Msg("Starting search engine")
	
	// Start backend
	if err := e.backend.Start(ctx); err != nil {
		return fmt.Errorf("failed to start search backend: %w", err)
	}
	
	// Process events from storage
	go func() {
		for {
			select {
			case event, ok := <-events:
				if !ok {
					e.logger.Info().Msg("Event stream closed, stopping indexer")
					return
				}
				
				// Index the event
				if err := e.backend.Index(ctx, event); err != nil {
					e.logger.Error().Err(err).Str("id", event.Id).Msg("Failed to index work unit")
				}
				
			case <-ctx.Done():
				e.logger.Info().Msg("Context canceled, stopping indexer")
				return
			}
		}
	}()
	
	return nil
}

// Search searches for work units
func (e *Engine) Search(ctx context.Context, query string, contextID string, agentIDs []string, types []proto.WorkUnitType, metaFilters map[string]string, limit int, offset int) ([]string, int, error) {
	options := SearchOptions{
		Query:      query,
		ContextID:  contextID,
		AgentIDs:   agentIDs,
		Types:      types,
		MetaFilters: metaFilters,
		Limit:      limit,
		Offset:     offset,
	}
	
	// Use backend to perform search
	results, err := e.backend.Search(ctx, options)
	if err != nil {
		return nil, 0, err
	}
	
	return results.IDs, results.TotalCount, nil
}

// SearchSemantic performs semantic search using vector embeddings
func (e *Engine) SearchSemantic(ctx context.Context, query string, contextID string, similarity float64, limit int) ([]string, []float64, error) {
	options := SearchOptions{
		Query:         query,
		ContextID:     contextID,
		SemanticSearch: true,
		Similarity:    similarity,
		Limit:         limit,
	}
	
	// Use backend to perform search
	results, err := e.backend.Search(ctx, options)
	if err != nil {
		return nil, nil, err
	}
	
	// Extract scores
	scores := make([]float64, len(results.IDs))
	for i, id := range results.IDs {
		scores[i] = results.Scores[id]
	}
	
	return results.IDs, scores, nil
}

// FindByRelationship finds work units related to the given ID
func (e *Engine) FindByRelationship(ctx context.Context, relType string, targetID string, limit int) ([]string, error) {
	options := SearchOptions{
		Limit: limit,
	}
	
	// Use backend to perform relationship search
	results, err := e.backend.FindByRelationship(ctx, relType, targetID, options)
	if err != nil {
		return nil, err
	}
	
	return results.IDs, nil
}

// Shutdown performs cleanup and stops the search engine
func (e *Engine) Shutdown(ctx context.Context) error {
	e.logger.Info().Msg("Shutting down search engine")
	
	// Shutdown backend
	if e.backend != nil {
		return e.backend.Shutdown(ctx)
	}
	
	return nil
}