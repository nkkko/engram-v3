package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Prefix keys for Badger indexing
const (
	prefixWorkUnit        = "wu:"      // Work unit storage
	prefixTextIndex       = "text:"    // Text search index
	prefixContextIndex    = "ctx:"     // Context index
	prefixAgentIndex      = "agent:"   // Agent index
	prefixTypeIndex       = "type:"    // Type index
	prefixMetaIndex       = "meta:"    // Metadata index
	prefixTimestampSort   = "ts:"      // Timestamp sorting
	prefixRelationship    = "rel:"     // Relationship index (format: rel:type:target:id)
	prefixReverseRel      = "rrel:"    // Reverse relationship index (format: rrel:type:source:id)
)

// BadgerBackendConfig contains configuration for the Badger search backend
type BadgerBackendConfig struct {
	// Base directory for data files
	DataDir string

	// Batch size for indexing
	BatchSize int

	// Max wait time for batching
	MaxWaitTime time.Duration
}

// DefaultBadgerConfig returns default configuration
func DefaultBadgerConfig() BadgerBackendConfig {
	return BadgerBackendConfig{
		DataDir:     "./data",
		BatchSize:   100,
		MaxWaitTime: 200 * time.Millisecond,
	}
}

// BadgerBackend implements SearchBackend using BadgerDB
type BadgerBackend struct {
	config     BadgerBackendConfig
	db         *badger.DB
	mu         sync.RWMutex
	logger     zerolog.Logger
	indexQueue chan *proto.WorkUnit
}

// NewBadgerBackend creates a new BadgerDB-based search backend
func NewBadgerBackend(config BadgerBackendConfig) (*BadgerBackend, error) {
	logger := log.With().Str("component", "search-badger").Logger()

	// Apply defaults for any zero values
	if config.BatchSize == 0 {
		config.BatchSize = DefaultBadgerConfig().BatchSize
	}
	if config.MaxWaitTime == 0 {
		config.MaxWaitTime = DefaultBadgerConfig().MaxWaitTime
	}

	// Ensure data directory exists
	searchDir := filepath.Join(config.DataDir, "search")
	if err := os.MkdirAll(searchDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create search data directory: %w", err)
	}

	// Open Badger database with optimized settings
	opts := badger.DefaultOptions(searchDir)
	opts.Logger = nil // Disable Badger's logger
	opts.SyncWrites = false // For better indexing performance
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open Badger database: %w", err)
	}

	// Initialize backend
	b := &BadgerBackend{
		config:     config,
		db:         db,
		logger:     logger,
		indexQueue: make(chan *proto.WorkUnit, 1000),
	}

	return b, nil
}

// Start initializes the backend
func (b *BadgerBackend) Start(ctx context.Context) error {
	b.logger.Info().Msg("Starting Badger search backend")

	// Start processing index queue
	go b.processIndexQueue(ctx)

	return nil
}

// processIndexQueue handles batched indexing of work units
func (b *BadgerBackend) processIndexQueue(ctx context.Context) {
	batch := make([]*proto.WorkUnit, 0, b.config.BatchSize)
	ticker := time.NewTicker(b.config.MaxWaitTime)
	defer ticker.Stop()

	for {
		select {
		case unit, ok := <-b.indexQueue:
			if !ok {
				// Channel closed, index any remaining items
				if len(batch) > 0 {
					_ = b.IndexBatch(ctx, batch)
				}
				return
			}

			batch = append(batch, unit)
			if len(batch) >= b.config.BatchSize {
				_ = b.IndexBatch(ctx, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Process any accumulated items after the wait period
			if len(batch) > 0 {
				_ = b.IndexBatch(ctx, batch)
				batch = batch[:0]
			}

		case <-ctx.Done():
			// Context canceled, index any remaining items
			if len(batch) > 0 {
				_ = b.IndexBatch(ctx, batch)
			}
			return
		}
	}
}

// Index indexes a single work unit
func (b *BadgerBackend) Index(ctx context.Context, unit *proto.WorkUnit) error {
	// Send to the batch queue
	select {
	case b.indexQueue <- unit:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue full, index directly
		return b.IndexBatch(ctx, []*proto.WorkUnit{unit})
	}
}

// IndexBatch indexes a batch of work units
func (b *BadgerBackend) IndexBatch(ctx context.Context, units []*proto.WorkUnit) error {
	if len(units) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Start a write transaction
	err := b.db.Update(func(txn *badger.Txn) error {
		for _, unit := range units {
			// Skip if no ID
			if unit.Id == "" {
				continue
			}

			// Get timestamp
			var timestamp int64
			if unit.Ts != nil {
				timestamp = unit.Ts.AsTime().UnixNano()
			} else {
				timestamp = time.Now().UnixNano()
			}

			// Store the work unit
			workUnitKey := []byte(prefixWorkUnit + unit.Id)
			err := txn.Set(workUnitKey, MarshalWorkUnit(unit))
			if err != nil {
				b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to store work unit")
				continue
			}

			// Index by context ID
			contextKey := []byte(fmt.Sprintf("%s%s:%s", prefixContextIndex, unit.ContextId, unit.Id))
			if err := txn.Set(contextKey, nil); err != nil {
				b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index by context")
				continue
			}

			// Index by agent ID
			agentKey := []byte(fmt.Sprintf("%s%s:%s", prefixAgentIndex, unit.AgentId, unit.Id))
			if err := txn.Set(agentKey, nil); err != nil {
				b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index by agent")
				continue
			}

			// Index by type
			typeValue := int(unit.Type)
			typeKey := []byte(fmt.Sprintf("%s%d:%s", prefixTypeIndex, typeValue, unit.Id))
			if err := txn.Set(typeKey, nil); err != nil {
				b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index by type")
				continue
			}

			// Index by timestamp (for sorting)
			tsKey := []byte(fmt.Sprintf("%s%s:%020d:%s", prefixTimestampSort, unit.ContextId, timestamp, unit.Id))
			if err := txn.Set(tsKey, nil); err != nil {
				b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index by timestamp")
				continue
			}

			// Extract payload as string if it's text
			if unit.Payload != nil && isTextPayload(unit.Payload) {
				payloadStr := string(unit.Payload)
				// Index each word in the text
				words := splitIntoWords(payloadStr)
				for _, word := range words {
					if len(word) < 3 {
						continue // Skip short words
					}
					wordKey := []byte(fmt.Sprintf("%s%s:%s", prefixTextIndex, word, unit.Id))
					if err := txn.Set(wordKey, nil); err != nil {
						b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index text")
						continue
					}
				}
			}

			// Index by metadata
			for key, value := range unit.Meta {
				metaKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixMetaIndex, key, value, unit.Id))
				if err := txn.Set(metaKey, nil); err != nil {
					b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index metadata")
					continue
				}

				// Also index metadata values in text search for better searchability
				if len(value) >= 3 {
					for _, word := range splitIntoWords(value) {
						if len(word) < 3 {
							continue // Skip short words
						}
						wordKey := []byte(fmt.Sprintf("%s%s:%s", prefixTextIndex, word, unit.Id))
						if err := txn.Set(wordKey, nil); err != nil {
							b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index metadata text")
							continue
						}
					}
				}
			}

			// Index relationships
			for _, rel := range unit.Relationships {
				// Skip if type or target is empty
				if rel.Type == "" || rel.Target == "" {
					continue
				}

				// Index relationship (source → target)
				relKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixRelationship, rel.Type, rel.Target, unit.Id))
				if err := txn.Set(relKey, nil); err != nil {
					b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index relationship")
					continue
				}

				// Index reverse relationship (target → source)
				rrelKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixReverseRel, rel.Type, unit.Id, rel.Target))
				if err := txn.Set(rrelKey, nil); err != nil {
					b.logger.Error().Err(err).Str("id", unit.Id).Msg("Failed to index reverse relationship")
					continue
				}
			}
		}

		return nil
	})

	if err != nil {
		b.logger.Error().Err(err).Int("count", len(units)).Msg("Failed to index batch")
		return err
	}

	b.logger.Debug().Int("count", len(units)).Msg("Indexed work units batch")
	return nil
}

// FindByRelationship finds work units by relationship
func (b *BadgerBackend) FindByRelationship(ctx context.Context, relType string, targetID string, options SearchOptions) (*SearchResults, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	results := &SearchResults{
		IDs:        make([]string, 0),
		WorkUnits:  make([]*proto.WorkUnit, 0),
		TotalCount: 0,
		Scores:     make(map[string]float64),
	}

	// Check if context exists
	if options.Limit <= 0 {
		options.Limit = 10
	}

	// Find all work units that have a relationship of the given type with the target
	matchedIDs := make(map[string]bool)
	
	err := b.db.View(func(txn *badger.Txn) error {
		// Look for forward relationships (works with source → target)
		if targetID != "" {
			relPrefix := []byte(fmt.Sprintf("%s%s:%s:", prefixRelationship, relType, targetID))
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Seek(relPrefix); it.ValidForPrefix(relPrefix); it.Next() {
				item := it.Item()
				key := string(item.Key())
				parts := strings.Split(key, ":")
				if len(parts) >= 3 {
					id := parts[len(parts)-1]
					matchedIDs[id] = true
				}
			}
		} else {
			// If no target specified, match any relationship of this type
			relPrefix := []byte(fmt.Sprintf("%s%s:", prefixRelationship, relType))
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Seek(relPrefix); it.ValidForPrefix(relPrefix); it.Next() {
				item := it.Item()
				key := string(item.Key())
				parts := strings.Split(key, ":")
				if len(parts) >= 3 {
					id := parts[len(parts)-1]
					matchedIDs[id] = true
				}
			}
		}

		// Look for reverse relationships (works with target → source)
		if targetID != "" {
			rrelPrefix := []byte(fmt.Sprintf("%s%s:%s:", prefixReverseRel, relType, targetID))
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Seek(rrelPrefix); it.ValidForPrefix(rrelPrefix); it.Next() {
				item := it.Item()
				key := string(item.Key())
				parts := strings.Split(key, ":")
				if len(parts) >= 3 {
					targetRef := parts[len(parts)-1]
					matchedIDs[targetRef] = true
				}
			}
		}

		// Apply filters
		filteredIDs := make(map[string]bool)
		
		// If we have matches, filter them
		if len(matchedIDs) > 0 {
			// Filter by context
			if options.ContextID != "" {
				for id := range matchedIDs {
					contextKey := []byte(fmt.Sprintf("%s%s:%s", prefixContextIndex, options.ContextID, id))
					_, err := txn.Get(contextKey)
					if err == nil {
						filteredIDs[id] = true
					}
				}
				matchedIDs = filteredIDs
				filteredIDs = make(map[string]bool)
			}

			// Filter by agent
			if len(options.AgentIDs) > 0 {
				for id := range matchedIDs {
					for _, agentID := range options.AgentIDs {
						agentKey := []byte(fmt.Sprintf("%s%s:%s", prefixAgentIndex, agentID, id))
						_, err := txn.Get(agentKey)
						if err == nil {
							filteredIDs[id] = true
							break
						}
					}
				}
				matchedIDs = filteredIDs
				filteredIDs = make(map[string]bool)
			}

			// Filter by type
			if len(options.Types) > 0 {
				for id := range matchedIDs {
					for _, t := range options.Types {
						typeKey := []byte(fmt.Sprintf("%s%d:%s", prefixTypeIndex, int(t), id))
						_, err := txn.Get(typeKey)
						if err == nil {
							filteredIDs[id] = true
							break
						}
					}
				}
				matchedIDs = filteredIDs
				filteredIDs = make(map[string]bool)
			}

			// Filter by metadata
			if len(options.MetaFilters) > 0 {
				for id := range matchedIDs {
					match := true
					for key, value := range options.MetaFilters {
						metaKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixMetaIndex, key, value, id))
						_, err := txn.Get(metaKey)
						if err != nil {
							match = false
							break
						}
					}
					if match {
						filteredIDs[id] = true
					}
				}
				matchedIDs = filteredIDs
			}
		}

		// Get total count
		results.TotalCount = len(matchedIDs)

		// Convert to sorted list by timestamp
		if len(matchedIDs) > 0 {
			// Create a map of ID to timestamp for sorting
			tsMap := make(map[string]int64)

			// Collect timestamps for all IDs
			for id := range matchedIDs {
				var latestTs int64

				// Find timestamp for the ID
				opts := badger.DefaultIteratorOptions
				opts.PrefetchValues = false
				it := txn.NewIterator(opts)
				defer it.Close()

				// Look through timestamp index for this ID
				prefix := []byte(prefixTimestampSort)
				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					item := it.Item()
					key := string(item.Key())

					// Check if this key contains our ID
					if strings.HasSuffix(key, ":"+id) {
						// Extract timestamp from key
						parts := strings.Split(key, ":")
						if len(parts) >= 3 {
							ts, err := parseTimestampFromKey(parts[2])
							if err == nil && ts > latestTs {
								latestTs = ts
							}
						}
					}
				}

				tsMap[id] = latestTs
			}

			// Sort IDs by timestamp
			idsByTs := make([]struct {
				ID string
				TS int64
			}, 0, len(tsMap))

			for id, ts := range tsMap {
				idsByTs = append(idsByTs, struct {
					ID string
					TS int64
				}{id, ts})
			}

			// Sort in descending order
			sortByTimestampDesc(idsByTs)

			// Apply pagination
			start := options.Offset
			end := options.Offset + options.Limit
			if start >= len(idsByTs) {
				return nil
			}
			if end > len(idsByTs) {
				end = len(idsByTs)
			}

			// Extract IDs in order
			for i := start; i < end; i++ {
				id := idsByTs[i].ID
				results.IDs = append(results.IDs, id)

				// Get full work units if requested
				if options.IncludeFull {
					workUnitKey := []byte(prefixWorkUnit + id)
					item, err := txn.Get(workUnitKey)
					if err != nil {
						continue
					}

					var workUnit *proto.WorkUnit
					err = item.Value(func(val []byte) error {
						workUnit = UnmarshalWorkUnit(val)
						return nil
					})

					if err == nil && workUnit != nil {
						results.WorkUnits = append(results.WorkUnits, workUnit)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search relationships: %w", err)
	}

	return results, nil
}

// Search performs a search with the given options
func (b *BadgerBackend) Search(ctx context.Context, options SearchOptions) (*SearchResults, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	results := &SearchResults{
		IDs:        make([]string, 0),
		WorkUnits:  make([]*proto.WorkUnit, 0),
		TotalCount: 0,
		Scores:     make(map[string]float64),
	}

	// Set default limit if not specified
	if options.Limit <= 0 {
		options.Limit = 10
	}

	// Results map to deduplicate
	matchedIDs := make(map[string]bool)

	// Get all work unit IDs matching criteria
	err := b.db.View(func(txn *badger.Txn) error {
		// 1. Text search if query is provided
		if options.Query != "" {
			// Split query into words
			queryWords := splitIntoWords(options.Query)

			// For each word, collect matching IDs
			wordMatches := make(map[string][]string)
			for _, word := range queryWords {
				if len(word) < 3 {
					continue // Skip short words
				}

				var matches []string
				prefix := []byte(fmt.Sprintf("%s%s:", prefixTextIndex, word))
				it := txn.NewIterator(badger.DefaultIteratorOptions)
				defer it.Close()

				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					item := it.Item()
					// Extract ID from key
					key := string(item.Key())
					parts := strings.Split(key, ":")
					if len(parts) >= 2 {
						id := parts[len(parts)-1]
						matches = append(matches, id)
					}
				}

				if len(matches) > 0 {
					wordMatches[word] = matches
				}
			}

			// Intersect results from all words
			if len(wordMatches) > 0 {
				// Start with the first word's matches
				var baseMatches []string
				for _, matches := range wordMatches {
					baseMatches = matches
					break
				}

				// Intersect with other words
				for _, matches := range wordMatches {
					if len(baseMatches) == 0 {
						break
					}

					// Skip the first word which we already used
					if len(baseMatches) == len(matches) && equalSlices(baseMatches, matches) {
						continue
					}

					// Create map for fast lookup
					matchMap := make(map[string]bool)
					for _, id := range matches {
						matchMap[id] = true
					}

					// Filter base matches
					filteredMatches := make([]string, 0, len(baseMatches))
					for _, id := range baseMatches {
						if matchMap[id] {
							filteredMatches = append(filteredMatches, id)
						}
					}

					baseMatches = filteredMatches
				}

				// Add to results
				for _, id := range baseMatches {
					matchedIDs[id] = true
				}
			}
		}

		// 2. Filter by context
		if options.ContextID != "" {
			prefix := []byte(fmt.Sprintf("%s%s:", prefixContextIndex, options.ContextID))
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			// If no text search was performed, collect all context matches
			if options.Query == "" {
				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					item := it.Item()
					key := string(item.Key())
					parts := strings.Split(key, ":")
					if len(parts) >= 2 {
						id := parts[len(parts)-1]
						matchedIDs[id] = true
					}
				}
			} else {
				// Filter existing results by context
				filteredResults := make(map[string]bool)
				for id := range matchedIDs {
					contextKey := []byte(fmt.Sprintf("%s%s:%s", prefixContextIndex, options.ContextID, id))
					_, err := txn.Get(contextKey)
					if err == nil {
						filteredResults[id] = true
					}
				}
				matchedIDs = filteredResults
			}
		}

		// 3. Filter by agent IDs
		if len(options.AgentIDs) > 0 {
			// If no previous filters, collect all matches for all agents
			if options.Query == "" && options.ContextID == "" {
				for _, agentID := range options.AgentIDs {
					prefix := []byte(fmt.Sprintf("%s%s:", prefixAgentIndex, agentID))
					it := txn.NewIterator(badger.DefaultIteratorOptions)
					defer it.Close()

					for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
						item := it.Item()
						key := string(item.Key())
						parts := strings.Split(key, ":")
						if len(parts) >= 2 {
							id := parts[len(parts)-1]
							matchedIDs[id] = true
						}
					}
				}
			} else {
				// Filter existing results by agents
				filteredResults := make(map[string]bool)
				for id := range matchedIDs {
					for _, agentID := range options.AgentIDs {
						agentKey := []byte(fmt.Sprintf("%s%s:%s", prefixAgentIndex, agentID, id))
						_, err := txn.Get(agentKey)
						if err == nil {
							filteredResults[id] = true
							break
						}
					}
				}
				matchedIDs = filteredResults
			}
		}

		// 4. Filter by types
		if len(options.Types) > 0 {
			// If no previous filters, collect all matches for all types
			if options.Query == "" && options.ContextID == "" && len(options.AgentIDs) == 0 {
				for _, t := range options.Types {
					prefix := []byte(fmt.Sprintf("%s%d:", prefixTypeIndex, int(t)))
					it := txn.NewIterator(badger.DefaultIteratorOptions)
					defer it.Close()

					for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
						item := it.Item()
						key := string(item.Key())
						parts := strings.Split(key, ":")
						if len(parts) >= 2 {
							id := parts[len(parts)-1]
							matchedIDs[id] = true
						}
					}
				}
			} else {
				// Filter existing results by types
				filteredResults := make(map[string]bool)
				for id := range matchedIDs {
					for _, t := range options.Types {
						typeKey := []byte(fmt.Sprintf("%s%d:%s", prefixTypeIndex, int(t), id))
						_, err := txn.Get(typeKey)
						if err == nil {
							filteredResults[id] = true
							break
						}
					}
				}
				matchedIDs = filteredResults
			}
		}

		// 5. Filter by metadata
		if len(options.MetaFilters) > 0 {
			// If no previous filters, collect all matches for all metadata
			if options.Query == "" && options.ContextID == "" && len(options.AgentIDs) == 0 && len(options.Types) == 0 {
				// Start with first meta filter
				var firstKey string
				var firstValue string
				for k, v := range options.MetaFilters {
					firstKey = k
					firstValue = v
					break
				}

				prefix := []byte(fmt.Sprintf("%s%s:%s:", prefixMetaIndex, firstKey, firstValue))
				it := txn.NewIterator(badger.DefaultIteratorOptions)
				defer it.Close()

				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					item := it.Item()
					key := string(item.Key())
					parts := strings.Split(key, ":")
					if len(parts) >= 3 {
						id := parts[len(parts)-1]
						matchedIDs[id] = true
					}
				}

				// Make a copy of meta filters to avoid modifying the original
				remainingFilters := make(map[string]string)
				for k, v := range options.MetaFilters {
					if k != firstKey {
						remainingFilters[k] = v
					}
				}

				// Filter by remaining metadata
				if len(remainingFilters) > 0 {
					filteredResults := make(map[string]bool)
					for id := range matchedIDs {
						match := true
						for key, value := range remainingFilters {
							metaKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixMetaIndex, key, value, id))
							_, err := txn.Get(metaKey)
							if err != nil {
								match = false
								break
							}
						}
						if match {
							filteredResults[id] = true
						}
					}
					matchedIDs = filteredResults
				}
			} else {
				// Filter existing results by metadata
				filteredResults := make(map[string]bool)
				for id := range matchedIDs {
					match := true
					for key, value := range options.MetaFilters {
						metaKey := []byte(fmt.Sprintf("%s%s:%s:%s", prefixMetaIndex, key, value, id))
						_, err := txn.Get(metaKey)
						if err != nil {
							match = false
							break
						}
					}
					if match {
						filteredResults[id] = true
					}
				}
				matchedIDs = filteredResults
			}
		}

		// Get total count
		results.TotalCount = len(matchedIDs)

		// If no filters at all, count total items
		if options.Query == "" && options.ContextID == "" && len(options.AgentIDs) == 0 && 
		   len(options.Types) == 0 && len(options.MetaFilters) == 0 {
			   
			var allCount int
			prefix := []byte(prefixWorkUnit)
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				allCount++
			}
			
			if allCount > 0 {
				results.TotalCount = allCount
				
				// If we don't have results yet (empty query), fill with all IDs
				if len(matchedIDs) == 0 {
					opts := badger.DefaultIteratorOptions
					opts.PrefetchValues = false
					it := txn.NewIterator(opts)
					defer it.Close()
					
					for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
						item := it.Item()
						key := string(item.Key())
						id := strings.TrimPrefix(key, prefixWorkUnit)
						matchedIDs[id] = true
					}
				}
			}
		}

		// Convert to sorted list by timestamp
		if len(matchedIDs) > 0 {
			// Create a map of ID to timestamp for sorting
			tsMap := make(map[string]int64)

			// Collect timestamps for all IDs
			for id := range matchedIDs {
				var latestTs int64

				// Find timestamp for the ID
				opts := badger.DefaultIteratorOptions
				opts.PrefetchValues = false
				it := txn.NewIterator(opts)
				defer it.Close()

				// Look through timestamp index for this ID
				prefix := []byte(prefixTimestampSort)
				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					item := it.Item()
					key := string(item.Key())

					// Check if this key contains our ID
					if strings.HasSuffix(key, ":"+id) {
						// Extract timestamp from key
						parts := strings.Split(key, ":")
						if len(parts) >= 3 {
							ts, err := parseTimestampFromKey(parts[2])
							if err == nil && ts > latestTs {
								latestTs = ts
							}
						}
					}
				}

				tsMap[id] = latestTs
			}

			// Sort IDs by timestamp
			idsByTs := make([]struct {
				ID string
				TS int64
			}, 0, len(tsMap))

			for id, ts := range tsMap {
				idsByTs = append(idsByTs, struct {
					ID string
					TS int64
				}{id, ts})
			}

			// Sort in descending order
			sortByTimestampDesc(idsByTs)

			// Apply pagination
			start := options.Offset
			end := options.Offset + options.Limit
			if start >= len(idsByTs) {
				return nil
			}
			if end > len(idsByTs) {
				end = len(idsByTs)
			}

			// Extract IDs in order
			for i := start; i < end; i++ {
				id := idsByTs[i].ID
				results.IDs = append(results.IDs, id)

				// Get full work units if requested
				if options.IncludeFull {
					workUnitKey := []byte(prefixWorkUnit + id)
					item, err := txn.Get(workUnitKey)
					if err != nil {
						continue
					}

					var workUnit *proto.WorkUnit
					err = item.Value(func(val []byte) error {
						workUnit = UnmarshalWorkUnit(val)
						return nil
					})

					if err == nil && workUnit != nil {
						results.WorkUnits = append(results.WorkUnits, workUnit)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	return results, nil
}

// Shutdown cleans up resources
func (b *BadgerBackend) Shutdown(ctx context.Context) error {
	b.logger.Info().Msg("Shutting down Badger search backend")

	// Close index queue
	close(b.indexQueue)

	// Close database
	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return fmt.Errorf("failed to close search database: %w", err)
		}
	}

	return nil
}

// Utility functions

// MarshalWorkUnit serializes a work unit to bytes
func MarshalWorkUnit(unit *proto.WorkUnit) []byte {
	data, err := json.Marshal(unit)
	if err != nil {
		return nil
	}
	return data
}

// UnmarshalWorkUnit deserializes a work unit from bytes
func UnmarshalWorkUnit(data []byte) *proto.WorkUnit {
	var unit proto.WorkUnit
	if err := json.Unmarshal(data, &unit); err != nil {
		return nil
	}
	return &unit
}