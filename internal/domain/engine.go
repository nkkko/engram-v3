package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// EngineConfig contains configuration for the engine
type EngineConfig struct {
	// Storage configuration
	Storage StorageConfig
	
	// Router configuration
	Router RouterConfig
	
	// API configuration
	API APIConfig
	
	// Lock manager configuration
	LockTTL time.Duration
	
	// Data directory
	DataDir string
	
	// Log level
	LogLevel string
}

// Engine is the main coordinator of all Engram components
type Engine struct {
	config      EngineConfig
	storage     StorageEngine
	router      EventRouter
	notifier    *notifier.Notifier
	lockMgr     *lockmanager.LockManager
	search      *search.Search
	api         APIEngine
	logger      zerolog.Logger
}

// NewEngine creates a new Engine with the given configuration
func NewEngine(config EngineConfig) (*Engine, error) {
	logger := log.With().Str("component", "engine").Logger()
	
	// Configure logging
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	
	// Initialize storage
	storage, err := NewStorageEngine(config.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}
	
	// Initialize router
	eventRouter, err := NewEventRouter(config.Router)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize router: %w", err)
	}
	
	// Initialize lock manager with enhanced implementation
	lockMgrConfig := lockmanager.FactoryConfig{
		Type:              lockmanager.EnhancedImplementation,
		DefaultTTL:        config.LockTTL,
		CleanupInterval:   time.Minute,
		MaxLocksPerAgent:  100,
		AcquisitionTimeout: 5 * time.Second,
		MaxRetries:        3,
		RetryDelay:        100 * time.Millisecond,
		RetryBackoff:      1.5,
		RetryJitter:       20 * time.Millisecond,
	}
	lockMgr := lockmanager.NewLockManagerFactory(lockMgrConfig, storage)
	
	// Initialize search
	searchConfig := search.Config{
		DataDir: config.DataDir,
	}
	search, err := search.NewSearch(searchConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize search: %w", err)
	}
	
	// Initialize notifier
	notifierConfig := notifier.Config{
		MaxIdleTime: 30 * time.Second,
	}
	notifier := notifier.NewNotifier(notifierConfig, eventRouter.(*router.Router))
	
	// Initialize API
	api, err := NewAPIEngine(
		config.API,
		storage,
		eventRouter,
		notifier,
		lockMgr,
		search,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API: %w", err)
	}
	
	// Create engine
	engine := &Engine{
		config:   config,
		storage:  storage,
		router:   eventRouter,
		notifier: notifier,
		lockMgr:  lockMgr,
		search:   search,
		api:      api,
		logger:   logger,
	}
	
	return engine, nil
}

// Start initializes and runs all components
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info().Msg("Starting Engram engine")
	
	// Create an error group for managing goroutines
	g, ctx := errgroup.WithContext(ctx)
	
	// Start the storage engine
	g.Go(func() error {
		return e.storage.Start(ctx)
	})
	
	// Connect the event stream
	events := e.storage.EventStream()
	
	// Start the router with events
	g.Go(func() error {
		return e.router.Start(ctx, events)
	})
	
	// Create a subscription for the notifier and start it
	subscription := e.router.Subscribe("*")
	g.Go(func() error {
		return e.notifier.Start(ctx, subscription)
	})
	
	// Start the API server
	g.Go(func() error {
		return e.api.Start(ctx)
	})
	
	// Start the lock manager
	g.Go(func() error {
		return e.lockMgr.Start(ctx)
	})
	
	// Create a subscription for the search engine and start it
	searchSubscription := e.router.Subscribe()
	g.Go(func() error {
		return e.search.Start(ctx, searchSubscription.Events)
	})
	
	// Wait for all goroutines to finish
	if err := g.Wait(); err != nil && err != context.Canceled {
		return fmt.Errorf("error running engine: %w", err)
	}
	
	e.logger.Info().Msg("Engram engine shut down successfully")
	return nil
}

// Shutdown stops the engine
func (e *Engine) Shutdown(ctx context.Context) error {
	e.logger.Info().Msg("Shutting down Engram engine")
	
	// Shut down API server first to stop accepting new connections
	if err := e.api.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down API")
	}
	
	// Shut down router to stop command processing
	if err := e.router.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down router")
	}
	
	// Shut down notifier
	if err := e.notifier.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down notifier")
	}
	
	// Shut down lock manager
	if err := e.lockMgr.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down lock manager")
	}
	
	// Shut down search engine
	if err := e.search.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down search")
	}
	
	// Shut down storage last
	if err := e.storage.Shutdown(ctx); err != nil {
		e.logger.Error().Err(err).Msg("Failed to shut down storage")
		return err
	}
	
	return nil
}