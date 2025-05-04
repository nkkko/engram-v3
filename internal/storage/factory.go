package storage

import (
	"github.com/nkkko/engram-v3/internal/storage/badger"
)

// NewStorage creates a new storage instance
func NewStorage(config Config) (Storage, error) {
	// Convert config to Badger config
	badgerConfig := badger.Config{
		DataDir:           config.DataDir,
		WALSyncInterval:   config.WALSyncInterval,
		WALBatchSize:      config.WALBatchSize,
		CacheEnabled:      config.CacheEnabled,
		WorkUnitCacheSize: config.WorkUnitCacheSize,
		ContextCacheSize:  config.ContextCacheSize,
		LockCacheSize:     config.LockCacheSize,
		CacheExpiration:   config.CacheExpiration,
	}
	
	return badger.NewStorage(badgerConfig)
}