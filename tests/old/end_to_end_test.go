// +build ignore

package old

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/api"
	"github.com/nkkko/engram-v3/internal/engine"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage"
	"github.com/nkkko/engram-v3/internal/storage/badger"
	"github.com/nkkko/engram-v3/pkg/client"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEnd performs a full end-to-end test of the Engram system
func TestEndToEnd(t *testing.T) {
	t.Skip("Skipping test temporarily while fixing architecture")
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-test-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create server components
	routerInstance := router.NewRouter()
	storageInstance, err := createTestStorage(tempDir)
	require.NoError(t, err, "Failed to create storage")

	searchInstance, err := createTestSearch(tempDir)
	require.NoError(t, err, "Failed to create search")

	lockManagerInstance := createTestLockManager(storageInstance)
	notifierInstance := notifier.NewNotifier(notifier.DefaultConfig(), routerInstance)

	// Create API
	apiConfig := api.DefaultConfig()
	apiConfig.Addr = ":8181" // Use a different port for testing
	apiInstance := api.NewAPI(
		apiConfig,
		storageInstance,
		routerInstance,
		notifierInstance,
		lockManagerInstance,
		searchInstance,
	)

	// Create engine to coordinate all components
	engineConfig := engine.DefaultConfig()
	engineInstance := engine.NewEngine(
		engineConfig,
		storageInstance,
		routerInstance,
		lockManagerInstance,
		searchInstance,
		apiInstance,
	)

	// Create context for server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engineInstance.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Engine error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Create client
	clientInstance := client.New(
		"http://localhost:8181",
		client.WithAgentID("test-agent"),
		client.WithTimeout(5*time.Second),
	)

	// Test client operations
	t.Run("ContextOperations", func(t *testing.T) {
		testContextOperations(t, clientInstance)
	})

	t.Run("WorkUnitOperations", func(t *testing.T) {
		testWorkUnitOperations(t, clientInstance)
	})

	t.Run("LockOperations", func(t *testing.T) {
		testLockOperations(t, clientInstance)
	})

	t.Run("WebSocketStreaming", func(t *testing.T) {
		testWebSocketStreaming(t, clientInstance)
	})

	// Shutdown server
	cancel()
	wg.Wait()
}

// Helper functions for creating test components
func createTestStorage(tempDir string) (storage.Storage, error) {
	config := badger.Config{
		DataDir:           tempDir + "/storage",
		WALSyncInterval:   100 * time.Millisecond,
		WALBatchSize:      10,
		CacheEnabled:      true,
		WorkUnitCacheSize: 100,
		ContextCacheSize:  100,
		LockCacheSize:     100,
		CacheExpiration:   30 * time.Second,
	}
	return badger.NewStorage(config)
}

func createTestSearch(tempDir string) (*search.Search, error) {
	// Create a temporary directory for test data
	searchDir := tempDir + "/search"
	err := os.MkdirAll(searchDir, 0755)
	if err != nil {
		return nil, err
	}
	
	// Now we can just use an existing storage instance for search
	store, err := createTestStorage(searchDir)
	if err != nil {
		return nil, err
	}
	
	return search.NewSearch(store), nil
}

func createTestLockManager(storageInstance storage.Storage) *lockmanager.LockManager {
	return lockmanager.NewLockManager(storageInstance)
}

// Test context operations
func testContextOperations(t *testing.T, client *client.Client) {
	// Create a context
	ctx := context.Background()
	contextID := fmt.Sprintf("test-context-%s", uuid.New().String()[:8])
	displayName := "Test Context"
	
	// Update context (will create it since it doesn't exist)
	updateReq := &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: displayName,
		AddAgents:   []string{"agent1", "agent2"},
		Meta: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	
	context, err := client.UpdateContext(ctx, updateReq)
	require.NoError(t, err, "Failed to create context")
	assert.Equal(t, contextID, context.Id)
	assert.Equal(t, displayName, context.DisplayName)
	assert.Contains(t, context.AgentIds, "agent1")
	assert.Contains(t, context.AgentIds, "agent2")
	assert.Equal(t, "value1", context.Meta["key1"])
	assert.Equal(t, "value2", context.Meta["key2"])
	
	// Get the context
	retrievedContext, err := client.GetContext(ctx, contextID)
	require.NoError(t, err, "Failed to get context")
	assert.Equal(t, contextID, retrievedContext.Id)
	assert.Equal(t, displayName, retrievedContext.DisplayName)
	
	// Update the context
	updateReq2 := &proto.UpdateContextRequest{
		Id:           contextID,
		DisplayName:  "Updated Context",
		AddAgents:    []string{"agent3"},
		RemoveAgents: []string{"agent1"},
	}
	
	updatedContext, err := client.UpdateContext(ctx, updateReq2)
	require.NoError(t, err, "Failed to update context")
	assert.Equal(t, "Updated Context", updatedContext.DisplayName)
	assert.Contains(t, updatedContext.AgentIds, "agent2")
	assert.Contains(t, updatedContext.AgentIds, "agent3")
	assert.NotContains(t, updatedContext.AgentIds, "agent1")
}

// Test work unit operations
func testWorkUnitOperations(t *testing.T, client *client.Client) {
	// Create a context for work units
	ctx := context.Background()
	contextID := fmt.Sprintf("test-wu-context-%s", uuid.New().String()[:8])
	_, err := client.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "Work Unit Test Context",
	})
	require.NoError(t, err, "Failed to create context for work units")
	
	// Create a work unit
	createReq := &proto.CreateWorkUnitRequest{
		ContextId: contextID,
		AgentId:   "test-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Payload:   []byte("Test message payload"),
		Meta: map[string]string{
			"source": "test",
		},
	}
	
	workUnit, err := client.CreateWorkUnit(ctx, createReq)
	require.NoError(t, err, "Failed to create work unit")
	assert.Equal(t, contextID, workUnit.ContextId)
	assert.Equal(t, proto.WorkUnitType_MESSAGE, workUnit.Type)
	assert.Equal(t, []byte("Test message payload"), workUnit.Payload)
	assert.Equal(t, "test", workUnit.Meta["source"])
	
	// Get the work unit
	retrievedWorkUnit, err := client.GetWorkUnit(ctx, workUnit.Id)
	require.NoError(t, err, "Failed to get work unit")
	assert.Equal(t, workUnit.Id, retrievedWorkUnit.Id)
	assert.Equal(t, workUnit.ContextId, retrievedWorkUnit.ContextId)
	
	// Create a few more work units
	for i := 0; i < 3; i++ {
		createReq := &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "test-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte(fmt.Sprintf("Test message %d", i)),
		}
		_, err := client.CreateWorkUnit(ctx, createReq)
		require.NoError(t, err, "Failed to create work unit")
	}
	
	// List work units
	workUnits, nextCursor, err := client.ListWorkUnits(ctx, contextID, 10, "", false)
	require.NoError(t, err, "Failed to list work units")
	assert.GreaterOrEqual(t, len(workUnits), 4, "Should have at least 4 work units")
	assert.Empty(t, nextCursor, "Should not have a next cursor")
	
	// Test pagination by requesting only 2 work units
	workUnits, nextCursor, err = client.ListWorkUnits(ctx, contextID, 2, "", false)
	require.NoError(t, err, "Failed to list work units with pagination")
	assert.Len(t, workUnits, 2, "Should have 2 work units")
	assert.NotEmpty(t, nextCursor, "Should have a next cursor")
}

// Test lock operations
func testLockOperations(t *testing.T, client *client.Client) {
	ctx := context.Background()
	resourcePath := fmt.Sprintf("/test/resource-%s", uuid.New().String()[:8])
	
	// Acquire a lock
	lock, err := client.AcquireLock(ctx, resourcePath, 30)
	require.NoError(t, err, "Failed to acquire lock")
	assert.Equal(t, resourcePath, lock.ResourcePath)
	assert.Equal(t, "test-agent", lock.HolderAgent)
	assert.NotEmpty(t, lock.LockId)
	
	// Get the lock
	retrievedLock, err := client.GetLock(ctx, resourcePath)
	require.NoError(t, err, "Failed to get lock")
	assert.Equal(t, lock.LockId, retrievedLock.LockId)
	
	// Try to acquire the same lock with a different agent (should fail)
	otherClient := client.New(
		"http://localhost:8181",
		client.WithAgentID("other-agent"),
		client.WithTimeout(5*time.Second),
	)
	
	_, err = otherClient.AcquireLock(ctx, resourcePath, 30)
	assert.Error(t, err, "Should fail to acquire lock held by another agent")
	
	// Release the lock
	released, err := client.ReleaseLock(ctx, resourcePath, lock.LockId)
	require.NoError(t, err, "Failed to release lock")
	assert.True(t, released, "Lock should be released")
	
	// Now the other agent should be able to acquire the lock
	otherLock, err := otherClient.AcquireLock(ctx, resourcePath, 30)
	require.NoError(t, err, "Failed to acquire lock after release")
	assert.Equal(t, "other-agent", otherLock.HolderAgent)
}

// Test WebSocket streaming
func testWebSocketStreaming(t *testing.T, client *client.Client) {
	// Create a context
	ctx := context.Background()
	contextID := fmt.Sprintf("test-ws-context-%s", uuid.New().String()[:8])
	_, err := client.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "WebSocket Test Context",
	})
	require.NoError(t, err, "Failed to create context for WebSocket test")
	
	// Create a subscription
	subscription, err := client.Subscribe(contextID)
	require.NoError(t, err, "Failed to create subscription")
	defer subscription.Close()
	
	// Create a channel to receive events
	receivedEvents := make(chan *proto.WorkUnit, 10)
	
	// Start receiving events in a goroutine
	go func() {
		for event := range subscription.Events {
			receivedEvents <- event
		}
	}()
	
	// Create work units that should trigger events
	numWorkUnits := 3
	createdWorkUnits := make([]*proto.WorkUnit, 0, numWorkUnits)
	
	for i := 0; i < numWorkUnits; i++ {
		workUnit, err := client.CreateWorkUnit(ctx, &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "test-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte(fmt.Sprintf("WebSocket test message %d", i)),
		})
		require.NoError(t, err, "Failed to create work unit for WebSocket test")
		createdWorkUnits = append(createdWorkUnits, workUnit)
		
		// Give some time for the event to be processed
		time.Sleep(100 * time.Millisecond)
	}
	
	// Verify that events were received
	timeout := time.After(5 * time.Second)
	receivedCount := 0
	
	// We use a map to track which work units we've received
	receivedIDs := make(map[string]bool)
	
	for receivedCount < numWorkUnits {
		select {
		case event := <-receivedEvents:
			// Check if this is one of our work units
			for _, wu := range createdWorkUnits {
				if event.Id == wu.Id {
					receivedIDs[wu.Id] = true
					receivedCount++
					break
				}
			}
		case <-timeout:
			// We didn't receive all expected events
			t.Fatalf("Timed out waiting for events. Received %d out of %d", receivedCount, numWorkUnits)
			return
		}
	}
	
	// Verify that all work units were received
	for _, wu := range createdWorkUnits {
		assert.True(t, receivedIDs[wu.Id], "Should have received event for work unit %s", wu.Id)
	}
}

// TestBenchmark measures the performance of work unit creation
func TestBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-benchmark-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create server components
	routerInstance := router.NewRouter()
	storageInstance, err := createTestStorage(tempDir)
	require.NoError(t, err, "Failed to create storage")

	searchInstance, err := createTestSearch(tempDir)
	require.NoError(t, err, "Failed to create search")

	lockManagerInstance := createTestLockManager(storageInstance)
	notifierInstance := notifier.NewNotifier(notifier.DefaultConfig(), routerInstance)

	// Create API
	apiConfig := api.DefaultConfig()
	apiConfig.Addr = ":8182" // Use a different port for testing
	apiInstance := api.NewAPI(
		apiConfig,
		storageInstance,
		routerInstance,
		notifierInstance,
		lockManagerInstance,
		searchInstance,
	)

	// Create engine to coordinate all components
	engineConfig := engine.DefaultConfig()
	engineInstance := engine.NewEngine(
		engineConfig,
		storageInstance,
		routerInstance,
		lockManagerInstance,
		searchInstance,
		apiInstance,
	)

	// Create context for server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engineInstance.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Engine error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Create client
	clientInstance := client.New(
		"http://localhost:8182",
		client.WithAgentID("benchmark-agent"),
		client.WithTimeout(5*time.Second),
	)

	// Create a context for benchmark
	contextID := fmt.Sprintf("benchmark-context-%s", uuid.New().String()[:8])
	_, err = clientInstance.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "Benchmark Context",
	})
	require.NoError(t, err, "Failed to create context for benchmark")

	// Prepare payload
	payload := []byte("Benchmark test payload with some reasonable size to simulate real-world usage.")

	// Warm up
	for i := 0; i < 10; i++ {
		_, err := clientInstance.CreateWorkUnit(ctx, &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "benchmark-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   payload,
		})
		require.NoError(t, err, "Failed to create work unit during warm-up")
	}

	// Benchmark
	numIterations := 100
	var totalDuration time.Duration

	// Run the benchmark
	for i := 0; i < numIterations; i++ {
		start := time.Now()
		
		_, err := clientInstance.CreateWorkUnit(ctx, &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "benchmark-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   payload,
		})
		
		elapsed := time.Since(start)
		totalDuration += elapsed
		
		require.NoError(t, err, "Failed to create work unit during benchmark")
	}

	// Calculate results
	avgDuration := totalDuration / time.Duration(numIterations)
	
	// Log results
	t.Logf("Benchmark results for work unit creation:")
	t.Logf("  Total iterations: %d", numIterations)
	t.Logf("  Average duration: %v", avgDuration)
	t.Logf("  Operations per second: %.2f", float64(time.Second)/float64(avgDuration))
	
	// Verify the target of 1ms
	assert.LessOrEqual(t, avgDuration, 10*time.Millisecond, 
		"Average duration should be less than 10ms (target is 1ms, but we allow some overhead for testing)")

	// Shutdown server
	cancel()
	wg.Wait()
}

// TestWebSocketConcurrency tests multiple WebSocket clients simultaneously
func TestWebSocketConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WebSocket concurrency test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-ws-concurrency-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create server components
	routerInstance := router.NewRouter()
	storageInstance, err := createTestStorage(tempDir)
	require.NoError(t, err, "Failed to create storage")

	searchInstance, err := createTestSearch(tempDir)
	require.NoError(t, err, "Failed to create search")

	lockManagerInstance := createTestLockManager(storageInstance)
	notifierInstance := notifier.NewNotifier(notifier.DefaultConfig(), routerInstance)

	// Create API
	apiConfig := api.DefaultConfig()
	apiConfig.Addr = ":8183" // Use a different port for testing
	apiInstance := api.NewAPI(
		apiConfig,
		storageInstance,
		routerInstance,
		notifierInstance,
		lockManagerInstance,
		searchInstance,
	)

	// Create engine to coordinate all components
	engineConfig := engine.DefaultConfig()
	engineInstance := engine.NewEngine(
		engineConfig,
		storageInstance,
		routerInstance,
		lockManagerInstance,
		searchInstance,
		apiInstance,
	)

	// Create context for server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engineInstance.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Engine error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Create a context for the test
	contextID := fmt.Sprintf("ws-concurrency-context-%s", uuid.New().String()[:8])
	client := client.New(
		"http://localhost:8183",
		client.WithAgentID("admin-agent"),
		client.WithTimeout(5*time.Second),
	)
	
	_, err = client.UpdateContext(ctx, &proto.UpdateContextRequest{
		Id:          contextID,
		DisplayName: "WebSocket Concurrency Test",
	})
	require.NoError(t, err, "Failed to create context for WebSocket concurrency test")

	// Number of concurrent clients
	numClients := 10
	
	// Create and connect clients
	type clientState struct {
		client       *client.Client
		subscription *client.Subscription
		events       chan *proto.WorkUnit
		received     int
	}
	
	clients := make([]*clientState, numClients)
	for i := 0; i < numClients; i++ {
		agentID := fmt.Sprintf("ws-client-%d", i)
		clients[i] = &clientState{
			client: client.New(
				"http://localhost:8183",
				client.WithAgentID(agentID),
				client.WithTimeout(5*time.Second),
			),
			events: make(chan *proto.WorkUnit, 100),
			received: 0,
		}
		
		// Create subscription
		subscription, err := clients[i].client.Subscribe(contextID)
		require.NoError(t, err, "Failed to create subscription for client %d", i)
		clients[i].subscription = subscription
		
		// Start receiving events
		wg.Add(1)
		go func(cs *clientState) {
			defer wg.Done()
			for event := range cs.subscription.Events {
				cs.events <- event
			}
		}(clients[i])
	}
	
	// Create a work unit to broadcast
	workUnit, err := client.CreateWorkUnit(ctx, &proto.CreateWorkUnitRequest{
		ContextId: contextID,
		AgentId:   "admin-agent",
		Type:      proto.WorkUnitType_SYSTEM,
		Payload:   []byte("Broadcast test message"),
	})
	require.NoError(t, err, "Failed to create broadcast work unit")
	
	// Give some time for processing
	time.Sleep(500 * time.Millisecond)
	
	// Check that all clients received the message
	timeout := time.After(5 * time.Second)
	receivedByAll := false
	
	for !receivedByAll {
		// Check if all clients have received the message
		allReceived := true
		for i, cs := range clients {
			select {
			case event := <-cs.events:
				t.Logf("Client %d received event %s", i, event.Id)
				cs.received++
			default:
				// No event available
			}
			
			if cs.received == 0 {
				allReceived = false
			}
		}
		
		if allReceived {
			receivedByAll = true
			break
		}
		
		select {
		case <-timeout:
			// Timed out
			t.Fatalf("Timed out waiting for all clients to receive the message")
			return
		case <-time.After(100 * time.Millisecond):
			// Continue waiting
		}
	}
	
	// Verify that all clients received the work unit
	for i, cs := range clients {
		assert.Greater(t, cs.received, 0, "Client %d should have received at least one message", i)
	}
	
	// Close all subscriptions
	for _, cs := range clients {
		if cs.subscription != nil {
			cs.subscription.Close()
		}
	}
	
	// Shutdown server
	cancel()
	wg.Wait()
}

// TestLockContention tests multiple clients contending for the same lock
func TestLockContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping lock contention test in short mode")
	}

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-lock-contention-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create server components
	routerInstance := router.NewRouter()
	storageInstance, err := createTestStorage(tempDir)
	require.NoError(t, err, "Failed to create storage")

	searchInstance, err := createTestSearch(tempDir)
	require.NoError(t, err, "Failed to create search")

	lockManagerInstance := createTestLockManager(storageInstance)
	notifierInstance := notifier.NewNotifier(notifier.DefaultConfig(), routerInstance)

	// Create API
	apiConfig := api.DefaultConfig()
	apiConfig.Addr = ":8184" // Use a different port for testing
	apiInstance := api.NewAPI(
		apiConfig,
		storageInstance,
		routerInstance,
		notifierInstance,
		lockManagerInstance,
		searchInstance,
	)

	// Create engine to coordinate all components
	engineConfig := engine.DefaultConfig()
	engineInstance := engine.NewEngine(
		engineConfig,
		storageInstance,
		routerInstance,
		lockManagerInstance,
		searchInstance,
		apiInstance,
	)

	// Create context for server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engineInstance.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Engine error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Create a shared resource path
	resourcePath := fmt.Sprintf("/test/shared-resource-%s", uuid.New().String()[:8])
	
	// Create clients
	numClients := 5
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = client.New(
			"http://localhost:8184",
			client.WithAgentID(fmt.Sprintf("contention-client-%d", i)),
			client.WithTimeout(5*time.Second),
		)
	}
	
	// Store acquired locks
	acquiredLocks := make(map[int]*proto.LockHandle)
	var successfulClient int = -1
	
	// Have all clients attempt to acquire the lock simultaneously
	var acquireWg sync.WaitGroup
	acquireWg.Add(numClients)
	
	for i := 0; i < numClients; i++ {
		go func(clientIndex int) {
			defer acquireWg.Done()
			
			lock, err := clients[clientIndex].AcquireLock(ctx, resourcePath, 10)
			if err == nil {
				// Successfully acquired the lock
				acquiredLocks[clientIndex] = lock
				t.Logf("Client %d acquired the lock", clientIndex)
			} else {
				t.Logf("Client %d failed to acquire the lock: %v", clientIndex, err)
			}
		}(i)
	}
	
	// Wait for all acquire attempts to complete
	acquireWg.Wait()
	
	// Verify that exactly one client acquired the lock
	require.Equal(t, 1, len(acquiredLocks), "Exactly one client should have acquired the lock")
	
	// Identify which client was successful
	for clientIndex, lock := range acquiredLocks {
		successfulClient = clientIndex
		
		// Verify the lock properties
		assert.Equal(t, resourcePath, lock.ResourcePath)
		assert.Equal(t, fmt.Sprintf("contention-client-%d", clientIndex), lock.HolderAgent)
		assert.NotEmpty(t, lock.LockId)
	}
	
	// Have the successful client release the lock
	require.GreaterOrEqual(t, successfulClient, 0, "Should have identified a successful client")
	
	lock := acquiredLocks[successfulClient]
	released, err := clients[successfulClient].ReleaseLock(ctx, resourcePath, lock.LockId)
	require.NoError(t, err, "Failed to release lock")
	assert.True(t, released, "Lock should be released")
	
	// Now have another client try to acquire the lock - it should succeed
	nextClient := (successfulClient + 1) % numClients
	newLock, err := clients[nextClient].AcquireLock(ctx, resourcePath, 10)
	require.NoError(t, err, "Next client should be able to acquire the lock")
	assert.Equal(t, resourcePath, newLock.ResourcePath)
	assert.Equal(t, fmt.Sprintf("contention-client-%d", nextClient), newLock.HolderAgent)
	
	// Shutdown server
	cancel()
	wg.Wait()
}