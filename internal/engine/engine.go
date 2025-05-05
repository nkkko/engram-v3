package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nkkko/engram-v3/internal/api"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/logging"
	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage"
	"github.com/nkkko/engram-v3/internal/storage/badger"
	"github.com/nkkko/engram-v3/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// Config contains engine configuration parameters
type Config struct {
	// Data directory for storage
	DataDir string

	// HTTP server address
	ServerAddr string

	// Log level
	LogLevel string

	// WAL sync interval in milliseconds
	WALSyncIntervalMs int

	// WAL batch size (number of work units)
	WALBatchSize int

	// Lock TTL in seconds
	DefaultLockTTLSeconds int

	// Maximum idle connection time in seconds
	MaxIdleConnectionSeconds int
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		DataDir:                 "./data",
		ServerAddr:              ":8080",
		LogLevel:                "info",
		WALSyncIntervalMs:       2,
		WALBatchSize:            64,
		DefaultLockTTLSeconds:   60,
		MaxIdleConnectionSeconds: 30,
	}
}

// Engine is the main coordinator of all Engram components
type Engine struct {
	config      Config
	storage     storage.Storage
	router      *router.Router
	notifier    *notifier.Notifier
	lockMgr     *lockmanager.LockManager
	search      *search.Engine
	api         *api.API
	shutdownCh  chan struct{}
	wg          sync.WaitGroup
	logger      zerolog.Logger
	metrics     *metrics.Metrics
	telemetry   telemetry.Config
	telemetryFn func(context.Context) error // Shutdown function for telemetry
}

// CreateEngine creates a new Engine instance with all components initialized from the config
// This can take either a Config or an EngramEngineConfig (from the config package) for compatibility
func CreateEngine(config interface{}) (*Engine, error) {
	var engineConfig Config
	
	// Check if we're getting a config from the config package
	switch cfg := config.(type) {
	case Config:
		engineConfig = cfg
	case struct {
		DataDir                 string
		ServerAddr              string
		LogLevel                string
		WALSyncIntervalMs       int
		WALBatchSize            int
		DefaultLockTTLSeconds   int
		MaxIdleConnectionSeconds int
	}:
		// This handles the EngramEngineConfig from the config package
		engineConfig = Config{
			DataDir:                 cfg.DataDir,
			ServerAddr:              cfg.ServerAddr,
			LogLevel:                cfg.LogLevel,
			WALSyncIntervalMs:       cfg.WALSyncIntervalMs,
			WALBatchSize:            cfg.WALBatchSize,
			DefaultLockTTLSeconds:   cfg.DefaultLockTTLSeconds,
			MaxIdleConnectionSeconds: cfg.MaxIdleConnectionSeconds,
		}
	default:
		return nil, fmt.Errorf("unsupported configuration type")
	}
	// Ensure data directory exists
	if err := os.MkdirAll(engineConfig.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	
	// Initialize Badger storage
	storageConfig := badger.Config{
		DataDir:           engineConfig.DataDir,
		WALSyncInterval:   time.Duration(engineConfig.WALSyncIntervalMs) * time.Millisecond,
		WALBatchSize:      engineConfig.WALBatchSize,
		CacheEnabled:      true,
		WorkUnitCacheSize: 10000,
		ContextCacheSize:  1000,
		LockCacheSize:     5000,
		CacheExpiration:   30 * time.Second,
	}
	
	storage, err := badger.NewStorage(storageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Badger storage: %w", err)
	}
	
	// Initialize router
	r := router.NewRouter()
	
	// Initialize lock manager with configuration
	lockMgrConfig := lockmanager.Config{
		DefaultTTL:      time.Duration(engineConfig.DefaultLockTTLSeconds) * time.Second,
		CleanupInterval: time.Minute,
	}
	lockMgr := lockmanager.NewLockManager(lockMgrConfig, storage)
	
	// Initialize search with configuration
	searchConfig := search.Config{
		DataDir: engineConfig.DataDir,
	}
	
	// Vector search configuration
	vectorConfig := search.VectorConfig{
		Enabled: false, // Disabled by default in engine config
	}
	
	// Create the search engine
	searchEngine, err := search.NewEngine(searchConfig, vectorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize search engine: %w", err)
	}
	
	// Initialize notifier with router
	notifierConfig := notifier.Config{
		MaxIdleTime: time.Duration(engineConfig.MaxIdleConnectionSeconds) * time.Second,
	}
	notifier := notifier.NewNotifier(notifierConfig, r)
	
	// Initialize API
	apiConfig := api.Config{
		Addr: engineConfig.ServerAddr,
	}
	api := api.NewAPI(apiConfig, storage, r, notifier, lockMgr, searchEngine)
	
	// Create and return the engine
	return NewEngine(engineConfig, storage, r, lockMgr, searchEngine, api), nil
}

// NewEngine creates a new Engine instance with the given configuration and components
func NewEngine(config Config, storage storage.Storage, router *router.Router, lockMgr *lockmanager.LockManager, search *search.Engine, api *api.API) *Engine {
	// Configure logging
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	logger := log.With().Str("component", "engine").Logger()

	// Initialize metrics
	metricsInstance := metrics.GetMetrics()

	// Create engine with provided components
	e := &Engine{
		config:     config,
		storage:    storage,
		router:     router,
		lockMgr:    lockMgr,
		search:     search,
		api:        api,
		shutdownCh: make(chan struct{}),
		logger:     logger,
		metrics:    metricsInstance,
	}

	return e
}

// Start initializes and runs all components
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info().Msg("Starting Engram engine")
	
	// Initialize structured logging
	loggingConfig := logging.DefaultConfig()
	loggingConfig.Level = logging.LevelInfo
	if e.config.LogLevel == "debug" {
		loggingConfig.Level = logging.LevelDebug
	} else if e.config.LogLevel == "warn" {
		loggingConfig.Level = logging.LevelWarn
	} else if e.config.LogLevel == "error" {
		loggingConfig.Level = logging.LevelError
	}
	
	// Set up logging
	if err := logging.Setup(loggingConfig); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}
	
	// Initialize OpenTelemetry if enabled
	telConfig := telemetry.DefaultConfig()
	telConfig.Enabled = false // Set to true to enable telemetry
	telConfig.ServiceName = "engram"
	
	// Set up telemetry
	telShutdown, err := telemetry.Setup(ctx, telConfig)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to set up telemetry, continuing without it")
	} else {
		e.telemetryFn = telShutdown
		e.telemetry = telConfig
		e.logger.Info().Msg("Telemetry initialized successfully")
	}

	// Use the provided context or create a new one
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
		
		// Set up signal handling if we created the context
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		
		// Set up a goroutine to handle signals
		go func() {
			sig := <-sigCh
			e.logger.Info().Str("signal", sig.String()).Msg("Caught signal, initiating shutdown")
			cancel()
		}()
	}

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
	
	// Shut down telemetry if initialized
	if e.telemetryFn != nil {
		if err := e.telemetryFn(ctx); err != nil {
			e.logger.Error().Err(err).Msg("Failed to shut down telemetry")
		} else {
			e.logger.Info().Msg("Telemetry shut down successfully")
		}
	}

	return nil
}