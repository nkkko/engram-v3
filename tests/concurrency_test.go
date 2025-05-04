package tests

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/api"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage/badger"
	"github.com/nkkko/engram-v3/pkg/client"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestConcurrentWorkUnitCreation tests concurrent creation of work units
func TestConcurrentWorkUnitCreation(t *testing.T) {
	t.Skip("Skipping test temporarily while fixing architecture")
	if testing.Short() {
		t.Skip("Skipping concurrent work unit creation test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-concurrency-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Set up server instance with optimized components
	serverInstance, _, cancel, err := setupTestServer(tempDir, 10)
	require.NoError(t, err, "Failed to set up test server")
	defer cancel()

	// Wait for server to be ready
	time.Sleep(1 * time.Second)

	// Create client
	clientInstance := client.New(
		fmt.Sprintf("http://localhost:%d", serverInstance.Port),
		client.WithAgentID("concurrency-test-agent"),
		client.WithTimeout(10*time.Second),
	)

	// Create a context to use for all work units
	ctx := context.Background()
	contextID := fmt.Sprintf("concurrent-context-%s", uuid.New().String()[:8])
	_, err = clientInstance.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "Concurrency Test Context",
	})
	require.NoError(t, err, "Failed to create context")

	// Number of goroutines and work units per goroutine
	const numClients = 10
	const unitsPerClient = 100
	const totalUnits = numClients * unitsPerClient

	// Track created work units
	var createdUnits sync.Map

	// Start concurrent clients
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Each client creates multiple work units
			for j := 0; j < unitsPerClient; j++ {
				req := &proto.CreateWorkUnitRequest{
					ContextId: contextID,
					AgentId:   fmt.Sprintf("client-%d", clientID),
					Type:      proto.WorkUnitType_MESSAGE,
					Payload:   []byte(fmt.Sprintf("Concurrent test message from client %d, unit %d", clientID, j)),
					Meta: map[string]string{
						"client_id": fmt.Sprintf("%d", clientID),
						"unit_id":   fmt.Sprintf("%d", j),
					},
				}

				workUnit, err := clientInstance.CreateWorkUnit(ctx, req)
				if err != nil {
					t.Logf("Error creating work unit: %v", err)
					continue
				}

				// Store the work unit ID
				createdUnits.Store(workUnit.Id, true)
			}
		}(i)
	}

	// Wait for all clients to finish
	wg.Wait()

	// Count created units
	var unitCount int
	createdUnits.Range(func(_, _ interface{}) bool {
		unitCount++
		return true
	})

	// Verify at least 90% success rate
	successRate := float64(unitCount) / float64(totalUnits)
	t.Logf("Created %d/%d work units (%.2f%% success rate)", unitCount, totalUnits, successRate*100)
	assert.GreaterOrEqual(t, successRate, 0.9, "At least 90% of work units should be created successfully")

	// Verify some work units can be retrieved
	sampledUnits := 0
	createdUnits.Range(func(key, _ interface{}) bool {
		if sampledUnits >= 10 {
			return false // Stop after sampling 10 units
		}

		id := key.(string)
		workUnit, err := clientInstance.GetWorkUnit(ctx, id)
		if err != nil {
			t.Logf("Error retrieving work unit %s: %v", id, err)
			return true
		}

		assert.Equal(t, id, workUnit.Id, "Retrieved work unit should have the correct ID")
		assert.Equal(t, contextID, workUnit.ContextId, "Retrieved work unit should have the correct context ID")

		sampledUnits++
		return true
	})
}

// TestConcurrentLockAcquisition tests concurrent lock acquisition with contention
func TestConcurrentLockAcquisition(t *testing.T) {
	t.Skip("Skipping test temporarily while fixing architecture")
	if testing.Short() {
		t.Skip("Skipping concurrent lock acquisition test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-lock-concurrency-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Set up server instance with optimized components
	serverInstance, _, cancel, err := setupTestServer(tempDir, 8)
	require.NoError(t, err, "Failed to set up test server")
	defer cancel()

	// Wait for server to be ready
	time.Sleep(1 * time.Second)

	// Number of resources and clients
	const numResources = 5
	const numClientsPerResource = 10
	const lockDurationSec = 1

	// Create resources and clients
	resources := make([]string, numResources)
	for i := 0; i < numResources; i++ {
		resources[i] = fmt.Sprintf("/concurrent/resource-%d", i)
	}

	clients := make([]*client.Client, numClientsPerResource*numResources)
	for i := 0; i < len(clients); i++ {
		clients[i] = client.New(
			fmt.Sprintf("http://localhost:%d", serverInstance.Port),
			client.WithAgentID(fmt.Sprintf("lock-client-%d", i)),
			client.WithTimeout(5*time.Second),
		)
	}

	// Track successful locks per resource
	successfulLocks := make([]int32, numResources)
	var mu sync.Mutex

	// Run concurrent lock acquisitions
	var eg errgroup.Group
	for i := 0; i < numResources; i++ {
		resourceIndex := i
		resourcePath := resources[i]

		for j := 0; j < numClientsPerResource; j++ {
			clientIndex := i*numClientsPerResource + j
			client := clients[clientIndex]

			eg.Go(func() error {
				ctx := context.Background()
				// Try to acquire lock
				lock, err := client.AcquireLock(ctx, resourcePath, lockDurationSec)
				if err != nil {
					// Expected contention errors
					return nil
				}

				// Successfully acquired lock
				mu.Lock()
				successfulLocks[resourceIndex]++
				mu.Unlock()

				// Hold lock briefly then release
				time.Sleep(100 * time.Millisecond)
				_, err = client.ReleaseLock(ctx, resourcePath, lock.LockId)
				return err
			})
		}
	}

	// Wait for all lock attempts to complete
	err = eg.Wait()
	require.NoError(t, err, "Lock operations should not return unexpected errors")

	// Verify that exactly one client acquired each lock at any given time
	for i := 0; i < numResources; i++ {
		assert.Equal(t, int32(1), successfulLocks[i], 
			"Exactly one client should acquire the lock for resource %s", resources[i])
	}

	// Test sequential acquisition
	t.Run("SequentialAcquisition", func(t *testing.T) {
		testResource := "/concurrent/sequential-resource"
		testClient := client.New(
			fmt.Sprintf("http://localhost:%d", serverInstance.Port),
			client.WithAgentID("sequential-client"),
			client.WithTimeout(5*time.Second),
		)

		ctx := context.Background()

		// First acquisition should succeed
		lock1, err := testClient.AcquireLock(ctx, testResource, 1)
		require.NoError(t, err, "First lock acquisition should succeed")

		// Release the lock
		released, err := testClient.ReleaseLock(ctx, testResource, lock1.LockId)
		require.NoError(t, err, "Lock release should succeed")
		assert.True(t, released, "Lock should be released")

		// Second acquisition should succeed after release
		lock2, err := testClient.AcquireLock(ctx, testResource, 1)
		require.NoError(t, err, "Second lock acquisition should succeed after release")
		assert.NotEqual(t, lock1.LockId, lock2.LockId, "Lock IDs should be different")
	})
}

// TestConcurrentSearch tests concurrent search operations
func TestConcurrentSearch(t *testing.T) {
	t.Skip("Skipping test temporarily while fixing architecture")
	if testing.Short() {
		t.Skip("Skipping concurrent search test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-search-concurrency-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Set up server instance with optimized components
	serverInstance, _, cancel, err := setupTestServer(tempDir, 9)
	require.NoError(t, err, "Failed to set up test server")
	defer cancel()

	// Wait for server to be ready
	time.Sleep(1 * time.Second)

	// Create client
	clientInstance := client.New(
		fmt.Sprintf("http://localhost:%d", serverInstance.Port),
		client.WithAgentID("search-test-agent"),
		client.WithTimeout(10*time.Second),
	)

	// Create a context with searchable content
	ctx := context.Background()
	contextID := fmt.Sprintf("search-context-%s", uuid.New().String()[:8])
	_, err = clientInstance.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "Search Test Context",
	})
	require.NoError(t, err, "Failed to create context")

	// Create work units with various keywords
	keywords := []string{"apple", "banana", "carrot", "date", "eggplant"}
	unitsPerKeyword := 20
	
	// Create work units
	for _, keyword := range keywords {
		for j := 0; j < unitsPerKeyword; j++ {
			req := &proto.CreateWorkUnitRequest{
				ContextId: contextID,
				AgentId:   "search-agent",
				Type:      proto.WorkUnitType_MESSAGE,
				Payload:   []byte(fmt.Sprintf("This is a %s related message number %d", keyword, j)),
				Meta: map[string]string{
					"keyword": keyword,
					"index":   fmt.Sprintf("%d", j),
				},
			}

			_, err := clientInstance.CreateWorkUnit(ctx, req)
			require.NoError(t, err, "Failed to create work unit")
		}
	}

	// Give the search index time to catch up
	time.Sleep(500 * time.Millisecond)

	// Run concurrent searches
	const numSearchers = 10
	const searchesPerClient = 10
	
	var wg sync.WaitGroup
	var searchErrors int32
	var searchMu sync.Mutex
	
	for i := 0; i < numSearchers; i++ {
		wg.Add(1)
		go func(searcherID int) {
			defer wg.Done()
			
			for j := 0; j < searchesPerClient; j++ {
				// Pick a random keyword
				keyword := keywords[j%len(keywords)]
				
				// Perform search
				_, _, err := clientInstance.Search(ctx, keyword, contextID, nil, nil, nil, 10, 0)
				if err != nil {
					t.Logf("Search error: %v", err)
					searchMu.Lock()
					searchErrors++
					searchMu.Unlock()
				}
			}
		}(i)
	}
	
	// Wait for all searches to complete
	wg.Wait()
	
	// Verify search error rate is acceptable (less than 10%)
	errorRate := float64(searchErrors) / float64(numSearchers*searchesPerClient)
	t.Logf("Search errors: %d/%d (%.2f%%)", searchErrors, numSearchers*searchesPerClient, errorRate*100)
	assert.LessOrEqual(t, errorRate, 0.1, "Search error rate should be less than 10%")
}

// setupTestServer creates a test server with all components
func setupTestServer(tempDir string, port int) (*TestServerInstance, context.Context, context.CancelFunc, error) {
	// Create context for server
	ctx, cancel := context.WithCancel(context.Background())

	// Create server components
	routerInstance := router.NewRouter()
	
	// Create a search data directory
	searchDir := tempDir + "/search"
	if err := os.MkdirAll(searchDir, 0755); err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to create search directory: %w", err)
	}
	
	// Create storage
	storageConfig := badger.Config{
		DataDir:           tempDir + "/storage",
		WALSyncInterval:   10 * time.Millisecond,
		WALBatchSize:      64,
		CacheEnabled:      true,
		WorkUnitCacheSize: 10000,
		ContextCacheSize:  1000,
		LockCacheSize:     1000,
		CacheExpiration:   30 * time.Second,
	}
	
	storageInstance, err := badger.NewStorage(storageConfig)
	if err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Start the storage
	go storageInstance.Start(ctx)
	
	// Connect router to event stream
	go routerInstance.Start(ctx, storageInstance.EventStream())

	// Create search
	searchConfig := search.Config{
		DataDir: tempDir,
	}
	searchInstance, err := search.NewSearch(searchConfig)
	if err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to create search: %w", err)
	}
	
	// Start search with router subscription
	searchSub := routerInstance.Subscribe()
	go searchInstance.Start(ctx, searchSub.Events)

	// Create lock manager
	lockMgrConfig := lockmanager.Config{
		DefaultTTL:      60 * time.Second,
		CleanupInterval: time.Minute,
	}
	lockManagerInstance := lockmanager.NewLockManager(lockMgrConfig, storageInstance)
	
	// Start lock manager
	go lockManagerInstance.Start(ctx)

	// Create notifier with router
	notifierConfig := notifier.Config{
		MaxIdleTime:            30 * time.Second,
		BroadcastBufferSize:    200,
		BroadcastFlushInterval: 50 * time.Millisecond,
	}
	notifierInstance := notifier.NewNotifier(notifierConfig, routerInstance)
	
	// Start notifier with router subscription
	notifierSub := routerInstance.Subscribe()
	go notifierInstance.Start(ctx, notifierSub)

	// Create API
	apiConfig := api.Config{
		Addr: fmt.Sprintf(":%d", port),
	}
	apiInstance := api.NewAPI(
		apiConfig,
		storageInstance,
		routerInstance,
		notifierInstance,
		lockManagerInstance,
		searchInstance,
	)
	
	// Start API
	go apiInstance.Start(ctx)

	return &TestServerInstance{
		Port:        port,
		Router:      routerInstance,
		Notifier:    notifierInstance,
		LockManager: lockManagerInstance,
		Search:      searchInstance,
		API:         apiInstance,
	}, ctx, cancel, nil
}

// TestServerInstance holds all components of a test server
type TestServerInstance struct {
	Port        int
	Router      *router.Router
	Notifier    *notifier.Notifier
	LockManager *lockmanager.LockManager
	Search      *search.Search
	API         *api.API
}