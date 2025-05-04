package lockmanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnhancedLockManager_New tests the creation of a new enhanced lock manager
func TestEnhancedLockManager_New(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	
	manager := NewEnhancedLockManager(config, mockStorage)
	
	assert.NotNil(t, manager, "Enhanced lock manager should not be nil")
	assert.Equal(t, config, manager.config, "Config should be set correctly")
	assert.Equal(t, mockStorage, manager.storage, "Storage should be set correctly")
	assert.NotNil(t, manager.locks, "Locks map should be initialized")
	assert.NotNil(t, manager.agentLocks, "Agent locks map should be initialized")
	assert.NotNil(t, manager.waitingRequests, "Waiting requests map should be initialized")
	assert.NotNil(t, manager.logger, "Logger should be initialized")
	assert.NotNil(t, manager.metrics, "Metrics should be initialized")
}

// TestEnhancedLockManager_AcquireLock tests acquiring a lock
func TestEnhancedLockManager_AcquireLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	lock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	
	// Assertions
	require.NoError(t, err, "Lock acquisition should succeed")
	assert.NotNil(t, lock, "Lock should not be nil")
	assert.Equal(t, "/test/resource", lock.ResourcePath, "Resource path should match")
	assert.Equal(t, "agent-1", lock.HolderAgent, "Agent ID should match")
	assert.NotEmpty(t, lock.LockId, "Lock ID should be set")
	
	// Verify the lock exists in the manager
	manager.mu.RLock()
	storedLock, exists := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.True(t, exists, "Lock should exist in the manager")
	assert.Equal(t, lock.LockId, storedLock.LockId, "Lock IDs should match")
	
	// Verify agent locks are updated
	manager.mu.RLock()
	agentLocks, exists := manager.agentLocks["agent-1"]
	manager.mu.RUnlock()
	
	assert.True(t, exists, "Agent should have locks")
	assert.Contains(t, agentLocks, "/test/resource", "Agent locks should contain the resource")
	
	// Verify storage was called
	time.Sleep(50 * time.Millisecond) // Wait for async storage call
	assert.Equal(t, 1, mockStorage.AcquireCallCount(), "Storage AcquireLock should be called")
}

// TestEnhancedLockManager_AcquireLock_AlreadyHeld tests behavior when a lock is already held
func TestEnhancedLockManager_AcquireLock_AlreadyHeld(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	lock1, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "First lock acquisition should succeed")
	
	// Try to acquire the same lock with a different agent
	lock2, err := manager.AcquireLock(ctx, "/test/resource", "agent-2", time.Minute)
	assert.Error(t, err, "Second lock acquisition with different agent should fail")
	assert.Nil(t, lock2, "Lock should be nil when acquisition fails")
	
	// Verify error type
	var lockErr *LockAcquisitionError
	assert.ErrorAs(t, err, &lockErr, "Error should be a LockAcquisitionError")
	assert.Equal(t, "/test/resource", lockErr.ResourcePath, "Error should have correct resource path")
	assert.Equal(t, "agent-1", lockErr.HolderAgent, "Error should have correct holder agent")
	assert.Equal(t, "resource already locked", lockErr.Reason, "Error should have correct reason")
	
	// Try to acquire the same lock with the same agent (should extend)
	originalExpiry := lock1.ExpiresAt.AsTime()
	time.Sleep(10 * time.Millisecond) // Ensure time passes
	
	lock3, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition by same agent should succeed (extend)")
	assert.Equal(t, lock1.LockId, lock3.LockId, "Lock ID should be the same (extended)")
	assert.True(t, lock3.ExpiresAt.AsTime().After(originalExpiry), "Lock expiry should be extended")
}

// TestEnhancedLockManager_AgentLockLimit tests max locks per agent limit
func TestEnhancedLockManager_AgentLockLimit(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	config.MaxLocksPerAgent = 3
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire max number of locks
	ctx := context.Background()
	for i := 0; i < config.MaxLocksPerAgent; i++ {
		resourcePath := fmt.Sprintf("/test/resource-%d", i)
		_, err := manager.AcquireLock(ctx, resourcePath, "agent-1", time.Minute)
		require.NoError(t, err, "Lock acquisition should succeed")
	}
	
	// Try to acquire one more lock
	_, err := manager.AcquireLock(ctx, "/test/resource-extra", "agent-1", time.Minute)
	assert.Error(t, err, "Should fail when agent tries to acquire more than the limit")
	assert.Contains(t, err.Error(), "maximum number of locks", "Error should mention the max locks limit")
	
	// Verify agent locks count
	manager.mu.RLock()
	agentLocks := manager.agentLocks["agent-1"]
	manager.mu.RUnlock()
	
	assert.Equal(t, config.MaxLocksPerAgent, len(agentLocks), "Agent should have exactly max locks")
}

// TestEnhancedLockManager_ReleaseLock tests releasing a lock
func TestEnhancedLockManager_ReleaseLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	lock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	// Release the lock
	released, err := manager.ReleaseLock(ctx, "/test/resource", "agent-1", lock.LockId)
	require.NoError(t, err, "Lock release should succeed")
	assert.True(t, released, "Lock should be marked as released")
	
	// Verify the lock no longer exists in the manager
	manager.mu.RLock()
	_, exists := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.False(t, exists, "Lock should not exist in the manager after release")
	
	// Verify agent locks are updated
	manager.mu.RLock()
	agentLocks, exists := manager.agentLocks["agent-1"]
	manager.mu.RUnlock()
	
	if exists {
		assert.NotContains(t, agentLocks, "/test/resource", "Agent locks should not contain the resource")
	} else {
		// If agent has no more locks, the map entry should be removed
		assert.True(t, true, "Agent locks map entry should be removed if empty")
	}
	
	// Verify storage was called
	time.Sleep(50 * time.Millisecond) // Wait for async storage call
	assert.Equal(t, 1, mockStorage.ReleaseCallCount(), "Storage ReleaseLock should be called")
}

// TestEnhancedLockManager_GetLock tests getting lock information
func TestEnhancedLockManager_GetLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	originalLock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	// Get the lock
	lock, err := manager.GetLock(ctx, "/test/resource")
	require.NoError(t, err, "GetLock should succeed")
	assert.Equal(t, originalLock.LockId, lock.LockId, "Lock IDs should match")
	assert.Equal(t, "/test/resource", lock.ResourcePath, "Resource paths should match")
	assert.Equal(t, "agent-1", lock.HolderAgent, "Agent IDs should match")
}

// TestEnhancedLockManager_ListLocks tests listing all locks
func TestEnhancedLockManager_ListLocks(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire multiple locks
	ctx := context.Background()
	resources := []string{"/test/resource1", "/test/resource2", "/test/resource3"}
	
	for _, resource := range resources {
		_, err := manager.AcquireLock(ctx, resource, "agent-1", time.Minute)
		require.NoError(t, err, "Lock acquisition should succeed")
	}
	
	// List all locks
	locks, err := manager.ListLocks(ctx)
	require.NoError(t, err, "ListLocks should succeed")
	assert.Len(t, locks, 3, "Should list 3 locks")
	
	// Verify all resources are in the list
	resourceSet := make(map[string]bool)
	for _, lock := range locks {
		resourceSet[lock.ResourcePath] = true
	}
	
	for _, resource := range resources {
		assert.True(t, resourceSet[resource], "Resource %s should be in the list", resource)
	}
}

// TestEnhancedLockManager_GetAgentLocks tests getting locks for a specific agent
func TestEnhancedLockManager_GetAgentLocks(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire locks for two different agents
	ctx := context.Background()
	
	// Agent 1 locks
	_, err := manager.AcquireLock(ctx, "/test/resource1", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition for agent-1 should succeed")
	_, err = manager.AcquireLock(ctx, "/test/resource2", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition for agent-1 should succeed")
	
	// Agent 2 locks
	_, err = manager.AcquireLock(ctx, "/test/resource3", "agent-2", time.Minute)
	require.NoError(t, err, "Lock acquisition for agent-2 should succeed")
	
	// Get locks for agent-1
	agent1Locks, err := manager.GetAgentLocks(ctx, "agent-1")
	require.NoError(t, err, "GetAgentLocks should succeed")
	assert.Len(t, agent1Locks, 2, "Agent-1 should have 2 locks")
	
	// Verify agent-1 locks
	resourceSet := make(map[string]bool)
	for _, lock := range agent1Locks {
		resourceSet[lock.ResourcePath] = true
		assert.Equal(t, "agent-1", lock.HolderAgent, "All locks should be held by agent-1")
	}
	
	assert.True(t, resourceSet["/test/resource1"], "Agent-1 should have lock on resource1")
	assert.True(t, resourceSet["/test/resource2"], "Agent-1 should have lock on resource2")
	
	// Get locks for agent-2
	agent2Locks, err := manager.GetAgentLocks(ctx, "agent-2")
	require.NoError(t, err, "GetAgentLocks should succeed")
	assert.Len(t, agent2Locks, 1, "Agent-2 should have 1 lock")
	assert.Equal(t, "/test/resource3", agent2Locks[0].ResourcePath, "Agent-2 should have lock on resource3")
}

// TestEnhancedLockManager_ExtendLock tests extending a lock's TTL
func TestEnhancedLockManager_ExtendLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	originalLock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	originalExpiry := originalLock.ExpiresAt.AsTime()
	
	// Wait a bit to ensure time advances
	time.Sleep(10 * time.Millisecond)
	
	// Extend the lock
	extendedLock, err := manager.ExtendLock(ctx, "/test/resource", "agent-1", originalLock.LockId, 2*time.Minute)
	require.NoError(t, err, "Lock extension should succeed")
	
	// Verify the lock was extended
	assert.Equal(t, originalLock.LockId, extendedLock.LockId, "Lock ID should remain the same")
	assert.True(t, extendedLock.ExpiresAt.AsTime().After(originalExpiry), "Expiry time should be extended")
	
	// Verify the lock in the manager was updated
	manager.mu.RLock()
	storedLock := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.Equal(t, extendedLock.ExpiresAt.AsTime(), storedLock.ExpiresAt.AsTime(), "Stored lock should have updated expiry")
}

// TestEnhancedLockManager_CleanupExpiredLocks tests cleaning up expired locks
func TestEnhancedLockManager_CleanupExpiredLocks(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Acquire a lock with very short TTL
	ctx := context.Background()
	_, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", 10*time.Millisecond)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	// Wait for lock to expire
	time.Sleep(20 * time.Millisecond)
	
	// Verify lock exists before cleanup
	manager.mu.RLock()
	assert.Len(t, manager.locks, 1, "Lock should exist before cleanup")
	manager.mu.RUnlock()
	
	// Run cleanup
	manager.performCleanup()
	
	// Verify lock no longer exists
	manager.mu.RLock()
	assert.Len(t, manager.locks, 0, "Lock should be removed after cleanup")
	manager.mu.RUnlock()
	
	// Verify agent locks are updated
	manager.mu.RLock()
	_, exists := manager.agentLocks["agent-1"]
	manager.mu.RUnlock()
	
	assert.False(t, exists, "Agent locks should be removed after cleanup")
}

// TestEnhancedLockManager_ConcurrentAcquire tests concurrent lock acquisition
func TestEnhancedLockManager_ConcurrentAcquire(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
	
	mockStorage := NewMockStorage()
	config := DefaultEnhancedConfig()
	manager := NewEnhancedLockManager(config, mockStorage)
	
	// Start lock manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := manager.Start(ctx)
	require.NoError(t, err, "Starting lock manager should succeed")
	
	// Number of concurrent agents
	const numAgents = 5
	
	// Channel to collect results
	results := make(chan bool, numAgents)
	
	// Launch concurrent lock acquisitions
	for i := 0; i < numAgents; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		go func(id string) {
			// Timeout context
			lockCtx, lockCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer lockCancel()
			
			lock, err := manager.AcquireLock(lockCtx, "/test/shared-resource", id, time.Second)
			if err != nil {
				// Failed to acquire the lock
				results <- false
				return
			}
			
			// Successfully acquired the lock
			results <- true
			
			// Hold the lock briefly
			time.Sleep(100 * time.Millisecond)
			
			// Release the lock
			manager.ReleaseLock(ctx, "/test/shared-resource", id, lock.LockId)
		}(agentID)
	}
	
	// Collect results
	successCount := 0
	failCount := 0
	
	for i := 0; i < numAgents; i++ {
		if result := <-results; result {
			successCount++
		} else {
			failCount++
		}
	}
	
	// Verify that exactly one agent succeeded
	assert.Equal(t, 1, successCount, "Exactly one agent should successfully acquire the lock")
	assert.Equal(t, numAgents-1, failCount, "The rest should fail")
}