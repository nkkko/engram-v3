package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/engine"
	"github.com/nkkko/engram-v3/pkg/client"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngineEndToEnd tests the full engine functionality
func TestEngineEndToEnd(t *testing.T) {
	// Skip in short test mode
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}
	
	// Uncomment this line when ready to run the test
	t.Skip("Skipping until we're ready to run full E2E tests")

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "engram-e2e-*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Get absolute path
	absDataDir, err := filepath.Abs(tempDir)
	require.NoError(t, err, "Failed to get absolute path for data directory")

	// Use a dynamic port
	port := 8090

	// Create engine configuration
	config := engine.Config{
		DataDir:                 absDataDir,
		ServerAddr:              fmt.Sprintf(":%d", port),
		LogLevel:                "debug",
		WALSyncIntervalMs:       2,
		WALBatchSize:            32,
		DefaultLockTTLSeconds:   30,
		MaxIdleConnectionSeconds: 10,
	}

	// Create engine with all components
	eng, err := engine.CreateEngine(config)
	require.NoError(t, err, "Failed to create engine")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start engine in a separate goroutine
	go func() {
		if err := eng.Start(ctx); err != nil {
			t.Logf("Engine error: %v", err)
		}
	}()

	// Wait for engine to start
	time.Sleep(2 * time.Second)

	t.Run("ClientCanConnect", func(t *testing.T) {
		// Create client
		c := client.New(
			fmt.Sprintf("http://localhost:%d", port),
			client.WithAgentID("e2e-test-agent"),
			client.WithTimeout(5*time.Second),
		)

		// Create a context
		contextID := fmt.Sprintf("test-context-%s", uuid.New().String()[:8])
		context, err := c.UpdateContext(ctx, &proto.UpdateContextRequest{
			Id:          contextID,
			DisplayName: "E2E Test Context",
			AddAgents:   []string{"e2e-test-agent"},
			Meta: map[string]string{
				"test": "true",
				"type": "e2e",
			},
		})
		require.NoError(t, err, "Failed to create context")
		assert.Equal(t, contextID, context.Id, "Context ID should match")
		assert.Equal(t, "E2E Test Context", context.DisplayName, "Context display name should match")
		assert.Contains(t, context.AgentIds, "e2e-test-agent", "Context should include the agent")
		assert.Equal(t, "true", context.Meta["test"], "Context metadata should be set")

		// Create a work unit
		workUnitReq := &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "e2e-test-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte("Test message from E2E test"),
			Meta: map[string]string{
				"test": "true",
				"type": "message",
			},
		}
		workUnit, err := c.CreateWorkUnit(ctx, workUnitReq)
		require.NoError(t, err, "Failed to create work unit")
		assert.NotEmpty(t, workUnit.Id, "Work unit ID should not be empty")
		assert.Equal(t, contextID, workUnit.ContextId, "Work unit context ID should match")
		assert.Equal(t, "e2e-test-agent", workUnit.AgentId, "Work unit agent ID should match")
		assert.Equal(t, proto.WorkUnitType_MESSAGE, workUnit.Type, "Work unit type should match")

		// Get the work unit
		retrievedUnit, err := c.GetWorkUnit(ctx, workUnit.Id)
		require.NoError(t, err, "Failed to get work unit")
		assert.Equal(t, workUnit.Id, retrievedUnit.Id, "Retrieved work unit ID should match")
		assert.Equal(t, workUnit.ContextId, retrievedUnit.ContextId, "Retrieved work unit context ID should match")
		assert.Equal(t, workUnit.AgentId, retrievedUnit.AgentId, "Retrieved work unit agent ID should match")
		assert.Equal(t, workUnit.Type, retrievedUnit.Type, "Retrieved work unit type should match")
		assert.Equal(t, "Test message from E2E test", string(retrievedUnit.Payload), "Retrieved work unit payload should match")

		// List work units
		units, nextCursor, err := c.ListWorkUnits(ctx, contextID, 10, "", false)
		require.NoError(t, err, "Failed to list work units")
		assert.NotEmpty(t, units, "Work units list should not be empty")
		assert.Empty(t, nextCursor, "Next cursor should be empty for small result set")
		assert.Equal(t, workUnit.Id, units[0].Id, "First work unit ID should match")

		// Update context
		updatedContext, err := c.UpdateContext(ctx, &proto.UpdateContextRequest{
			Id:          contextID,
			DisplayName: "Updated E2E Test Context",
			PinUnits:    []string{workUnit.Id},
		})
		require.NoError(t, err, "Failed to update context")
		assert.Equal(t, "Updated E2E Test Context", updatedContext.DisplayName, "Updated context display name should match")
		assert.Contains(t, updatedContext.PinnedUnits, workUnit.Id, "Updated context should have pinned work unit")

		// Get context
		retrievedContext, err := c.GetContext(ctx, contextID)
		require.NoError(t, err, "Failed to get context")
		assert.Equal(t, updatedContext.DisplayName, retrievedContext.DisplayName, "Retrieved context display name should match")
		assert.Contains(t, retrievedContext.PinnedUnits, workUnit.Id, "Retrieved context should have pinned work unit")
	})

	t.Run("LockManagement", func(t *testing.T) {
		// Create client
		c := client.New(
			fmt.Sprintf("http://localhost:%d", port),
			client.WithAgentID("e2e-lock-agent"),
			client.WithTimeout(5*time.Second),
		)

		// Acquire a lock
		resourcePath := "/e2e/test/resource"
		lock, err := c.AcquireLock(ctx, resourcePath, 10)
		require.NoError(t, err, "Failed to acquire lock")
		assert.Equal(t, resourcePath, lock.ResourcePath, "Lock resource path should match")
		assert.Equal(t, "e2e-lock-agent", lock.HolderAgent, "Lock holder agent should match")
		assert.NotEmpty(t, lock.LockId, "Lock ID should not be empty")

		// Get the lock
		retrievedLock, err := c.GetLock(ctx, resourcePath)
		require.NoError(t, err, "Failed to get lock")
		assert.Equal(t, lock.ResourcePath, retrievedLock.ResourcePath, "Retrieved lock resource path should match")
		assert.Equal(t, lock.HolderAgent, retrievedLock.HolderAgent, "Retrieved lock holder agent should match")
		assert.Equal(t, lock.LockId, retrievedLock.LockId, "Retrieved lock ID should match")

		// Try to acquire the same lock with a different agent (should fail)
		c2 := client.New(
			fmt.Sprintf("http://localhost:%d", port),
			client.WithAgentID("e2e-lock-agent-2"),
			client.WithTimeout(5*time.Second),
		)
		_, err = c2.AcquireLock(ctx, resourcePath, 10)
		assert.Error(t, err, "Acquiring locked resource should fail")

		// Release the lock
		released, err := c.ReleaseLock(ctx, resourcePath, lock.LockId)
		require.NoError(t, err, "Failed to release lock")
		assert.True(t, released, "Lock should be released")

		// Now the second agent should be able to acquire the lock
		lock2, err := c2.AcquireLock(ctx, resourcePath, 10)
		require.NoError(t, err, "Failed to acquire lock after release")
		assert.Equal(t, resourcePath, lock2.ResourcePath, "Second lock resource path should match")
		assert.Equal(t, "e2e-lock-agent-2", lock2.HolderAgent, "Second lock holder agent should match")
	})

	t.Run("Search", func(t *testing.T) {
		// Create client
		c := client.New(
			fmt.Sprintf("http://localhost:%d", port),
			client.WithAgentID("e2e-search-agent"),
			client.WithTimeout(5*time.Second),
		)

		// Create a context for search
		contextID := fmt.Sprintf("search-context-%s", uuid.New().String()[:8])
		_, err := c.UpdateContext(ctx, &proto.UpdateContextRequest{
			Id:          contextID,
			DisplayName: "E2E Search Context",
		})
		require.NoError(t, err, "Failed to create search context")

		// Create work units with searchable content
		terms := []string{"apple", "banana", "cherry"}
		for _, term := range terms {
			for j := 0; j < 3; j++ {
				req := &proto.CreateWorkUnitRequest{
					ContextId: contextID,
					AgentId:   "e2e-search-agent",
					Type:      proto.WorkUnitType_MESSAGE,
					Payload:   []byte(fmt.Sprintf("This is a test message about %s number %d", term, j)),
					Meta: map[string]string{
						"fruit":  term,
						"index":  fmt.Sprintf("%d", j),
						"search": "true",
					},
				}
				_, err := c.CreateWorkUnit(ctx, req)
				require.NoError(t, err, "Failed to create work unit for search")
			}
		}

		// Give the search index time to update
		time.Sleep(1 * time.Second)

		// Search for a specific term
		results, total, err := c.Search(ctx, "apple", contextID, nil, nil, nil, 10, 0)
		require.NoError(t, err, "Failed to search")
		assert.NotEmpty(t, results, "Search results should not be empty")
		assert.Greater(t, total, 0, "Total search results should be greater than 0")

		// Search with metadata filter
		metaFilter := map[string]string{"fruit": "banana"}
		results, total, err = c.Search(ctx, "", contextID, nil, nil, metaFilter, 10, 0)
		require.NoError(t, err, "Failed to search with metadata filter")
		assert.NotEmpty(t, results, "Search results with metadata filter should not be empty")
		assert.Equal(t, 3, total, "Should find 3 work units with banana metadata")

		// Search with agent filter
		agentFilter := []string{"e2e-search-agent"}
		results, total, err = c.Search(ctx, "cherry", contextID, agentFilter, nil, nil, 10, 0)
		require.NoError(t, err, "Failed to search with agent filter")
		assert.NotEmpty(t, results, "Search results with agent filter should not be empty")
	})

	// Graceful shutdown
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = eng.Shutdown(shutdownCtx)
	require.NoError(t, err, "Failed to shutdown engine")
}