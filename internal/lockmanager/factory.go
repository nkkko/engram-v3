package lockmanager

import (
	"time"

	"github.com/nkkko/engram-v3/internal/domain"
)

// ImplementationType defines the type of LockManager implementation
type ImplementationType string

const (
	// BasicImplementation represents the original basic implementation
	BasicImplementation ImplementationType = "basic"
	
	// EnhancedImplementation represents the enhanced implementation
	EnhancedImplementation ImplementationType = "enhanced"
)

// FactoryConfig contains configuration for the LockManager factory
type FactoryConfig struct {
	// ImplementationType specifies which implementation to use
	Type ImplementationType
	
	// Basic configuration
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	
	// Enhanced configuration
	MaxLocksPerAgent    int
	AcquisitionTimeout  time.Duration
	MaxRetries          int
	RetryDelay          time.Duration
	RetryBackoff        float64
	RetryJitter         time.Duration
}

// NewLockManagerFactory creates a new LockManager factory with the given configuration
func NewLockManagerFactory(config FactoryConfig, storage StorageInterface) domain.LockManager {
	switch config.Type {
	case EnhancedImplementation:
		// Use the enhanced implementation
		enhancedConfig := EnhancedConfig{
			DefaultTTL:         config.DefaultTTL,
			CleanupInterval:    config.CleanupInterval,
			MaxLocksPerAgent:   config.MaxLocksPerAgent,
			AcquisitionTimeout: config.AcquisitionTimeout,
			MaxRetries:         config.MaxRetries,
			RetryDelay:         config.RetryDelay,
			RetryBackoff:       config.RetryBackoff,
			RetryJitter:        config.RetryJitter,
		}
		return NewEnhancedLockManager(enhancedConfig, storage)
		
	case BasicImplementation:
		// Fall back to the basic implementation
		fallthrough
		
	default:
		// Use the basic implementation for backward compatibility
		basicConfig := Config{
			DefaultTTL:      config.DefaultTTL,
			CleanupInterval: config.CleanupInterval,
		}
		return NewLockManager(basicConfig, storage)
	}
}