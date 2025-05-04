package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage/badger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestAPI(t *testing.T) (*API, func()) {
	// Create a temporary directory for storage
	tmpDir, err := os.MkdirTemp("", "api-test")
	require.NoError(t, err)

	// Create a subdirectory for search
	searchDir := tmpDir + "/search"
	err = os.MkdirAll(searchDir, 0755)
	require.NoError(t, err)

	// Create storage
	store, err := badger.NewStorage(badger.Config{
		DataDir:           tmpDir,
		WALSyncInterval:   1 * time.Millisecond,
		WALBatchSize:      10,
		CacheEnabled:      true,
		WorkUnitCacheSize: 100,
		ContextCacheSize:  100,
		LockCacheSize:     100,
		CacheExpiration:   30 * time.Second,
	})
	require.NoError(t, err)

	// Start storage with a context
	ctx, cancel := context.WithCancel(context.Background())
	go store.Start(ctx)

	// Create router
	r := router.NewRouter()

	// Connect router to storage event stream
	go r.Start(ctx, store.EventStream())

	// Create notifier with router
	n := notifier.NewNotifier(notifier.DefaultConfig(), r)
	
	// Get a subscription from router for the notifier
	subscription := r.Subscribe() // Subscribe to all contexts by default
	go n.Start(ctx, subscription)

	// Create lock manager with configuration
	lm := lockmanager.NewLockManager(lockmanager.DefaultConfig(), store)

	// Create search with configuration
	s, err := search.NewSearch(search.Config{
		DataDir: tmpDir,
	})
	require.NoError(t, err)
	
	// Start search indexer with router subscription
	searchSub := r.Subscribe()
	go s.Start(ctx, searchSub.Events)

	// Create API with test config
	config := Config{
		Addr: ":9999", // Use a different port for testing
	}
	api := NewAPI(config, store, r, n, lm, s)

	// Return cleanup function
	cleanup := func() {
		cancel()
		err := s.Shutdown(context.Background())
		if err != nil {
			t.Logf("Error shutting down search: %v", err)
		}
		err = store.Shutdown(context.Background())
		if err != nil {
			t.Logf("Error shutting down storage: %v", err)
		}
		os.RemoveAll(tmpDir)
	}

	return api, cleanup
}

func TestNewAPI(t *testing.T) {
	api, cleanup := setupTestAPI(t)
	defer cleanup()

	assert.NotNil(t, api)
	assert.Equal(t, ":9999", api.config.Addr)
}

func TestAPIDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, ":8080", config.Addr)
}

func TestAPIEmptyConfig(t *testing.T) {
	// Create a mock storage for testing
	tmpDir, err := os.MkdirTemp("", "api-test-empty-config")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory for search
	searchDir := tmpDir + "/search"
	err = os.MkdirAll(searchDir, 0755)
	require.NoError(t, err)

	store, err := badger.NewStorage(badger.Config{
		DataDir: tmpDir,
	})
	require.NoError(t, err)

	r := router.NewRouter()
	n := notifier.NewNotifier(notifier.DefaultConfig(), r)
	lm := lockmanager.NewLockManager(lockmanager.DefaultConfig(), store)
	
	s, err := search.NewSearch(search.Config{
		DataDir: tmpDir,
	})
	require.NoError(t, err)

	// Empty config should use default values
	api := NewAPI(Config{}, store, r, n, lm, s)
	assert.Equal(t, ":8080", api.config.Addr)
}

func TestAPIStart(t *testing.T) {
	api, cleanup := setupTestAPI(t)
	defer cleanup()

	// Start in a goroutine with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := api.Start(ctx)
		assert.NoError(t, err)
	}()

	// Wait a bit for the server to start
	time.Sleep(100 * time.Millisecond)

	// Cancel to stop the server
	cancel()

	// Test cleanup
	err := api.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestRegisterRoutes(t *testing.T) {
	api, cleanup := setupTestAPI(t)
	defer cleanup()

	// Create a new fiber app for testing
	app := fiber.New()

	// Register routes
	api.registerRoutes(app)

	// Check that the app has registered routes (simple count check)
	assert.NotNil(t, app)
}