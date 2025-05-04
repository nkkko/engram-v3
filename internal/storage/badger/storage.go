package badger

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/domain"
	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Ensure Storage implements domain.StorageEngine
var _ domain.StorageEngine = (*Storage)(nil)

const (
	// Prefix keys for different types (replacing column families)
	prefixDefault   = "d:"
	prefixWorkUnits = "wu:"
	prefixContexts  = "ctx:"
	prefixMeta      = "meta:"
	prefixLocks     = "lock:"

	// WAL file name
	walFileName = "wal.log"

	// WAL magic number for file identification
	walMagic = uint32(0x454E4752) // "ENGR" in ASCII
)

// Config contains storage configuration, same as the original
type Config struct {
	// Base directory for data files
	DataDir string

	// WAL sync interval
	WALSyncInterval time.Duration

	// WAL batch size
	WALBatchSize int
	
	// Cache settings
	CacheEnabled bool
	WorkUnitCacheSize int
	ContextCacheSize int
	LockCacheSize int
	CacheExpiration time.Duration
}

// DefaultConfig returns a default configuration for Badger-based storage
func DefaultConfig() Config {
	return Config{
		DataDir:               "./data",
		WALSyncInterval:       2 * time.Millisecond,
		WALBatchSize:          64,
		CacheEnabled:          true,
		WorkUnitCacheSize:     10000,
		ContextCacheSize:      1000,
		LockCacheSize:         5000,
		CacheExpiration:       30 * time.Second,
	}
}

// WALEntry represents a single write-ahead log entry
type WALEntry struct {
	Type      WALEntryType
	Timestamp time.Time
	Data      []byte
}

// WALEntryType defines the type of WAL entry
type WALEntryType uint8

const (
	WALWorkUnit WALEntryType = iota + 1
	WALContext
	WALLock
)

// Storage manages persistence of work units and contexts using Badger
type Storage struct {
	config      Config
	db          *badger.DB
	walFile     *os.File
	walLock     sync.Mutex
	walEntries  chan WALEntry
	walSync     chan struct{}
	walDone     chan struct{}
	eventStream chan *proto.WorkUnit
	cache       *Cache
	logger      zerolog.Logger
}

// NewStorage creates a new Storage instance using Badger
func NewStorage(config Config) (*Storage, error) {
	logger := log.With().Str("component", "storage-badger").Logger()

	// Apply default configuration values if not provided
	if config.WALBatchSize <= 0 {
		config.WALBatchSize = DefaultConfig().WALBatchSize
	}
	
	if config.WALSyncInterval <= 0 {
		config.WALSyncInterval = DefaultConfig().WALSyncInterval
	}

	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set up storage instance
	s := &Storage{
		config:      config,
		walEntries:  make(chan WALEntry, config.WALBatchSize*2), // Double buffer size
		walSync:     make(chan struct{}),
		walDone:     make(chan struct{}),
		eventStream: make(chan *proto.WorkUnit, 1000),
		logger:      logger,
	}

	// Open WAL file
	walPath := filepath.Join(config.DataDir, walFileName)
	var err error
	s.walFile, err = os.OpenFile(walPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	// Initialize Badger
	if err := s.initBadger(); err != nil {
		s.walFile.Close()
		return nil, err
	}

	// Initialize cache if enabled
	if config.CacheEnabled {
		// Apply default cache settings if not provided
		if config.WorkUnitCacheSize <= 0 {
			config.WorkUnitCacheSize = DefaultConfig().WorkUnitCacheSize
		}
		if config.ContextCacheSize <= 0 {
			config.ContextCacheSize = DefaultConfig().ContextCacheSize
		}
		if config.LockCacheSize <= 0 {
			config.LockCacheSize = DefaultConfig().LockCacheSize
		}
		if config.CacheExpiration <= 0 {
			config.CacheExpiration = DefaultConfig().CacheExpiration
		}
		
		cache, err := NewCache(
			config.WorkUnitCacheSize,
			config.ContextCacheSize,
			config.LockCacheSize,
			config.CacheExpiration,
		)
		if err != nil {
			s.db.Close()
			s.walFile.Close()
			return nil, fmt.Errorf("failed to initialize cache: %w", err)
		}
		s.cache = cache
		s.logger.Info().
			Int("work_unit_cache_size", config.WorkUnitCacheSize).
			Int("context_cache_size", config.ContextCacheSize).
			Int("lock_cache_size", config.LockCacheSize).
			Dur("cache_expiration", config.CacheExpiration).
			Msg("Cache initialized")
	}

	// Replay WAL if needed
	if err := s.replayWAL(); err != nil {
		if s.cache != nil {
			s.cache.Clear()
		}
		s.db.Close()
		s.walFile.Close()
		return nil, fmt.Errorf("failed to replay WAL: %w", err)
	}

	return s, nil
}

// initBadger initializes the Badger database
func (s *Storage) initBadger() error {
	dbPath := filepath.Join(s.config.DataDir, "badger")

	// Ensure directory exists
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create badger directory: %w", err)
	}

	// Configure Badger options
	options := badger.DefaultOptions(dbPath)
	options = options.WithLoggingLevel(badger.WARNING) // Reduce logging noise
	options = options.WithSyncWrites(false)           // We'll manage syncing via our WAL

	// Open the Badger database
	db, err := badger.Open(options)
	if err != nil {
		return fmt.Errorf("failed to open Badger: %w", err)
	}

	s.db = db
	return nil
}

// prefixKey adds the appropriate type prefix to a key
func prefixKey(prefix string, key []byte) []byte {
	prefixedKey := make([]byte, len(prefix)+len(key))
	copy(prefixedKey, prefix)
	copy(prefixedKey[len(prefix):], key)
	return prefixedKey
}

// replayWAL replays the WAL to ensure consistency
func (s *Storage) replayWAL() error {
	s.logger.Info().Msg("Replaying WAL")

	// Get WAL file info
	stat, err := s.walFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat WAL file: %w", err)
	}

	// Skip if WAL is empty
	if stat.Size() == 0 {
		s.logger.Info().Msg("WAL is empty, no replay needed")
		return nil
	}

	// Read and validate WAL header
	var magic uint32
	if err := binary.Read(s.walFile, binary.LittleEndian, &magic); err != nil {
		return fmt.Errorf("failed to read WAL header: %w", err)
	}

	if magic != walMagic {
		return fmt.Errorf("invalid WAL file format")
	}

	// Reset file position
	if _, err := s.walFile.Seek(4, 0); err != nil {
		return fmt.Errorf("failed to reset WAL file position: %w", err)
	}

	// Read WAL entries
	var entryType uint8
	var timestampNanos int64
	var dataLen uint32
	for {
		// Read entry header
		if err := binary.Read(s.walFile, binary.LittleEndian, &entryType); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read WAL entry type: %w", err)
		}

		if err := binary.Read(s.walFile, binary.LittleEndian, &timestampNanos); err != nil {
			return fmt.Errorf("failed to read WAL timestamp: %w", err)
		}

		if err := binary.Read(s.walFile, binary.LittleEndian, &dataLen); err != nil {
			return fmt.Errorf("failed to read WAL data length: %w", err)
		}

		// Read entry data
		data := make([]byte, dataLen)
		if _, err := io.ReadFull(s.walFile, data); err != nil {
			return fmt.Errorf("failed to read WAL entry data: %w", err)
		}

		// Process entry based on type
		switch WALEntryType(entryType) {
		case WALWorkUnit:
			var workUnit proto.WorkUnit
			if err := json.Unmarshal(data, &workUnit); err != nil {
				s.logger.Error().Err(err).Msg("Failed to unmarshal work unit from WAL")
				continue
			}
			if err := s.storeWorkUnitInternal(&workUnit, false); err != nil {
				s.logger.Error().Err(err).Str("id", workUnit.Id).Msg("Failed to restore work unit from WAL")
			}

		case WALContext:
			var context proto.Context
			if err := json.Unmarshal(data, &context); err != nil {
				s.logger.Error().Err(err).Msg("Failed to unmarshal context from WAL")
				continue
			}
			if err := s.storeContextInternal(&context, false); err != nil {
				s.logger.Error().Err(err).Str("id", context.Id).Msg("Failed to restore context from WAL")
			}

		case WALLock:
			var lock proto.LockHandle
			if err := json.Unmarshal(data, &lock); err != nil {
				s.logger.Error().Err(err).Msg("Failed to unmarshal lock from WAL")
				continue
			}
			if err := s.storeLockInternal(&lock, false); err != nil {
				s.logger.Error().Err(err).Str("path", lock.ResourcePath).Msg("Failed to restore lock from WAL")
			}
		}
	}

	s.logger.Info().Msg("WAL replay completed")
	return nil
}

// Start begins the storage engine operation
func (s *Storage) Start(ctx context.Context) error {
	// Initialize WAL file if needed
	stat, err := s.walFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat WAL file: %w", err)
	}

	if stat.Size() == 0 {
		// Write WAL header with magic number
		if err := binary.Write(s.walFile, binary.LittleEndian, walMagic); err != nil {
			return fmt.Errorf("failed to write WAL header: %w", err)
		}
		if err := s.walFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL header: %w", err)
		}
	}

	// Start WAL processing goroutine
	go s.processWAL(ctx)
	
	// Start metrics collection goroutine
	go s.collectMetrics(ctx)

	<-ctx.Done()
	close(s.walDone)
	return nil
}

// collectMetrics periodically collects and reports database metrics
func (s *Storage) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	
	m := metrics.GetMetrics()
	
	for {
		select {
		case <-ticker.C:
			// Collect DB size
			dbPath := filepath.Join(s.config.DataDir, "badger")
			dirSize, err := getDirSize(dbPath)
			if err == nil {
				m.DBSize.Set(float64(dirSize))
			}
			
			// Collect WAL size
			walPath := filepath.Join(s.config.DataDir, walFileName)
			walInfo, err := os.Stat(walPath)
			if err == nil {
				// Report WAL size as a label in storage operations
				m.StorageOperations.WithLabelValues("wal_size", "true").Add(float64(walInfo.Size()))
			}
			
			// Count active locks
			lockCount := 0
			err = s.db.View(func(txn *badger.Txn) error {
				it := txn.NewIterator(badger.DefaultIteratorOptions)
				defer it.Close()
				
				prefix := []byte(prefixLocks)
				for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
					lockCount++
				}
				return nil
			})
			
			if err == nil {
				m.LocksActiveGauge.Set(float64(lockCount))
			}
			
		case <-ctx.Done():
			return
		}
	}
}

// getDirSize returns the size of a directory and its subdirectories in bytes
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// processWAL handles WAL entry batching and syncing
func (s *Storage) processWAL(ctx context.Context) {
	// Pre-allocate batch slice to avoid reallocation
	batch := make([]WALEntry, 0, s.config.WALBatchSize)
	batchSize := s.config.WALBatchSize
	syncInterval := s.config.WALSyncInterval
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	// Track batch start time for adaptive syncing
	batchStartTime := time.Now()
	maxBatchTime := 5 * time.Millisecond // Maximum time to hold a batch before syncing

	for {
		select {
		case entry := <-s.walEntries:
			// Add entry to batch
			batch = append(batch, entry)
			
			// Check if batch is full or has been accumulating too long
			if len(batch) >= batchSize || time.Since(batchStartTime) > maxBatchTime {
				s.syncWALBatch(batch)
				batch = batch[:0]
				batchStartTime = time.Now()
			}

		case <-ticker.C:
			// Periodic flush of any pending entries
			if len(batch) > 0 {
				s.syncWALBatch(batch)
				batch = batch[:0]
				batchStartTime = time.Now()
			}

		case <-s.walSync:
			// Immediate sync requested
			if len(batch) > 0 {
				s.syncWALBatch(batch)
				batch = batch[:0]
				batchStartTime = time.Now()
			}

		case <-s.walDone:
			// Final flush before shutdown
			if len(batch) > 0 {
				s.syncWALBatch(batch)
			}
			return
		}

		// Try to drain more entries without blocking
		// This improves throughput by reducing the number of syncs
		drainLoop:
		for i := 0; i < batchSize-len(batch); i++ {
			select {
			case entry := <-s.walEntries:
				batch = append(batch, entry)
				if len(batch) >= batchSize {
					s.syncWALBatch(batch)
					batch = batch[:0]
					batchStartTime = time.Now()
					break drainLoop
				}
			default:
				// No more entries waiting
				break drainLoop
			}
		}
	}
}

// syncWALBatch writes and syncs a batch of WAL entries
func (s *Storage) syncWALBatch(entries []WALEntry) {
	if len(entries) == 0 {
		return
	}

	// Track metrics for WAL batch size
	m := metrics.GetMetrics()
	m.WALBatchSize.Observe(float64(len(entries)))
	
	syncStart := time.Now()
	s.walLock.Lock()
	defer s.walLock.Unlock()

	// Use a buffer to reduce the number of system calls
	// Estimate buffer size based on average entry size
	bufSize := 0
	for _, entry := range entries {
		// Each entry has: type (1 byte) + timestamp (8 bytes) + data length (4 bytes) + data
		bufSize += 1 + 8 + 4 + len(entry.Data)
	}

	// Create buffer with some extra space
	buf := bytes.NewBuffer(make([]byte, 0, bufSize+128))

	// Write all entries to the buffer
	for _, entry := range entries {
		// Write entry header
		binary.Write(buf, binary.LittleEndian, uint8(entry.Type))
		binary.Write(buf, binary.LittleEndian, entry.Timestamp.UnixNano())
		binary.Write(buf, binary.LittleEndian, uint32(len(entry.Data)))

		// Write entry data
		buf.Write(entry.Data)
	}

	// Write the entire buffer to the file in a single call
	if _, err := s.walFile.Write(buf.Bytes()); err != nil {
		s.logger.Error().Err(err).Msg("Failed to write WAL entries")
		m.StorageOperations.WithLabelValues("wal_write", "false").Inc()
		return
	}
	m.StorageOperations.WithLabelValues("wal_write", "true").Inc()

	// Sync to disk - use a configurable sync strategy to balance durability and performance
	if err := s.walFile.Sync(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to sync WAL file")
		m.StorageOperations.WithLabelValues("wal_sync", "false").Inc()
	} else {
		m.StorageOperations.WithLabelValues("wal_sync", "true").Inc()
	}
	
	// Record WAL sync duration
	m.WALSyncDuration.Observe(time.Since(syncStart).Seconds())
}

// Shutdown stops the storage engine
func (s *Storage) Shutdown(ctx context.Context) error {
	// Signal WAL processor to complete and wait
	close(s.walSync)
	select {
	case <-s.walDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Close WAL file
	if err := s.walFile.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing WAL file")
	}

	// Close Badger database
	if err := s.db.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing Badger database")
	}

	close(s.eventStream)
	return nil
}

// EventStream returns a channel of work unit events
func (s *Storage) EventStream() <-chan *proto.WorkUnit {
	return s.eventStream
}

// CreateWorkUnit creates a new work unit
func (s *Storage) CreateWorkUnit(ctx context.Context, req *proto.CreateWorkUnitRequest) (*proto.WorkUnit, error) {
	// Start operation timer
	m := metrics.GetMetrics()
	timer := prometheus.NewTimer(m.StorageOperationDuration.WithLabelValues("create_work_unit"))
	defer timer.ObserveDuration()
	
	// Generate a new UUID
	id := uuid.New().String()
	
	// Create work unit
	workUnit := &proto.WorkUnit{
		Id:            id,
		ContextId:     req.ContextId,
		AgentId:       req.AgentId,
		Type:          req.Type,
		Ts:            timestamppb.Now(),
		Meta:          req.Meta,
		Payload:       req.Payload,
		Relationships: req.Relationships,
	}

	// Store the work unit
	if err := s.storeWorkUnitInternal(workUnit, true); err != nil {
		m.StorageOperations.WithLabelValues("create_work_unit", "false").Inc()
		return nil, err
	}

	// Update metrics
	m.WorkUnitsTotal.Inc()
	m.StorageOperations.WithLabelValues("create_work_unit", "true").Inc()

	return workUnit, nil
}

// storeWorkUnitInternal stores a work unit and optionally records to WAL
func (s *Storage) storeWorkUnitInternal(workUnit *proto.WorkUnit, logToWAL bool) error {
	// Get metrics
	m := metrics.GetMetrics()
	
	// Serialize work unit
	data, err := json.Marshal(workUnit)
	if err != nil {
		m.StorageOperations.WithLabelValues("marshal_work_unit", "false").Inc()
		return fmt.Errorf("failed to marshal work unit: %w", err)
	}
	m.StorageOperations.WithLabelValues("marshal_work_unit", "true").Inc()

	// Log to WAL if requested
	if logToWAL {
		walData, _ := json.Marshal(workUnit)
		s.walEntries <- WALEntry{
			Type:      WALWorkUnit,
			Timestamp: time.Now(),
			Data:      walData,
		}
	}

	// Create a transaction
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	// Store primary record - with work unit prefix
	key := prefixKey(prefixWorkUnits, []byte(workUnit.Id))
	if err := txn.Set(key, data); err != nil {
		m.StorageOperations.WithLabelValues("store_work_unit", "false").Inc()
		return fmt.Errorf("failed to store work unit: %w", err)
	}

	// Create context-timestamp index
	contextKey := makeContextTimeKey(workUnit.ContextId, workUnit.Ts.AsTime())
	if err := txn.Set(contextKey, []byte(workUnit.Id)); err != nil {
		m.StorageOperations.WithLabelValues("create_work_unit_index", "false").Inc()
		return fmt.Errorf("failed to create context-time index: %w", err)
	}

	// Commit the transaction
	if err := txn.Commit(); err != nil {
		m.StorageOperations.WithLabelValues("store_work_unit", "false").Inc()
		return fmt.Errorf("failed to commit work unit transaction: %w", err)
	}

	// Send to event stream for real-time notifications
	eventStart := time.Now()
	select {
	case s.eventStream <- workUnit:
		// Calculate and observe event notification delay
		delay := time.Since(eventStart).Seconds()
		m.NotifierEventDelay.Observe(delay)
	default:
		s.logger.Warn().Str("id", workUnit.Id).Msg("Event stream buffer full, dropping work unit notification")
	}
	
	// Add to cache if enabled
	if s.config.CacheEnabled && s.cache != nil {
		s.cache.SetWorkUnit(workUnit)
	}

	m.StorageOperations.WithLabelValues("store_work_unit", "true").Inc()
	return nil
}

// makeContextTimeKey creates a composite key for context-timestamp indexing
// Using prefix format meta:{contextID}:{timestamp}
func makeContextTimeKey(contextID string, ts time.Time) []byte {
	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(ts.UnixNano()))
	
	// Create key with meta: prefix, followed by contextID, then a separator, then timestamp
	key := make([]byte, len(prefixMeta)+len(contextID)+1+8)
	copy(key, prefixMeta)
	copy(key[len(prefixMeta):], contextID)
	key[len(prefixMeta)+len(contextID)] = ':'
	copy(key[len(prefixMeta)+len(contextID)+1:], timeBytes)
	
	return key
}

// GetWorkUnit retrieves a work unit by ID
func (s *Storage) GetWorkUnit(ctx context.Context, id string) (*proto.WorkUnit, error) {
	// Start operation timer
	m := metrics.GetMetrics()
	timer := prometheus.NewTimer(m.StorageOperationDuration.WithLabelValues("get_work_unit"))
	defer timer.ObserveDuration()
	
	// Check cache first if enabled
	if s.config.CacheEnabled && s.cache != nil {
		if workUnit, found := s.cache.GetWorkUnit(id); found {
			return workUnit, nil
		}
	}
	
	// Create a read-only transaction
	err := s.db.View(func(txn *badger.Txn) error {
		// Get work unit from DB - with work unit prefix
		key := prefixKey(prefixWorkUnits, []byte(id))
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return fmt.Errorf("work unit not found: %s", id)
			}
			return fmt.Errorf("failed to retrieve work unit: %w", err)
		}
		
		// Get the actual value
		var valueData []byte
		err = item.Value(func(val []byte) error {
			valueData = append([]byte{}, val...)
			return nil
		})
		
		if err != nil {
			return fmt.Errorf("failed to read work unit value: %w", err)
		}
		
		// Unmarshal work unit
		var workUnit proto.WorkUnit
		if err := json.Unmarshal(valueData, &workUnit); err != nil {
			m.StorageOperations.WithLabelValues("get_work_unit", "false").Inc()
			return fmt.Errorf("failed to unmarshal work unit: %w", err)
		}
		
		// Add to cache if enabled
		if s.config.CacheEnabled && s.cache != nil {
			s.cache.SetWorkUnit(&workUnit)
		}
		
		// For tracking metrics
		m.StorageOperations.WithLabelValues("get_work_unit", "true").Inc()
		
		return nil
	})
	
	if err != nil {
		m.StorageOperations.WithLabelValues("get_work_unit", "false").Inc()
		return nil, err
	}
	
	// If we reach here without a workUnit, something went wrong
	m.StorageOperations.WithLabelValues("get_work_unit", "false").Inc()
	return nil, fmt.Errorf("work unit not found: %s", id)
}

// ListWorkUnits retrieves work units for a context
func (s *Storage) ListWorkUnits(ctx context.Context, contextID string, limit int, cursor string, reverse bool) ([]*proto.WorkUnit, string, error) {
	units := make([]*proto.WorkUnit, 0, limit)
	var nextCursor string

	err := s.db.View(func(txn *badger.Txn) error {
		// Define options for the iterator
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = reverse
		
		it := txn.NewIterator(opts)
		defer it.Close()

		// Create prefix for context-time index
		prefix := prefixMeta + contextID + ":"
		
		// Set up starting point
		var startKey []byte
		if cursor == "" {
			startKey = []byte(prefix)
			if reverse {
				// For reverse iteration, create a key that's just past the prefix range
				endKey := make([]byte, len(prefix)+8)
				copy(endKey, prefix)
				for i := len(prefix); i < len(endKey); i++ {
					endKey[i] = 0xFF
				}
				startKey = endKey
			}
		} else {
			startKey = []byte(cursor)
		}

		// Position iterator
		if reverse {
			it.Seek(startKey)
			// For reverse iteration with empty cursor, we need to position correctly
			if cursor == "" && it.Valid() && bytes.HasPrefix(it.Item().Key(), []byte(prefix)) {
				// Already positioned correctly at the highest key with the prefix
			}
		} else {
			it.Seek(startKey)
		}

		// Collect work units
		count := 0
		for ; it.Valid() && count < limit; it.Next() {
			key := it.Item().Key()
			
			// Check if we're still in the correct prefix
			if !bytes.HasPrefix(key, []byte(prefix)) {
				if reverse {
					// In reverse mode, we might have gone before our prefix
					continue
				} else {
					// In forward mode, we've gone past our prefix
					break
				}
			}

			// Get work unit ID from value
			var unitID string
			err := it.Item().Value(func(val []byte) error {
				unitID = string(val)
				return nil
			})
			
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to read work unit ID from index")
				continue
			}

			// Get the actual work unit
			unit, err := s.GetWorkUnit(ctx, unitID)
			if err != nil {
				s.logger.Error().Err(err).Str("id", unitID).Msg("Failed to retrieve work unit")
				continue
			}

			units = append(units, unit)
			count++
			
			// Keep track of the last key for cursor
			nextCursor = string(key)
		}

		// Set next cursor if more results might exist
		if count >= limit && it.Valid() && bytes.HasPrefix(it.Item().Key(), []byte(prefix)) {
			nextCursor = string(it.Item().Key())
		} else {
			// No more results
			nextCursor = ""
		}

		return nil
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to list work units: %w", err)
	}

	return units, nextCursor, nil
}

// UpdateContext updates or creates a context
func (s *Storage) UpdateContext(ctx context.Context, req *proto.UpdateContextRequest) (*proto.Context, error) {
	// Get existing context if it exists
	var existingContext *proto.Context
	var err error
	
	existingContext, err = s.GetContext(ctx, req.Id)
	if err == nil {
		// Update existing context
		context := existingContext
		
		// Update display name if provided
		if req.DisplayName != "" {
			context.DisplayName = req.DisplayName
		}
		
		// Add agents
		for _, agentID := range req.AddAgents {
			// Check if agent already exists
			exists := false
			for _, id := range context.AgentIds {
				if id == agentID {
					exists = true
					break
				}
			}
			
			if !exists {
				context.AgentIds = append(context.AgentIds, agentID)
			}
		}
		
		// Remove agents
		if len(req.RemoveAgents) > 0 {
			newAgents := make([]string, 0, len(context.AgentIds))
			for _, id := range context.AgentIds {
				remove := false
				for _, removeID := range req.RemoveAgents {
					if id == removeID {
						remove = true
						break
					}
				}
				
				if !remove {
					newAgents = append(newAgents, id)
				}
			}
			context.AgentIds = newAgents
		}
		
		// Pin units
		for _, unitID := range req.PinUnits {
			// Check if unit already pinned
			exists := false
			for _, id := range context.PinnedUnits {
				if id == unitID {
					exists = true
					break
				}
			}
			
			if !exists {
				context.PinnedUnits = append(context.PinnedUnits, unitID)
			}
		}
		
		// Unpin units
		if len(req.UnpinUnits) > 0 {
			newPinned := make([]string, 0, len(context.PinnedUnits))
			for _, id := range context.PinnedUnits {
				unpin := false
				for _, unpinID := range req.UnpinUnits {
					if id == unpinID {
						unpin = true
						break
					}
				}
				
				if !unpin {
					newPinned = append(newPinned, id)
				}
			}
			context.PinnedUnits = newPinned
		}
		
		// Update metadata
		for k, v := range req.Meta {
			context.Meta[k] = v
		}
		
		// Update timestamp
		context.UpdatedAt = timestamppb.Now()
		
		// Store the context
		if err := s.storeContextInternal(context, true); err != nil {
			return nil, err
		}
		
		return context, nil
	} else {
		// Create new context
		if req.Id == "" {
			req.Id = uuid.New().String()
		}
		
		context := &proto.Context{
			Id:          req.Id,
			DisplayName: req.DisplayName,
			AgentIds:    req.AddAgents,
			PinnedUnits: req.PinUnits,
			Meta:        req.Meta,
			CreatedAt:   timestamppb.Now(),
			UpdatedAt:   timestamppb.Now(),
		}
		
		// Store the context
		if err := s.storeContextInternal(context, true); err != nil {
			return nil, err
		}
		
		return context, nil
	}
}

// storeContextInternal stores a context and optionally records to WAL
func (s *Storage) storeContextInternal(context *proto.Context, logToWAL bool) error {
	// Serialize context
	data, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}
	
	// Log to WAL if requested
	if logToWAL {
		walData, _ := json.Marshal(context)
		s.walEntries <- WALEntry{
			Type:      WALContext,
			Timestamp: time.Now(),
			Data:      walData,
		}
	}
	
	// Create a transaction
	txn := s.db.NewTransaction(true)
	defer txn.Discard()
	
	// Store context record with prefix
	key := prefixKey(prefixContexts, []byte(context.Id))
	if err := txn.Set(key, data); err != nil {
		return fmt.Errorf("failed to store context: %w", err)
	}
	
	// Commit the transaction
	if err := txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit context transaction: %w", err)
	}
	
	// Add to cache if enabled
	if s.config.CacheEnabled && s.cache != nil {
		s.cache.SetContext(context)
	}
	
	return nil
}

// GetContext retrieves a context by ID
func (s *Storage) GetContext(ctx context.Context, id string) (*proto.Context, error) {
	// Check cache first if enabled
	if s.config.CacheEnabled && s.cache != nil {
		if context, found := s.cache.GetContext(id); found {
			return context, nil
		}
	}
	
	var context *proto.Context
	
	err := s.db.View(func(txn *badger.Txn) error {
		// Get context from DB with prefix
		key := prefixKey(prefixContexts, []byte(id))
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return fmt.Errorf("context not found: %s", id)
			}
			return fmt.Errorf("failed to retrieve context: %w", err)
		}
		
		// Read the value
		var valueData []byte
		err = item.Value(func(val []byte) error {
			valueData = append([]byte{}, val...)
			return nil
		})
		
		if err != nil {
			return fmt.Errorf("failed to read context value: %w", err)
		}
		
		// Unmarshal context
		context = &proto.Context{}
		if err := json.Unmarshal(valueData, context); err != nil {
			return fmt.Errorf("failed to unmarshal context: %w", err)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Add to cache if enabled
	if s.config.CacheEnabled && s.cache != nil && context != nil {
		s.cache.SetContext(context)
	}
	
	return context, nil
}

// AcquireLock attempts to acquire a lock on a resource
func (s *Storage) AcquireLock(ctx context.Context, req *proto.AcquireLockRequest) (*proto.LockHandle, error) {
	// Check if lock already exists
	existingLock, err := s.GetLock(ctx, req.ResourcePath)
	if err == nil {
		// Lock exists, check if it's expired
		if existingLock.ExpiresAt.AsTime().After(time.Now()) {
			// Lock is still valid
			if existingLock.HolderAgent != req.AgentId {
				return nil, fmt.Errorf("resource already locked by another agent")
			}
			// Same agent already has the lock, extend it
			existingLock.ExpiresAt = timestamppb.New(time.Now().Add(time.Duration(req.TtlSeconds) * time.Second))
			if err := s.storeLockInternal(existingLock, true); err != nil {
				return nil, err
			}
			return existingLock, nil
		}
		// Lock is expired, can be taken
	}
	
	// Create new lock
	lock := &proto.LockHandle{
		ResourcePath: req.ResourcePath,
		HolderAgent:  req.AgentId,
		ExpiresAt:    timestamppb.New(time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)),
		LockId:       uuid.New().String(),
	}
	
	// Store the lock
	if err := s.storeLockInternal(lock, true); err != nil {
		return nil, err
	}
	
	return lock, nil
}

// storeLockInternal stores a lock and optionally records to WAL
func (s *Storage) storeLockInternal(lock *proto.LockHandle, logToWAL bool) error {
	// Serialize lock
	data, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock: %w", err)
	}
	
	// Log to WAL if requested
	if logToWAL {
		walData, _ := json.Marshal(lock)
		s.walEntries <- WALEntry{
			Type:      WALLock,
			Timestamp: time.Now(),
			Data:      walData,
		}
	}
	
	// Create a transaction
	txn := s.db.NewTransaction(true)
	defer txn.Discard()
	
	// Store lock record with prefix
	key := prefixKey(prefixLocks, []byte(lock.ResourcePath))
	if err := txn.Set(key, data); err != nil {
		return fmt.Errorf("failed to store lock: %w", err)
	}
	
	// Commit the transaction
	if err := txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit lock transaction: %w", err)
	}
	
	// Add to cache if enabled
	if s.config.CacheEnabled && s.cache != nil {
		s.cache.SetLock(lock)
	}
	
	return nil
}

// GetLock retrieves a lock by resource path
func (s *Storage) GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error) {
	// Check cache first if enabled
	if s.config.CacheEnabled && s.cache != nil {
		if lock, found := s.cache.GetLock(resourcePath); found {
			return lock, nil
		}
	}
	
	var lock *proto.LockHandle
	
	err := s.db.View(func(txn *badger.Txn) error {
		// Get lock from DB with prefix
		key := prefixKey(prefixLocks, []byte(resourcePath))
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return fmt.Errorf("lock not found: %s", resourcePath)
			}
			return fmt.Errorf("failed to retrieve lock: %w", err)
		}
		
		// Read the value
		var valueData []byte
		err = item.Value(func(val []byte) error {
			valueData = append([]byte{}, val...)
			return nil
		})
		
		if err != nil {
			return fmt.Errorf("failed to read lock value: %w", err)
		}
		
		// Unmarshal lock
		lock = &proto.LockHandle{}
		if err := json.Unmarshal(valueData, lock); err != nil {
			return fmt.Errorf("failed to unmarshal lock: %w", err)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Add to cache if enabled
	if s.config.CacheEnabled && s.cache != nil && lock != nil {
		s.cache.SetLock(lock)
	}
	
	return lock, nil
}

// ReleaseLock releases a lock on a resource
func (s *Storage) ReleaseLock(ctx context.Context, req *proto.ReleaseLockRequest) (bool, error) {
	// Get existing lock
	existingLock, err := s.GetLock(ctx, req.ResourcePath)
	if err != nil {
		// Lock doesn't exist
		return false, nil
	}
	
	// Check if the requester is the lock holder
	if existingLock.HolderAgent != req.AgentId {
		return false, fmt.Errorf("lock is held by another agent")
	}
	
	// Check if lock IDs match
	if existingLock.LockId != req.LockId {
		return false, fmt.Errorf("lock ID mismatch")
	}
	
	// Delete the lock
	txn := s.db.NewTransaction(true)
	defer txn.Discard()
	
	key := prefixKey(prefixLocks, []byte(req.ResourcePath))
	if err := txn.Delete(key); err != nil {
		return false, fmt.Errorf("failed to delete lock: %w", err)
	}
	
	// Commit the transaction
	if err := txn.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit lock deletion: %w", err)
	}
	
	// Remove from cache if enabled
	if s.config.CacheEnabled && s.cache != nil {
		s.cache.DeleteLock(req.ResourcePath)
	}
	
	return true, nil
}

// Cache implementation for Badger storage
type Cache struct {
	workUnits     map[string]*proto.WorkUnit
	contexts      map[string]*proto.Context
	locks         map[string]*proto.LockHandle
	workUnitsMux  sync.RWMutex
	contextsMux   sync.RWMutex
	locksMux      sync.RWMutex
	expiration    time.Duration
	workUnitSize  int
	contextSize   int
	lockSize      int
}

// NewCache creates a new cache
func NewCache(workUnitSize, contextSize, lockSize int, expiration time.Duration) (*Cache, error) {
	if workUnitSize <= 0 || contextSize <= 0 || lockSize <= 0 || expiration <= 0 {
		return nil, fmt.Errorf("invalid cache parameters")
	}
	
	return &Cache{
		workUnits:    make(map[string]*proto.WorkUnit, workUnitSize),
		contexts:     make(map[string]*proto.Context, contextSize),
		locks:        make(map[string]*proto.LockHandle, lockSize),
		expiration:   expiration,
		workUnitSize: workUnitSize,
		contextSize:  contextSize,
		lockSize:     lockSize,
	}, nil
}

// SetWorkUnit adds or updates a work unit in the cache
func (c *Cache) SetWorkUnit(wu *proto.WorkUnit) {
	c.workUnitsMux.Lock()
	defer c.workUnitsMux.Unlock()
	
	c.workUnits[wu.Id] = wu
	
	// Simple cache eviction if too large
	if len(c.workUnits) > c.workUnitSize {
		// Evict random entry to keep size in check
		// In a real implementation, would use LRU or similar policy
		for k := range c.workUnits {
			delete(c.workUnits, k)
			break
		}
	}
}

// GetWorkUnit retrieves a work unit from the cache
func (c *Cache) GetWorkUnit(id string) (*proto.WorkUnit, bool) {
	c.workUnitsMux.RLock()
	defer c.workUnitsMux.RUnlock()
	
	wu, found := c.workUnits[id]
	return wu, found
}

// SetContext adds or updates a context in the cache
func (c *Cache) SetContext(ctx *proto.Context) {
	c.contextsMux.Lock()
	defer c.contextsMux.Unlock()
	
	c.contexts[ctx.Id] = ctx
	
	// Simple cache eviction if too large
	if len(c.contexts) > c.contextSize {
		// Evict random entry to keep size in check
		for k := range c.contexts {
			delete(c.contexts, k)
			break
		}
	}
}

// GetContext retrieves a context from the cache
func (c *Cache) GetContext(id string) (*proto.Context, bool) {
	c.contextsMux.RLock()
	defer c.contextsMux.RUnlock()
	
	ctx, found := c.contexts[id]
	return ctx, found
}

// SetLock adds or updates a lock in the cache
func (c *Cache) SetLock(lock *proto.LockHandle) {
	c.locksMux.Lock()
	defer c.locksMux.Unlock()
	
	c.locks[lock.ResourcePath] = lock
	
	// Simple cache eviction if too large
	if len(c.locks) > c.lockSize {
		// Evict random entry to keep size in check
		for k := range c.locks {
			delete(c.locks, k)
			break
		}
	}
}

// GetLock retrieves a lock from the cache
func (c *Cache) GetLock(resourcePath string) (*proto.LockHandle, bool) {
	c.locksMux.RLock()
	defer c.locksMux.RUnlock()
	
	lock, found := c.locks[resourcePath]
	return lock, found
}

// DeleteLock removes a lock from the cache
func (c *Cache) DeleteLock(resourcePath string) {
	c.locksMux.Lock()
	defer c.locksMux.Unlock()
	
	delete(c.locks, resourcePath)
}

// Clear empties the cache
func (c *Cache) Clear() {
	c.workUnitsMux.Lock()
	c.contextsMux.Lock()
	c.locksMux.Lock()
	defer c.workUnitsMux.Unlock()
	defer c.contextsMux.Unlock()
	defer c.locksMux.Unlock()
	
	c.workUnits = make(map[string]*proto.WorkUnit, c.workUnitSize)
	c.contexts = make(map[string]*proto.Context, c.contextSize)
	c.locks = make(map[string]*proto.LockHandle, c.lockSize)
}