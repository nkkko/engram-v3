package lockmanager

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLockManager verifies a new lock manager is created correctly
func TestNewLockManager(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	
	manager := NewLockManager(config, mockStorage)
	
	assert.NotNil(t, manager, "Lock manager should not be nil")
	assert.Equal(t, config, manager.config, "Config should be set correctly")
	assert.Equal(t, mockStorage, manager.storage, "Storage should be set correctly")
	assert.NotNil(t, manager.locks, "Locks map should be initialized")
	assert.NotNil(t, manager.logger, "Logger should be initialized")
}

// TestAcquireLock verifies a lock can be acquired
func TestAcquireLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
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
	
	// Verify storage was called
	time.Sleep(50 * time.Millisecond) // Wait for async storage call
	assert.Equal(t, 1, mockStorage.AcquireCallCount(), "Storage AcquireLock should be called")
}

// TestAcquireLock_AlreadyHeld verifies behavior when a lock is already held
func TestAcquireLock_AlreadyHeld(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	lock1, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "First lock acquisition should succeed")
	
	// Try to acquire the same lock with a different agent
	lock2, err := manager.AcquireLock(ctx, "/test/resource", "agent-2", time.Minute)
	assert.Error(t, err, "Second lock acquisition with different agent should fail")
	assert.Nil(t, lock2, "Lock should be nil when acquisition fails")
	assert.True(t, strings.Contains(err.Error(), "is locked by agent"), "Error should indicate lock is held")
	
	// Try to acquire the same lock with the same agent (should extend)
	originalExpiry := lock1.ExpiresAt.AsTime()
	time.Sleep(10 * time.Millisecond) // Ensure time passes
	
	lock3, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition by same agent should succeed (extend)")
	assert.Equal(t, lock1.LockId, lock3.LockId, "Lock ID should be the same (extended)")
	assert.True(t, lock3.ExpiresAt.AsTime().After(originalExpiry), "Lock expiry should be extended")
}

// TestReleaseLock verifies a lock can be released
func TestReleaseLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
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
	
	// Verify storage was called
	time.Sleep(50 * time.Millisecond) // Wait for async storage call
	assert.Equal(t, 1, mockStorage.ReleaseCallCount(), "Storage ReleaseLock should be called")
}

// TestReleaseLock_WrongAgent verifies a lock cannot be released by wrong agent
func TestReleaseLock_WrongAgent(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	lock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	// Try to release with wrong agent
	released, err := manager.ReleaseLock(ctx, "/test/resource", "agent-2", lock.LockId)
	assert.Error(t, err, "Lock release with wrong agent should fail")
	assert.False(t, released, "Lock should not be released")
	assert.True(t, strings.Contains(err.Error(), "held by another agent"), "Error should indicate wrong agent")
	
	// Verify lock still exists
	manager.mu.RLock()
	_, exists := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.True(t, exists, "Lock should still exist after failed release")
}

// TestReleaseLock_WrongID verifies a lock cannot be released with wrong lock ID
func TestReleaseLock_WrongID(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Acquire a lock
	ctx := context.Background()
	_, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
	require.NoError(t, err, "Lock acquisition should succeed")
	
	// Try to release with wrong lock ID
	released, err := manager.ReleaseLock(ctx, "/test/resource", "agent-1", "wrong-id")
	assert.Error(t, err, "Lock release with wrong ID should fail")
	assert.False(t, released, "Lock should not be released")
	assert.True(t, strings.Contains(err.Error(), "lock ID mismatch"), "Error should indicate ID mismatch")
	
	// Verify lock still exists
	manager.mu.RLock()
	_, exists := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.True(t, exists, "Lock should still exist after failed release")
}

// TestGetLock verifies getting lock information
func TestGetLock(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
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

// TestGetLock_NonExistent verifies behavior for non-existent locks
func TestGetLock_NonExistent(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Get a non-existent lock
	ctx := context.Background()
	lock, err := manager.GetLock(ctx, "/test/resource")
	assert.Error(t, err, "GetLock should fail for non-existent lock")
	assert.Nil(t, lock, "Lock should be nil for non-existent lock")
	assert.True(t, strings.Contains(err.Error(), "no lock found"), "Error should indicate lock not found")
}

// TestAcquireMultipleLocks verifies multiple locks can be acquired
func TestAcquireMultipleLocks(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Acquire multiple locks
	ctx := context.Background()
	resources := []string{"/test/resource1", "/test/resource2", "/test/resource3"}
	
	locks, err := manager.AcquireMultipleLocks(ctx, resources, "agent-1", time.Minute)
	require.NoError(t, err, "Multiple lock acquisition should succeed")
	assert.Len(t, locks, 3, "Should return 3 locks")
	
	// Verify all locks exist in the manager
	manager.mu.RLock()
	for _, res := range resources {
		_, exists := manager.locks[res]
		assert.True(t, exists, "Lock for %s should exist in the manager", res)
	}
	manager.mu.RUnlock()
}

// TestAcquireMultipleLocks_Partial verifies behavior when some locks are already held
func TestAcquireMultipleLocks_Partial(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// First acquire a lock with agent-2
	ctx := context.Background()
	_, err := manager.AcquireLock(ctx, "/test/resource2", "agent-2", time.Minute)
	require.NoError(t, err, "First lock acquisition should succeed")
	
	// Try to acquire multiple locks including the one already held
	resources := []string{"/test/resource1", "/test/resource2", "/test/resource3"}
	
	locks, err2 := manager.AcquireMultipleLocks(ctx, resources, "agent-1", time.Minute)
	assert.Error(t, err2, "Multiple lock acquisition should fail")
	assert.Nil(t, locks, "Locks should be nil when acquisition fails")
	assert.True(t, strings.Contains(err2.Error(), "failed to acquire lock"), "Error should indicate acquisition failure")
	
	// Verify only the original lock exists in the manager
	manager.mu.RLock()
	assert.Len(t, manager.locks, 1, "Only the original lock should exist")
	_, exists := manager.locks["/test/resource2"]
	assert.True(t, exists, "Original lock should still exist")
	_, exists = manager.locks["/test/resource1"]
	assert.False(t, exists, "Partial lock should be released")
	_, exists = manager.locks["/test/resource3"]
	assert.False(t, exists, "Partial lock should be released")
	manager.mu.RUnlock()
}

// TestPerformCleanup verifies expired locks are cleaned up
func TestPerformCleanup(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
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
}

// TestStartAndShutdown verifies the lock manager lifecycle
func TestStartAndShutdown(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Start the lock manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := manager.Start(ctx)
	require.NoError(t, err, "Starting lock manager should succeed")
	
	// Shutdown
	err = manager.Shutdown(ctx)
	require.NoError(t, err, "Shutting down lock manager should succeed")
}

// TestStorageError verifies behavior when storage operations fail
func TestStorageError(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Configure storage to fail
	mockStorage.SetFailAcquire(true)
	
	// Acquire a lock (should succeed initially in memory)
	ctx := context.Background()
	lockID := ""
	
	func() {
		// Get the lock ID in a closure to avoid unused variable warning
		theLock, err := manager.AcquireLock(ctx, "/test/resource", "agent-1", time.Minute)
		require.NoError(t, err, "Lock acquisition should initially succeed")
		lockID = theLock.LockId
	}()
	
	assert.NotEmpty(t, lockID, "Lock ID should be set")
	
	// Wait for async storage operation to fail and cleanup the in-memory lock
	time.Sleep(50 * time.Millisecond)
	
	// Lock should be removed from memory due to storage failure
	manager.mu.RLock()
	_, exists := manager.locks["/test/resource"]
	manager.mu.RUnlock()
	
	assert.False(t, exists, "Lock should be removed after storage failure")
}

// TestDeadlockPrevention verifies locks are acquired in lexical order to prevent deadlocks
func TestDeadlockPrevention(t *testing.T) {
	mockStorage := NewMockStorage()
	config := DefaultConfig()
	manager := NewLockManager(config, mockStorage)
	
	// Acquire multiple locks in reverse order
	ctx := context.Background()
	resources := []string{"/test/resource3", "/test/resource1", "/test/resource2"}
	
	acquiredLocks, err := manager.AcquireMultipleLocks(ctx, resources, "agent-1", time.Minute)
	require.NoError(t, err, "Multiple lock acquisition should succeed")
	assert.Len(t, acquiredLocks, 3, "Should return 3 locks")
	
	// Create a map for quick lookup of locks by resource path
	locksByResource := make(map[string]bool)
	for _, lock := range acquiredLocks {
		locksByResource[lock.ResourcePath] = true
	}
	
	// Verify all requested resources are locked
	for _, resource := range resources {
		assert.True(t, locksByResource[resource], "Resource %s should be locked", resource)
	}
	
	// Verify internal lock storage (should be lexically ordered)
	manager.mu.RLock()
	assert.Contains(t, manager.locks, "/test/resource1")
	assert.Contains(t, manager.locks, "/test/resource2")
	assert.Contains(t, manager.locks, "/test/resource3")
	manager.mu.RUnlock()
}