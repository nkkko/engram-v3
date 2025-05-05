# Lock Manager in Engram v3

This document describes the lock manager implementation in Engram v3, including its purpose, features, and usage patterns.

## Overview

The Lock Manager provides distributed locking capabilities that enable multiple agents to coordinate access to shared resources. It ensures that only one agent can access a resource at a time, preventing race conditions and data corruption.

## Features

- **Resource-Path Based Locks**: Lock resources by their path (e.g., `/files/document.txt`)
- **TTL-Based Expiration**: Automatic lock expiration to prevent deadlocks
- **Atomic Operations**: Acquire, check, and release operations are atomic
- **Retry Mechanism**: Automatic retry with exponential backoff
- **Context Cancellation**: Support for cancellation via context
- **Detailed Metrics**: Comprehensive metrics for monitoring
- **Deadlock Prevention**: Lexical ordering of locks to prevent deadlocks
- **Extensible Storage Backend**: Currently uses BadgerDB with factory pattern

## API

### Lock Acquisition

```
POST /locks
```

Request:
```json
{
  "resource_path": "/example/resource.txt",
  "agent_id": "agent-123",
  "ttl_seconds": 60,
  "wait": true,
  "wait_timeout_seconds": 10
}
```

Response:
```json
{
  "lock": {
    "resource_path": "/example/resource.txt",
    "holder_agent": "agent-123",
    "expires_at": "2023-09-01T12:35:00Z",
    "lock_id": "lock-456"
  }
}
```

### Lock Status Check

```
GET /locks/:resource
```

Response:
```json
{
  "lock": {
    "resource_path": "/example/resource.txt",
    "holder_agent": "agent-123",
    "expires_at": "2023-09-01T12:35:00Z",
    "lock_id": "lock-456"
  }
}
```

### Lock Release

```
DELETE /locks/:resource
```

Query Parameters:
- `agent_id`: The ID of the agent releasing the lock
- `lock_id`: The ID of the lock to release

Response:
```json
{
  "released": true
}
```

## Implementation Details

### Lock Manager Interface

```go
type LockManager interface {
    Acquire(ctx context.Context, resourcePath, agentID string, ttlSeconds int) (Lock, error)
    AcquireWithRetry(ctx context.Context, resourcePath, agentID string, ttlSeconds int, retryOpts RetryOptions) (Lock, error)
    Check(ctx context.Context, resourcePath string) (Lock, bool, error)
    Release(ctx context.Context, resourcePath, agentID, lockID string) (bool, error)
    Refresh(ctx context.Context, resourcePath, agentID, lockID string, ttlSeconds int) (Lock, error)
    Close() error
}
```

### Lock Structure

```go
type Lock struct {
    ResourcePath string    `json:"resource_path"`
    HolderAgent  string    `json:"holder_agent"`
    ExpiresAt    time.Time `json:"expires_at"`
    LockID       string    `json:"lock_id"`
}
```

### Lock Storage

Locks are stored in BadgerDB with a prefix-based key scheme:

```go
func lockKey(resourcePath string) []byte {
    return []byte(fmt.Sprintf("lock_%s", resourcePath))
}
```

### TTL Management

Locks automatically expire after their TTL:

```go
func (m *Manager) Acquire(ctx context.Context, resourcePath, agentID string, ttlSeconds int) (Lock, error) {
    // Create the lock
    lock := Lock{
        ResourcePath: resourcePath,
        HolderAgent:  agentID,
        ExpiresAt:    time.Now().Add(time.Duration(ttlSeconds) * time.Second),
        LockID:       uuid.New().String(),
    }
    
    // Store the lock with TTL...
}
```

### Retry Mechanism

The lock manager supports automatic retry with exponential backoff:

```go
type RetryOptions struct {
    MaxAttempts      int
    InitialBackoff   time.Duration
    MaxBackoff       time.Duration
    BackoffMultiplier float64
}

func (m *Manager) AcquireWithRetry(ctx context.Context, resourcePath, agentID string, ttlSeconds int, retryOpts RetryOptions) (Lock, error) {
    var lock Lock
    var err error
    
    backoff := retryOpts.InitialBackoff
    
    for attempt := 0; attempt < retryOpts.MaxAttempts; attempt++ {
        lock, err = m.Acquire(ctx, resourcePath, agentID, ttlSeconds)
        if err == nil {
            return lock, nil
        }
        
        if !errors.Is(err, ErrResourceLocked) {
            return Lock{}, err
        }
        
        // Apply jitter to prevent thundering herd
        jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
        sleepTime := backoff + jitter
        
        select {
        case <-time.After(sleepTime):
            // Increase backoff for next attempt
            backoff = time.Duration(float64(backoff) * retryOpts.BackoffMultiplier)
            if backoff > retryOpts.MaxBackoff {
                backoff = retryOpts.MaxBackoff
            }
        case <-ctx.Done():
            return Lock{}, ctx.Err()
        }
    }
    
    return Lock{}, fmt.Errorf("failed to acquire lock after %d attempts: %w", retryOpts.MaxAttempts, err)
}
```

### Lock Refreshing

Locks can be refreshed to extend their TTL:

```go
func (m *Manager) Refresh(ctx context.Context, resourcePath, agentID, lockID string, ttlSeconds int) (Lock, error) {
    // Check if the lock exists and is held by the requesting agent
    existingLock, exists, err := m.Check(ctx, resourcePath)
    if err != nil {
        return Lock{}, err
    }
    
    if !exists {
        return Lock{}, ErrLockNotFound
    }
    
    if existingLock.HolderAgent != agentID || existingLock.LockID != lockID {
        return Lock{}, ErrNotLockHolder
    }
    
    // Update the expiration time
    existingLock.ExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
    
    // Store the updated lock
    // ...
    
    return existingLock, nil
}
```

### Metrics

The lock manager provides detailed metrics:

```go
type LockManagerMetrics struct {
    AcquireTotal      *prometheus.CounterVec
    ReleaseTotal      *prometheus.CounterVec
    RefreshTotal      *prometheus.CounterVec
    CheckTotal        *prometheus.CounterVec
    AcquireDuration   *prometheus.HistogramVec
    ReleaseDuration   *prometheus.HistogramVec
    RefreshDuration   *prometheus.HistogramVec
    CheckDuration     *prometheus.HistogramVec
    WaitDuration      *prometheus.HistogramVec
    ActiveLocks       prometheus.Gauge
    ContentionTotal   prometheus.Counter
    ExpiredTotal      prometheus.Counter
    StaleReleaseTotal prometheus.Counter
}
```

## Usage Patterns

### Basic Lock/Unlock Pattern

```go
// Acquire lock
lock, err := lockManager.Acquire(ctx, "/resources/file.txt", "agent-123", 60)
if err != nil {
    return err
}

// Do work with the locked resource
// ...

// Release lock
released, err := lockManager.Release(ctx, lock.ResourcePath, lock.HolderAgent, lock.LockID)
if err != nil {
    return err
}
```

### Wait with Context Cancellation

```go
// Create a context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Attempt to acquire with retry
retryOpts := lockmanager.RetryOptions{
    MaxAttempts:       10,
    InitialBackoff:    100 * time.Millisecond,
    MaxBackoff:        5 * time.Second,
    BackoffMultiplier: 1.5,
}

lock, err := lockManager.AcquireWithRetry(ctx, "/resources/file.txt", "agent-123", 60, retryOpts)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return fmt.Errorf("timed out waiting for lock")
    }
    return err
}

// Do work with the locked resource
// ...

// Release lock
released, err := lockManager.Release(ctx, lock.ResourcePath, lock.HolderAgent, lock.LockID)
if err != nil {
    return err
}
```

### Periodic Refresh

```go
func lockWithRefresh(ctx context.Context, lm lockmanager.LockManager, resourcePath, agentID string, ttlSeconds int) error {
    // Acquire lock
    lock, err := lm.Acquire(ctx, resourcePath, agentID, ttlSeconds)
    if err != nil {
        return err
    }
    
    // Create a refresh ticker at half the TTL interval
    refreshInterval := time.Duration(ttlSeconds/2) * time.Second
    ticker := time.NewTicker(refreshInterval)
    defer ticker.Stop()
    
    // Start a goroutine to handle the lock
    errCh := make(chan error, 1)
    doneCh := make(chan struct{})
    
    go func() {
        defer close(errCh)
        
        for {
            select {
            case <-ticker.C:
                // Refresh the lock
                refreshedLock, err := lm.Refresh(ctx, resourcePath, agentID, lock.LockID, ttlSeconds)
                if err != nil {
                    errCh <- fmt.Errorf("failed to refresh lock: %w", err)
                    return
                }
                lock = refreshedLock
            case <-doneCh:
                // Work is done, release the lock
                _, err := lm.Release(ctx, resourcePath, agentID, lock.LockID)
                if err != nil {
                    errCh <- fmt.Errorf("failed to release lock: %w", err)
                }
                return
            case <-ctx.Done():
                // Context cancelled, release the lock
                _, _ = lm.Release(context.Background(), resourcePath, agentID, lock.LockID)
                errCh <- ctx.Err()
                return
            }
        }
    }()
    
    // Do the work here
    // ...
    
    // Signal that work is done
    close(doneCh)
    
    // Wait for any errors from the goroutine
    if err := <-errCh; err != nil {
        return err
    }
    
    return nil
}
```

### Multiple Lock Acquisition (Deadlock Prevention)

```go
func acquireMultipleLocks(ctx context.Context, lm lockmanager.LockManager, resourcePaths []string, agentID string, ttlSeconds int) ([]lockmanager.Lock, error) {
    // Sort resource paths to prevent deadlocks
    sort.Strings(resourcePaths)
    
    locks := make([]lockmanager.Lock, 0, len(resourcePaths))
    
    // Function to release all acquired locks in case of error
    releaseAcquired := func() {
        for _, lock := range locks {
            _, _ = lm.Release(context.Background(), lock.ResourcePath, lock.HolderAgent, lock.LockID)
        }
    }
    
    // Acquire locks in sorted order
    for _, resourcePath := range resourcePaths {
        lock, err := lm.Acquire(ctx, resourcePath, agentID, ttlSeconds)
        if err != nil {
            releaseAcquired()
            return nil, err
        }
        locks = append(locks, lock)
    }
    
    return locks, nil
}
```

## Best Practices

### TTL Selection

- **Short-lived operations**: 30-60 seconds
- **Medium-duration tasks**: 5-15 minutes
- **Long-running processes**: Use periodic refresh

### Resource Path Naming

- Use hierarchical paths: `/category/subcategory/resource-name`
- Include version information if relevant: `/documents/report-v1.2.3`
- Avoid special characters except `/`, `-`, `_`, and `.`

### Error Handling

- Distinguish between different error types:
  - `ErrResourceLocked`: Resource is currently locked by another agent
  - `ErrLockNotFound`: Lock doesn't exist (may have expired)
  - `ErrNotLockHolder`: Agent is not the holder of the lock
  - `ErrStorageError`: Underlying storage error

### Monitoring

Key metrics to watch:
- Lock contention rate: `engram_lock_contentions_total`
- Lock acquisition time: `engram_lock_acquire_duration_seconds`
- Active locks count: `engram_locks_total{status="active"}`
- Expired locks: `engram_locks_total{status="expired"}`

### Performance Considerations

- Avoid short TTLs with frequent refreshes (high overhead)
- Be cautious with very long TTLs (risk of abandoned locks)
- Use `AcquireWithRetry` for predictable wait times

## Conclusion

The Lock Manager in Engram v3 provides a robust, distributed locking mechanism that enables coordination between multiple agents. With features like automatic retry, TTL-based expiration, and comprehensive metrics, it offers a reliable foundation for building concurrent applications.