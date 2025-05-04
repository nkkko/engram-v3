# Lock Manager Documentation

The Lock Manager in Engram v3 provides concurrency control for resources. This document describes the lock manager implementations, their features, and how to use them.

## Features

### Basic Lock Manager

The basic lock manager provides simple resource locking capabilities:

- Acquire and release locks on resources
- Default TTL for locks
- Periodic cleanup of expired locks
- Deadlock prevention with sorted lock acquisition

### Enhanced Lock Manager

The enhanced lock manager builds on the basic implementation with additional features:

1. **Context Cancellation Support**
   - Respect context deadlines and cancellation
   - Built-in support for acquisition timeouts
   - Proper cleanup on cancellation

2. **Advanced TTL Management**
   - Explicit lock extension mechanism
   - Automatic cleanup of expired locks
   - Configurable TTL for different resource types

3. **Robustness Improvements**
   - Retry support for storage operations
   - Backoff with jitter for distributed resilience
   - Waiting mechanism for contested locks

4. **Per-Agent Limits**
   - Configurable maximum locks per agent
   - Protection against resource exhaustion

5. **Metrics and Observability**
   - Detailed Prometheus metrics for monitoring
   - Lock acquisition and release tracking
   - Lock contention metrics
   - Timing of acquisition and hold durations

6. **Management Operations**
   - List all active locks
   - Get locks for a specific agent
   - Count active locks
   - Detailed lock information

## Configuration

The lock manager can be configured with different parameters:

```go
type EnhancedConfig struct {
    // Default TTL for locks
    DefaultTTL time.Duration

    // How often to clean expired locks
    CleanupInterval time.Duration

    // Maximum locks per agent
    MaxLocksPerAgent int

    // Lock acquisition timeout
    AcquisitionTimeout time.Duration

    // Retry settings
    MaxRetries     int
    RetryDelay     time.Duration
    RetryBackoff   float64
    RetryJitter    time.Duration
}
```

## Usage

### Acquiring a Lock

```go
// Acquire a lock with context support
lock, err := lockManager.AcquireLock(ctx, "/resources/myresource", "agent-123", 1*time.Minute)
if err != nil {
    // Handle error (maybe wait and retry)
    return err
}

// Use the locked resource
// ...

// Release the lock when done
released, err := lockManager.ReleaseLock(ctx, "/resources/myresource", "agent-123", lock.LockId)
```

### Acquiring Multiple Locks

```go
// Acquire multiple locks at once (deadlock-free)
resources := []string{"/resources/res1", "/resources/res2", "/resources/res3"}
locks, err := lockManager.AcquireMultipleLocks(ctx, resources, "agent-123", 1*time.Minute)
if err != nil {
    // Handle error
    return err
}

// Use locked resources
// ...

// Release all locks
for _, lock := range locks {
    lockManager.ReleaseLock(ctx, lock.ResourcePath, "agent-123", lock.LockId)
}
```

### Extending a Lock

```go
// Extend an existing lock
extendedLock, err := lockManager.ExtendLock(ctx, "/resources/myresource", "agent-123", lock.LockId, 2*time.Minute)
if err != nil {
    // Handle error
    return err
}
```

### Listing and Managing Locks

```go
// List all active locks
locks, err := lockManager.ListLocks(ctx)

// Get locks held by a specific agent
agentLocks, err := lockManager.GetAgentLocks(ctx, "agent-123")

// Get lock count
count, err := lockManager.GetLockCount(ctx)
```

## Metrics

The enhanced lock manager provides the following Prometheus metrics:

- `engram_lock_acquisitions_total` - Total number of lock acquisitions
- `engram_lock_releases_total` - Total number of lock releases
- `engram_lock_failures_total` - Total number of lock acquisition failures
- `engram_lock_contention_total` - Total number of lock contentions
- `engram_active_locks` - Number of currently active locks
- `engram_lock_acquisition_duration_seconds` - Duration of lock acquisitions
- `engram_lock_hold_duration_seconds` - Duration locks are held
- `engram_locks_expired_total` - Total number of locks that expired
- `engram_locks_cleaned_up_total` - Total number of locks that were cleaned up

## Implementation Selection

The system supports both the basic and enhanced lock manager implementations. You can select which implementation to use through the factory:

```go
// Use the enhanced implementation
config := lockmanager.FactoryConfig{
    Type:              lockmanager.EnhancedImplementation,
    DefaultTTL:        1 * time.Minute,
    CleanupInterval:   1 * time.Minute,
    MaxLocksPerAgent:  100,
    AcquisitionTimeout: 5 * time.Second,
    MaxRetries:        3,
    RetryDelay:        100 * time.Millisecond,
    RetryBackoff:      1.5,
    RetryJitter:       20 * time.Millisecond,
}
lockMgr := lockmanager.NewLockManagerFactory(config, storage)
```