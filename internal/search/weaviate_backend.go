package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WeaviateBackend implements the SearchBackend interface using Weaviate
type WeaviateBackend struct {
	client     *weaviate.Client
	embedding  *EmbeddingService
	config     VectorConfig
	classSetup bool
	logger     zerolog.Logger
}

// NewWeaviateBackend creates a new Weaviate search backend
func NewWeaviateBackend(config VectorConfig) (*WeaviateBackend, error) {
	logger := log.With().Str("component", "weaviate-search").Logger()

	// Create Weaviate client config
	cfg := weaviate.Config{
		Host:   config.WeaviateURL,
		Scheme: "http",
	}

	// Add API key if provided
	if config.WeaviateAPIKey != "" {
		cfg.Headers = map[string]string{
			"Authorization": "Bearer " + config.WeaviateAPIKey,
		}
		cfg.AuthConfig = auth.ApiKey{
			Value: config.WeaviateAPIKey,
		}
	}

	// Create client
	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	// Create embedding service
	embedding := NewEmbeddingService(
		config.EmbeddingModel,
		config.EmbeddingURL,
		config.EmbeddingKey,
	)

	return &WeaviateBackend{
		client:    client,
		embedding: embedding,
		config:    config,
		logger:    logger,
	}, nil
}

// setupClass ensures the required class exists in Weaviate
func (w *WeaviateBackend) setupClass(ctx context.Context) error {
	if w.classSetup {
		return nil
	}

	className := w.config.ClassName
	w.logger.Info().Str("class", className).Msg("Setting up Weaviate class")

	// Check if class exists
	classExists, err := w.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check class existence: %w", err)
	}

	// Delete class if it exists (for development only)
	if classExists {
		w.logger.Info().Str("class", className).Msg("Class already exists, deleting and recreating")
		err = w.client.Schema().ClassDeleter().WithClassName(className).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete existing class: %w", err)
		}
	}

	// Create class
	class := &models.Class{
		Class:      className,
		Vectorizer: "none", // We'll provide our own vectors
		Properties: []*models.Property{
			{
				Name:     "workUnitId",
				DataType: []string{"string"},
			},
			{
				Name:     "contextId",
				DataType: []string{"string"},
			},
			{
				Name:     "agentId",
				DataType: []string{"string"},
			},
			{
				Name:     "workUnitType",
				DataType: []string{"int"},
			},
			{
				Name:     "payload",
				DataType: []string{"text"},
			},
			{
				Name:     "metadata",
				DataType: []string{"object"},
			},
			{
				Name:     "timestamp",
				DataType: []string{"int"},
			},
		},
	}

	err = w.client.Schema().ClassCreator().WithClass(class).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create class: %w", err)
	}

	w.classSetup = true
	w.logger.Info().Str("class", className).Msg("Weaviate class setup complete")
	return nil
}

// Start initializes the backend
func (w *WeaviateBackend) Start(ctx context.Context) error {
	// Check if Weaviate is reachable
	ready, err := w.client.Misc().ReadyChecker().Do(ctx)
	if err != nil || !ready {
		return fmt.Errorf("Weaviate is not ready: %v", err)
	}

	// Set up the class
	if err := w.setupClass(ctx); err != nil {
		return err
	}

	w.logger.Info().Msg("Weaviate backend started successfully")
	return nil
}

// Index indexes a work unit
func (w *WeaviateBackend) Index(ctx context.Context, unit *proto.WorkUnit) error {
	return w.IndexBatch(ctx, []*proto.WorkUnit{unit})
}

// IndexBatch indexes a batch of work units
func (w *WeaviateBackend) IndexBatch(ctx context.Context, units []*proto.WorkUnit) error {
	if len(units) == 0 {
		return nil
	}

	// Ensure class is set up
	if err := w.setupClass(ctx); err != nil {
		return err
	}

	// Start batch operation
	batcher := w.client.Batch().ObjectsBatcher()
	className := w.config.ClassName

	for _, unit := range units {
		// Skip if no ID
		if unit.Id == "" {
			continue
		}

		// Extract timestamp
		var timestamp int64
		if unit.Ts != nil {
			timestamp = unit.Ts.AsTime().UnixNano()
		} else {
			timestamp = time.Now().UnixNano()
		}

		// Extract payload as text for embedding
		var payloadText string
		if unit.Payload != nil && isTextPayload(unit.Payload) {
			payloadText = string(unit.Payload)
		}

		// Generate embedding
		embedding, err := w.embedding.GetEmbedding(ctx, payloadText)
		if err != nil {
			w.logger.Warn().Err(err).Str("id", unit.Id).Msg("Failed to generate embedding")
			// Continue without embedding
		}

		// Convert metadata to JSON-compatible map
		metadata := make(map[string]interface{})
		for k, v := range unit.Meta {
			metadata[k] = v
		}

		// Create object
		obj := &models.Object{
			Class: className,
			ID:    uuid.NewString(), // Generate a new UUID for Weaviate
			Properties: map[string]interface{}{
				"workUnitId":   unit.Id,
				"contextId":    unit.ContextId,
				"agentId":      unit.AgentId,
				"workUnitType": int(unit.Type),
				"payload":      payloadText,
				"metadata":     metadata,
				"timestamp":    timestamp,
			},
			Vector: embedding,
		}

		// Add to batch
		batcher = batcher.WithObject(obj)
	}

	// Execute batch operation
	resp, err := batcher.Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to index batch: %w", err)
	}

	// Check for errors in response
	if len(resp) > 0 {
		var errMsgs []string
		for _, result := range resp {
			if result.Errors != nil && len(result.Errors.Error) > 0 {
				errMsgs = append(errMsgs, result.Errors.Error)
			}
		}
		if len(errMsgs) > 0 {
			return fmt.Errorf("batch indexing errors: %s", strings.Join(errMsgs, "; "))
		}
	}

	w.logger.Debug().Int("count", len(units)).Msg("Indexed work units in Weaviate")
	return nil
}

// Search performs a search with the given options
func (w *WeaviateBackend) Search(ctx context.Context, options SearchOptions) (*SearchResults, error) {
	className := w.config.ClassName

	// Create GraphQL fields
	fields := []graphql.Field{
		{Name: "workUnitId"},
	}

	// If we want the full work units
	if options.IncludeFull {
		fields = append(fields, []graphql.Field{
			{Name: "payload"},
			{Name: "contextId"},
			{Name: "agentId"},
			{Name: "workUnitType"},
			{Name: "timestamp"},
			{Name: "metadata"},
		}...)
	}

	// Start query builder
	queryBuilder := w.client.GraphQL().Get().WithClassName(className).WithFields(fields...)

	// Add filters based on search options
	var filters []string
	var filterValues []interface{}

	// Context filter
	if options.ContextID != "" {
		filters = append(filters, "contextId == $contextId")
		filterValues = append(filterValues, options.ContextID)
	}

	// Agent filter
	if len(options.AgentIDs) > 0 {
		if len(options.AgentIDs) == 1 {
			filters = append(filters, "agentId == $agentId")
			filterValues = append(filterValues, options.AgentIDs[0])
		} else {
			agentFilters := make([]string, len(options.AgentIDs))
			for i, agentID := range options.AgentIDs {
				paramName := fmt.Sprintf("agentId%d", i)
				agentFilters[i] = fmt.Sprintf("agentId == $%s", paramName)
				filterValues = append(filterValues, agentID)
			}
			filters = append(filters, "("+strings.Join(agentFilters, " or ")+")")
		}
	}

	// Type filter
	if len(options.Types) > 0 {
		if len(options.Types) == 1 {
			filters = append(filters, "workUnitType == $workUnitType")
			filterValues = append(filterValues, int(options.Types[0]))
		} else {
			typeFilters := make([]string, len(options.Types))
			for i, t := range options.Types {
				paramName := fmt.Sprintf("workUnitType%d", i)
				typeFilters[i] = fmt.Sprintf("workUnitType == $%s", paramName)
				filterValues = append(filterValues, int(t))
			}
			filters = append(filters, "("+strings.Join(typeFilters, " or ")+")")
		}
	}

	// Metadata filters
	if len(options.MetaFilters) > 0 {
		for key, value := range options.MetaFilters {
			filters = append(filters, fmt.Sprintf("metadata.%s == $meta_%s", key, key))
			filterValues = append(filterValues, value)
		}
	}

	// Combine filters
	if len(filters) > 0 {
		whereFilter := "where { " + strings.Join(filters, " and ") + " }"
		queryBuilder = queryBuilder.WithWhere(whereFilter)
	}

	// Add semantic search if requested
	if options.SemanticSearch && options.Query != "" {
		// Generate embedding for query
		embedding, err := w.embedding.GetEmbedding(ctx, options.Query)
		if err != nil {
			return nil, fmt.Errorf("failed to generate query embedding: %w", err)
		}

		// Use nearVector to find semantically similar items
		queryBuilder = queryBuilder.WithNearVector(graphql.NearVectorArgument{
			Vector:    embedding,
			Certainty: options.Similarity,
		})
	} else if options.Query != "" {
		// Use regular text search
		queryBuilder = queryBuilder.WithBM25(graphql.BM25Arguments{
			Query:      options.Query,
			Fields:     []string{"payload"},
			Properties: []string{"payload"},
		})
	}

	// Add pagination
	if options.Limit > 0 {
		queryBuilder = queryBuilder.WithLimit(options.Limit)
	}
	if options.Offset > 0 {
		queryBuilder = queryBuilder.WithOffset(options.Offset)
	}

	// Execute query
	result, err := queryBuilder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}

	// Process results
	searchResults := &SearchResults{
		IDs:        make([]string, 0),
		WorkUnits:  make([]*proto.WorkUnit, 0),
		TotalCount: 0,
		Scores:     make(map[string]float64),
	}

	// Parse data from result
	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return searchResults, nil
	}

	objects, ok := data[className].([]interface{})
	if !ok {
		return searchResults, nil
	}

	// Set total count
	searchResults.TotalCount = len(objects)

	// Extract data from objects
	for _, obj := range objects {
		objData, ok := obj.(map[string]interface{})
		if !ok {
			continue
		}

		// Get work unit ID
		idValue, ok := objData["workUnitId"]
		if !ok {
			continue
		}
		id, ok := idValue.(string)
		if !ok {
			continue
		}

		// Add ID to results
		searchResults.IDs = append(searchResults.IDs, id)

		// Get score if available
		if _score, ok := objData["_additional"].(map[string]interface{}); ok {
			if certainty, ok := _score["certainty"].(float64); ok {
				searchResults.Scores[id] = certainty
			}
		}

		// If full work units are requested
		if options.IncludeFull {
			// Construct work unit from data
			workUnit := &proto.WorkUnit{
				Id: id,
			}

			// Get context ID
			if contextID, ok := objData["contextId"].(string); ok {
				workUnit.ContextId = contextID
			}

			// Get agent ID
			if agentID, ok := objData["agentId"].(string); ok {
				workUnit.AgentId = agentID
			}

			// Get type
			if typeVal, ok := objData["workUnitType"].(float64); ok {
				workUnit.Type = proto.WorkUnitType(int32(typeVal))
			}

			// Get payload
			if payload, ok := objData["payload"].(string); ok {
				workUnit.Payload = []byte(payload)
			}

			// Get timestamp
			if ts, ok := objData["timestamp"].(float64); ok {
				tsTime := time.Unix(0, int64(ts))
				workUnit.Ts = timestamppb.New(tsTime)
			}

			// Get metadata
			if meta, ok := objData["metadata"].(map[string]interface{}); ok {
				workUnit.Meta = make(map[string]string)
				for k, v := range meta {
					if strVal, ok := v.(string); ok {
						workUnit.Meta[k] = strVal
					} else {
						// Convert non-string values to JSON
						jsonVal, err := json.Marshal(v)
						if err == nil {
							workUnit.Meta[k] = string(jsonVal)
						}
					}
				}
			}

			searchResults.WorkUnits = append(searchResults.WorkUnits, workUnit)
		}
	}

	return searchResults, nil
}

// FindByRelationship finds work units by relationship
func (w *WeaviateBackend) FindByRelationship(ctx context.Context, relType string, targetID string, options SearchOptions) (*SearchResults, error) {
	// For Weaviate, we implement relationships as metadata filters
	if options.MetaFilters == nil {
		options.MetaFilters = make(map[string]string)
	}

	// Add relationship filter
	options.MetaFilters["relation_"+relType] = targetID

	// Use regular search
	return w.Search(ctx, options)
}

// Shutdown cleans up resources
func (w *WeaviateBackend) Shutdown(ctx context.Context) error {
	w.logger.Info().Msg("Shutting down Weaviate backend")

	// Close embedding service
	w.embedding.Close()

	return nil
}
