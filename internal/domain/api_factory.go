package domain

import (
	"fmt"
	"time"

	"github.com/nkkko/engram-v3/internal/api/chi"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/search"
)

// APIType represents the type of API implementation
type APIType string

const (
	// ChiAPI represents the Chi router-based API
	ChiAPI APIType = "chi"
	
	// FiberAPI represents the Fiber framework-based API
	FiberAPI APIType = "fiber"
)

// APIConfig holds common configuration for all API implementations
type APIConfig struct {
	// API type
	Type APIType
	
	// Server address
	Addr string
	
	// Timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewAPIEngine creates a new API engine of the specified type
func NewAPIEngine(
	config APIConfig, 
	storage StorageEngine,
	router EventRouter,
	notifier *notifier.Notifier,
	lockManager *lockmanager.LockManager,
	search *search.Search,
) (APIEngine, error) {
	switch config.Type {
	case ChiAPI:
		chiConfig := chi.Config{
			Addr:         config.Addr,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			IdleTimeout:  config.IdleTimeout,
		}
		
		return chi.NewChiAPI(chiConfig, storage, router, notifier, lockManager, search), nil
		
	case FiberAPI:
		// Keep existing Fiber API implementation for backward compatibility
		return nil, fmt.Errorf("fiber API implementation not wrapped yet")
		
	default:
		return nil, fmt.Errorf("unsupported API type: %s", config.Type)
	}
}