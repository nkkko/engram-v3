package domain

import (
	"fmt"

	"github.com/nkkko/engram-v3/internal/storage/badger"
)

// StorageType represents the type of storage engine
type StorageType string

const (
	// BadgerStorage represents the Badger storage engine
	BadgerStorage StorageType = "badger"
	
	// InMemoryStorage represents an in-memory storage engine for testing
	InMemoryStorage StorageType = "memory"
)

// StorageConfig holds common configuration for all storage engines
type StorageConfig struct {
	// Storage type (badger, memory)
	Type StorageType

	// Specific engine configuration
	Config interface{}
}

// NewStorageEngine creates a new storage engine of the specified type
func NewStorageEngine(config StorageConfig) (StorageEngine, error) {
	switch config.Type {
	case BadgerStorage:
		// Convert config to badger-specific config
		var badgerConfig badger.Config
		
		// Check if a specific config was provided
		if config.Config != nil {
			var ok bool
			badgerConfig, ok = config.Config.(badger.Config)
			if !ok {
				return nil, fmt.Errorf("invalid configuration type for Badger storage")
			}
		} else {
			// Use default config
			badgerConfig = badger.DefaultConfig()
		}
		
		return badger.NewStorage(badgerConfig)
		
	case InMemoryStorage:
		// Future implementation of in-memory storage for testing
		return nil, fmt.Errorf("in-memory storage not yet implemented")
		
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}