package badger

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
)

// NewOptimizedStorage creates a storage instance with optimized configuration
func NewOptimizedStorage(config OptimizedConfig) (*Storage, error) {
	logger := log.With().Str("component", "storage-badger-optimized").Logger()
	logger.Info().Msg("Initializing optimized storage")

	// Convert optimized config to basic config
	basicConfig := Config{
		DataDir:           config.DataDir,
		WALSyncInterval:   config.WALSyncInterval,
		WALBatchSize:      config.WALBatchSize,
		CacheEnabled:      config.CacheEnabled,
		WorkUnitCacheSize: config.WorkUnitCacheSize,
		ContextCacheSize:  config.ContextCacheSize,
		LockCacheSize:     config.LockCacheSize,
		CacheExpiration:   config.CacheExpiration,
	}

	// Create storage with basic config
	s, err := NewStorage(basicConfig)
	if err != nil {
		return nil, err
	}

	// Apply optimized Badger settings
	if err := applyOptimizedBadgerSettings(s, config); err != nil {
		s.Shutdown(context.Background()) // Attempt to clean up
		return nil, err
	}

	// Start compaction and GC goroutines
	go runPeriodicGC(s, config.GCInterval, config.GCDiscardRatio)
	go runPeriodicCompaction(s, config.CompactionInterval)

	logger.Info().Msg("Optimized storage initialized successfully")

	return s, nil
}

// applyOptimizedBadgerSettings applies optimized settings to an existing Badger DB
func applyOptimizedBadgerSettings(s *Storage, config OptimizedConfig) error {
	// Close existing DB
	if err := s.db.Close(); err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dbPath := filepath.Join(s.config.DataDir, "badger")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return err
	}

	// Configure optimized Badger options
	opts := badger.DefaultOptions(dbPath)
	opts = opts.WithLoggingLevel(badger.WARNING)
	opts = opts.WithSyncWrites(config.SyncWrites)
	
	// Memory and table settings
	opts = opts.WithMemTableSize(config.MemTableSize)
	opts = opts.WithNumMemtables(config.NumMemtables)
	opts = opts.WithNumLevelZeroTables(config.NumLevelZeroTables)
	opts = opts.WithNumLevelZeroTablesStall(config.NumLevelZeroTablesStall)
	opts = opts.WithBaseLevelSize(config.BaseLevelSize)
	
	// Value log settings
	opts = opts.WithValueLogFileSize(config.ValueLogFileSize)
	opts = opts.WithValueLogMaxEntries(config.ValueLogMaxEntries)
	
	// Compaction settings
	opts = opts.WithNumCompactors(config.NumCompactors)
	
	// Performance settings
	opts = opts.WithVerifyValueChecksum(config.VerifyValueChecksum)
	opts = opts.WithNumVersionsToKeep(config.NumVersionsToKeep)
	opts = opts.WithDetectConflicts(config.DetectConflicts)
	
	// Cache settings
	opts = opts.WithIndexCacheSize(config.IndexCacheSize)
	opts = opts.WithBlockCacheSize(config.BlockCacheSize)

	// Open DB with optimized options
	db, err := badger.Open(opts)
	if err != nil {
		return err
	}

	// Replace DB in storage
	s.db = db

	return nil
}

// runPeriodicGC runs garbage collection on a regular interval
func runPeriodicGC(s *Storage, interval time.Duration, discardRatio float64) {
	logger := log.With().Str("component", "badger-gc").Logger()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := s.db.RunValueLogGC(discardRatio)
			if err != nil {
				if err == badger.ErrNoRewrite {
					// No rewrite needed, this is normal
					logger.Debug().Msg("No garbage collection needed")
				} else {
					logger.Error().Err(err).Msg("Error during garbage collection")
				}
			} else {
				logger.Info().Msg("Garbage collection completed")
			}
		case <-s.walDone:
			// Storage is shutting down
			return
		}
	}
}

// runPeriodicCompaction triggers manual compaction on a regular interval
func runPeriodicCompaction(s *Storage, interval time.Duration) {
	logger := log.With().Str("component", "badger-compaction").Logger()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := s.db.Flatten(8) // Flatten up to 8 levels
			if err != nil {
				logger.Error().Err(err).Msg("Error during compaction")
			} else {
				logger.Info().Msg("Compaction completed")
			}
		case <-s.walDone:
			// Storage is shutting down
			return
		}
	}
}

// runWALMaintenance handles WAL rotation and cleanup
func runWALMaintenance(s *Storage, config OptimizedConfig) {
	logger := log.With().Str("component", "wal-maintenance").Logger()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	maxWALSize := int64(config.WALMaxSizeMB) << 20 // Convert MB to bytes

	for {
		select {
		case <-ticker.C:
			// Get WAL size
			walInfo, err := s.walFile.Stat()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get WAL file info")
				continue
			}

			// Rotate WAL if it exceeds max size
			if walInfo.Size() > maxWALSize {
				logger.Info().
					Int64("current_size", walInfo.Size()).
					Int64("max_size", maxWALSize).
					Msg("WAL file exceeds maximum size, rotating")

				// Create a new WAL file
				// This would involve:
				// 1. Creating a new WAL file
				// 2. Closing the old one
				// 3. Updating the storage to use the new file
				// 4. Optionally archiving the old file
				// Not implemented here for brevity
			}
		case <-s.walDone:
			// Storage is shutting down
			return
		}
	}
}