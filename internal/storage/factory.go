package storage

import (
	"github.com/nkkko/engram-v3/internal/storage/badger"
)

// StorageType represents the type of storage implementation to use
type StorageType string

const (
	// BadgerStorage is the default storage type
	BadgerStorage StorageType = "badger"
	
	// OptimizedBadgerStorage uses enhanced configuration for better performance
	OptimizedBadgerStorage StorageType = "badger-optimized"
)

// FactoryConfig contains configuration for the storage factory
type FactoryConfig struct {
	// Storage type to create
	Type StorageType
	
	// Basic configuration
	Config Config
	
	// Enhanced configuration for optimized storage
	OptimizedConfig badger.OptimizedConfig
}

// DefaultFactoryConfig returns the default factory configuration
func DefaultFactoryConfig() FactoryConfig {
	return FactoryConfig{
		Type:   BadgerStorage,
		Config: DefaultConfig(),
		OptimizedConfig: badger.OptimizedDefaultConfig(),
	}
}

// NewStorage creates a new storage instance with the default storage type
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

// CreateStorage creates a storage instance based on the factory configuration
func CreateStorage(config FactoryConfig) (Storage, error) {
	switch config.Type {
	case BadgerStorage:
		// Convert config to Badger config
		badgerConfig := badger.Config{
			DataDir:           config.Config.DataDir,
			WALSyncInterval:   config.Config.WALSyncInterval,
			WALBatchSize:      config.Config.WALBatchSize,
			CacheEnabled:      config.Config.CacheEnabled,
			WorkUnitCacheSize: config.Config.WorkUnitCacheSize,
			ContextCacheSize:  config.Config.ContextCacheSize,
			LockCacheSize:     config.Config.LockCacheSize,
			CacheExpiration:   config.Config.CacheExpiration,
		}
		
		return badger.NewStorage(badgerConfig)
		
	case OptimizedBadgerStorage:
		// Use optimized Badger configuration
		optimizedConfig := config.OptimizedConfig
		
		// Override data directory from basic config if set
		if config.Config.DataDir != "" {
			optimizedConfig.DataDir = config.Config.DataDir
		}
		
		return badger.NewOptimizedStorage(optimizedConfig)
		
	default:
		// Default to regular Badger storage
		return NewStorage(config.Config)
	}
}