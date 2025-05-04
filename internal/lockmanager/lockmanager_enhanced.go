package lockmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/domain"
	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Ensure EnhancedLockManager implements domain.LockManager
var _ domain.LockManager = (*EnhancedLockManager)(nil)

// Config contains lock manager configuration
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

// DefaultEnhancedConfig returns a default configuration
func DefaultEnhancedConfig() EnhancedConfig {
	return EnhancedConfig{
		DefaultTTL:          time.Minute,
		CleanupInterval:     time.Minute,
		MaxLocksPerAgent:    100,
		AcquisitionTimeout:  5 * time.Second,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		RetryBackoff:        1.5,
		RetryJitter:         20 * time.Millisecond,
	}
}

// Resources sharing a common prefix are considered to be of the same type
// This is used for categorizing metrics
func extractResourceType(resourcePath string) string {
	parts := strings.Split(resourcePath, "/")
	if len(parts) > 1 {
		return parts[1] // First non-empty segment after split
	}
	return "unknown"
}

// LockAcquisitionError represents an error during lock acquisition
type LockAcquisitionError struct {
	ResourcePath string
	HolderAgent  string
	ExpiresAt    time.Time
	Reason       string
}

// Error implements the error interface
func (e *LockAcquisitionError) Error() string {
	return fmt.Sprintf("failed to acquire lock on '%s': %s (held by '%s' until %s)",
		e.ResourcePath, e.Reason, e.HolderAgent, e.ExpiresAt.Format(time.RFC3339))
}

// EnhancedLockManager handles resource locking and concurrency control with advanced features
type EnhancedLockManager struct {
	config          EnhancedConfig
	storage         StorageInterface
	locks           map[string]*proto.LockHandle
	agentLocks      map[string]map[string]struct{} // agentID -> set of resourcePaths
	mu              sync.RWMutex
	logger          zerolog.Logger
	metrics         *metrics.LockManagerMetrics
	waitingRequests map[string]*waitingRequest
	waitingMu       sync.Mutex
}

// waitingRequest represents a lock acquisition request that is waiting for a lock to be released
type waitingRequest struct {
	resourcePath string
	agentID      string
	lockID       string
	ttl          time.Duration
	deadline     time.Time
	resultCh     chan lockResult
	cancelCh     chan struct{}
}

// lockResult represents the result of a lock acquisition
type lockResult struct {
	lock *proto.LockHandle
	err  error
}

// NewEnhancedLockManager creates a new enhanced lock manager
func NewEnhancedLockManager(config EnhancedConfig, storage StorageInterface) *EnhancedLockManager {
	logger := log.With().Str("component", "lockmanager-enhanced").Logger()

	if config.DefaultTTL == 0 {
		config.DefaultTTL = DefaultEnhancedConfig().DefaultTTL
	}

	if config.CleanupInterval == 0 {
		config.CleanupInterval = DefaultEnhancedConfig().CleanupInterval
	}

	if config.MaxLocksPerAgent == 0 {
		config.MaxLocksPerAgent = DefaultEnhancedConfig().MaxLocksPerAgent
	}

	if config.AcquisitionTimeout == 0 {
		config.AcquisitionTimeout = DefaultEnhancedConfig().AcquisitionTimeout
	}

	m := metrics.GetLockManagerMetrics()

	return &EnhancedLockManager{
		config:          config,
		storage:         storage,
		locks:           make(map[string]*proto.LockHandle),
		agentLocks:      make(map[string]map[string]struct{}),
		waitingRequests: make(map[string]*waitingRequest),
		logger:          logger,
		metrics:         m,
	}
}

// Start begins the lock manager operation
func (m *EnhancedLockManager) Start(ctx context.Context) error {
	m.logger.Info().Msg("Starting enhanced lock manager")

	// Load existing locks from storage
	if err := m.loadLocks(ctx); err != nil {
		return fmt.Errorf("failed to load locks: %w", err)
	}

	// Start cleanup goroutine
	go m.cleanupExpiredLocks(ctx)

	// Start waiting request processor
	go m.processWaitingRequests(ctx)

	return nil
}

// loadLocks loads existing locks from storage
func (m *EnhancedLockManager) loadLocks(ctx context.Context) error {
	// This would use storage to load existing locks
	// For now, just initialize with empty locks
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.locks = make(map[string]*proto.LockHandle)
	m.agentLocks = make(map[string]map[string]struct{})
	return nil
}

// cleanupExpiredLocks periodically removes expired locks
func (m *EnhancedLockManager) cleanupExpiredLocks(ctx context.Context) {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performCleanup()
		case <-ctx.Done():
			return
		}
	}
}

// performCleanup removes locks that have expired
func (m *EnhancedLockManager) performCleanup() {
	now := time.Now()
	var expired []string
	var expiredAgents []string
	var expiredLockIDs []string

	m.mu.RLock()
	for path, lock := range m.locks {
		if lock.ExpiresAt.AsTime().Before(now) {
			expired = append(expired, path)
			expiredAgents = append(expiredAgents, lock.HolderAgent)
			expiredLockIDs = append(expiredLockIDs, lock.LockId)
			m.metrics.LocksExpired.Inc()
		}
	}
	m.mu.RUnlock()

	if len(expired) > 0 {
		m.mu.Lock()
		for i, path := range expired {
			// Double check that it's still expired
			if lock, exists := m.locks[path]; exists && lock.ExpiresAt.AsTime().Before(now) {
				// Remove from agent locks
				agentID := expiredAgents[i]
				if agentLocks, exists := m.agentLocks[agentID]; exists {
					delete(agentLocks, path)
					if len(agentLocks) == 0 {
						delete(m.agentLocks, agentID)
					}
				}

				// Remove from locks
				delete(m.locks, path)
				m.metrics.LocksCleanedUp.Inc()

				// Update active locks metric
				m.metrics.ActiveLocks.Dec()

				m.logger.Debug().
					Str("resource", path).
					Str("agent", lock.HolderAgent).
					Msg("Cleaned up expired lock")

				// Notify any waiting requests
				m.notifyWaitingRequestsLater(path)
			}
		}
		m.mu.Unlock()
	}
}

// notifyWaitingRequestsLater schedules notification of waiting requests in a goroutine
func (m *EnhancedLockManager) notifyWaitingRequestsLater(resourcePath string) {
	go func() {
		// Small delay to allow the cleanup to complete
		time.Sleep(10 * time.Millisecond)
		m.notifyWaitingRequests(resourcePath)
	}()
}

// notifyWaitingRequests notifies any waiting requests that a lock has been released
func (m *EnhancedLockManager) notifyWaitingRequests(resourcePath string) {
	m.waitingMu.Lock()
	defer m.waitingMu.Unlock()

	// Find waiting requests for this resource
	req, exists := m.waitingRequests[resourcePath]
	if !exists {
		return
	}

	// Remove from waiting requests
	delete(m.waitingRequests, resourcePath)

	// Signal the waiting request
	select {
	case req.resultCh <- lockResult{nil, nil}: // Signal that the lock is available
	default:
		// Channel is closed or full, request may have timed out or been cancelled
	}
}

// processWaitingRequests processes waiting lock acquisition requests
func (m *EnhancedLockManager) processWaitingRequests(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkWaitingRequests()
		case <-ctx.Done():
			return
		}
	}
}

// checkWaitingRequests checks if any waiting requests can be fulfilled or have timed out
func (m *EnhancedLockManager) checkWaitingRequests() {
	now := time.Now()
	var timedOut []string
	var tryAcquire []string

	m.waitingMu.Lock()
	defer m.waitingMu.Unlock()

	for path, req := range m.waitingRequests {
		// Check if the request has timed out
		if now.After(req.deadline) {
			timedOut = append(timedOut, path)
			continue
		}

		// Check if the lock is now available
		m.mu.RLock()
		_, exists := m.locks[path]
		m.mu.RUnlock()

		if !exists {
			tryAcquire = append(tryAcquire, path)
		}
	}

	// Handle timed out requests
	for _, path := range timedOut {
		req := m.waitingRequests[path]
		delete(m.waitingRequests, path)

		err := &LockAcquisitionError{
			ResourcePath: path,
			Reason:       "acquisition timeout",
		}

		select {
		case req.resultCh <- lockResult{nil, err}:
		default:
			// Channel is closed or full
		}

		m.metrics.LockFailures.WithLabelValues("timeout").Inc()
		m.logger.Debug().
			Str("resource", path).
			Str("agent", req.agentID).
			Msg("Lock acquisition timed out")
	}

	// Try to acquire locks that may now be available
	for _, path := range tryAcquire {
		req := m.waitingRequests[path]
		delete(m.waitingRequests, path)

		// Try to acquire the lock
		go func(r *waitingRequest) {
			lock, err := m.tryAcquireLock(context.Background(), r.resourcePath, r.agentID, r.ttl)
			select {
			case r.resultCh <- lockResult{lock, err}:
			case <-r.cancelCh:
				// Request was cancelled
			}
		}(req)
	}
}

// tryAcquireLock attempts to acquire a lock without waiting
func (m *EnhancedLockManager) tryAcquireLock(ctx context.Context, resourcePath string, agentID string, ttl time.Duration) (*proto.LockHandle, error) {
	if ttl == 0 {
		ttl = m.config.DefaultTTL
	}

	// Create lock request
	req := &proto.AcquireLockRequest{
		ResourcePath: resourcePath,
		AgentId:      agentID,
		TtlSeconds:   int32(ttl.Seconds()),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	resourceType := extractResourceType(resourcePath)

	// Check if agent has reached its lock limit
	if m.config.MaxLocksPerAgent > 0 {
		agentLocks, exists := m.agentLocks[agentID]
		if exists && len(agentLocks) >= m.config.MaxLocksPerAgent {
			m.metrics.LockFailures.WithLabelValues("max_locks_per_agent").Inc()
			return nil, fmt.Errorf("agent has reached maximum number of locks (%d)", m.config.MaxLocksPerAgent)
		}
	}

	// Check if lock is already held
	existingLock, exists := m.locks[resourcePath]
	if exists {
		// If lock exists and is not expired
		if existingLock.ExpiresAt.AsTime().After(time.Now()) {
			// If same agent, extend the lock
			if existingLock.HolderAgent == agentID {
				originalExpiry := existingLock.ExpiresAt.AsTime()
				existingLock.ExpiresAt = timestamppb.New(time.Now().Add(ttl))

				// Update in storage asynchronously
				go func() {
					if _, err := m.storage.AcquireLock(context.Background(), req); err != nil {
						m.logger.Error().Err(err).
							Str("resource", resourcePath).
							Str("agent", agentID).
							Msg("Failed to persist lock extension")
					}
				}()

				m.metrics.LockAcquisitions.WithLabelValues(resourceType, "true").Inc()
				m.logger.Debug().
					Str("resource", resourcePath).
					Str("agent", agentID).
					Str("lock_id", existingLock.LockId).
					Time("original_expiry", originalExpiry).
					Time("new_expiry", existingLock.ExpiresAt.AsTime()).
					Msg("Lock extended")

				return existingLock, nil
			}

			// Record contention
			m.metrics.LockContention.WithLabelValues(resourceType).Inc()
			m.metrics.LockFailures.WithLabelValues("contention").Inc()

			// Another agent holds the lock
			return nil, &LockAcquisitionError{
				ResourcePath: resourcePath,
				HolderAgent:  existingLock.HolderAgent,
				ExpiresAt:    existingLock.ExpiresAt.AsTime(),
				Reason:       "resource already locked",
			}
		}
		// Lock exists but is expired, can be taken (will be handled below)
		m.metrics.LocksExpired.Inc()
	}

	// Create new lock
	lockID := uuid.New().String()
	lock := &proto.LockHandle{
		ResourcePath: resourcePath,
		HolderAgent:  agentID,
		ExpiresAt:    timestamppb.New(time.Now().Add(ttl)),
		LockId:       lockID,
	}

	// Store in memory
	m.locks[resourcePath] = lock

	// Update agent locks
	agentLocks, exists := m.agentLocks[agentID]
	if !exists {
		agentLocks = make(map[string]struct{})
		m.agentLocks[agentID] = agentLocks
	}
	agentLocks[resourcePath] = struct{}{}

	// Update active locks metric
	m.metrics.ActiveLocks.Inc()
	m.metrics.LockAcquisitions.WithLabelValues(resourceType, "true").Inc()

	// Store in storage asynchronously with retries
	go func() {
		success := false
		retryDelay := m.config.RetryDelay

		for retries := 0; retries <= m.config.MaxRetries; retries++ {
			if retries > 0 {
				// Add jitter to retry delay
				jitter := time.Duration(float64(m.config.RetryJitter) * (2.0*rand.Float64() - 1.0))
				time.Sleep(retryDelay + jitter)
				retryDelay = time.Duration(float64(retryDelay) * m.config.RetryBackoff)
			}

			_, err := m.storage.AcquireLock(context.Background(), req)
			if err == nil {
				success = true
				break
			}

			m.logger.Warn().Err(err).
				Str("resource", resourcePath).
				Str("agent", agentID).
				Int("retry", retries+1).
				Int("max_retries", m.config.MaxRetries).
				Msg("Failed to persist lock, retrying")
		}

		if !success {
			m.logger.Error().
				Str("resource", resourcePath).
				Str("agent", agentID).
				Msg("Failed to persist lock after retries")

			// If we can't persist, remove from memory
			m.mu.Lock()
			if current, exists := m.locks[resourcePath]; exists && current.LockId == lockID {
				delete(m.locks, resourcePath)

				// Remove from agent locks
				if agentLocks, exists := m.agentLocks[agentID]; exists {
					delete(agentLocks, resourcePath)
					if len(agentLocks) == 0 {
						delete(m.agentLocks, agentID)
					}
				}

				// Update active locks metric
				m.metrics.ActiveLocks.Dec()
			}
			m.mu.Unlock()
		}
	}()

	m.logger.Debug().
		Str("resource", resourcePath).
		Str("agent", agentID).
		Str("lock_id", lockID).
		Time("expires", lock.ExpiresAt.AsTime()).
		Msg("Lock acquired")

	return lock, nil
}

// AcquireLock attempts to acquire a lock on a resource with context support
func (m *EnhancedLockManager) AcquireLock(ctx context.Context, resourcePath string, agentID string, ttl time.Duration) (*proto.LockHandle, error) {
	// Start timing the acquisition
	startTime := time.Now()
	resourceType := extractResourceType(resourcePath)
	timer := prometheus.NewTimer(m.metrics.LockAcquisitionDuration.WithLabelValues(resourceType))
	defer timer.ObserveDuration()

	// Use provided context or default timeout
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, m.config.AcquisitionTimeout)
		defer cancel()
	}

	// Try to acquire the lock immediately
	lock, err := m.tryAcquireLock(ctx, resourcePath, agentID, ttl)
	if err == nil {
		return lock, nil
	}

	// Check if it's a contention error and we can wait
	var lockErr *LockAcquisitionError
	if !errors.As(err, &lockErr) || lockErr.Reason != "resource already locked" {
		// Not a contention error or not held by another agent
		return nil, err
	}

	// Set up a waiting request
	resultCh := make(chan lockResult, 1)
	cancelCh := make(chan struct{})
	
	waitReq := &waitingRequest{
		resourcePath: resourcePath,
		agentID:      agentID,
		ttl:          ttl,
		deadline:     time.Now().Add(m.config.AcquisitionTimeout),
		resultCh:     resultCh,
		cancelCh:     cancelCh,
	}

	// Register the waiting request
	m.waitingMu.Lock()
	m.waitingRequests[resourcePath] = waitReq
	m.waitingMu.Unlock()

	// Ensure we clean up if we exit early
	defer func() {
		m.waitingMu.Lock()
		if req, exists := m.waitingRequests[resourcePath]; exists && req == waitReq {
			delete(m.waitingRequests, resourcePath)
			close(cancelCh)
		}
		m.waitingMu.Unlock()
	}()

	// Wait for the result or context cancellation
	select {
	case result := <-resultCh:
		if result.err != nil {
			m.metrics.LockAcquisitions.WithLabelValues(resourceType, "false").Inc()
			m.logger.Debug().
				Err(result.err).
				Str("resource", resourcePath).
				Str("agent", agentID).
				Dur("wait_time", time.Since(startTime)).
				Msg("Lock acquisition failed after waiting")
			return nil, result.err
		}

		// Try again now that the lock is available
		lock, err := m.tryAcquireLock(ctx, resourcePath, agentID, ttl)
		if err != nil {
			m.metrics.LockAcquisitions.WithLabelValues(resourceType, "false").Inc()
			m.logger.Debug().
				Err(err).
				Str("resource", resourcePath).
				Str("agent", agentID).
				Dur("wait_time", time.Since(startTime)).
				Msg("Lock acquisition failed after waiting")
			return nil, err
		}

		m.logger.Debug().
			Str("resource", resourcePath).
			Str("agent", agentID).
			Str("lock_id", lock.LockId).
			Dur("wait_time", time.Since(startTime)).
			Msg("Lock acquired after waiting")
		return lock, nil

	case <-ctx.Done():
		m.metrics.LockFailures.WithLabelValues("context_cancelled").Inc()
		m.logger.Debug().
			Err(ctx.Err()).
			Str("resource", resourcePath).
			Str("agent", agentID).
			Dur("wait_time", time.Since(startTime)).
			Msg("Lock acquisition cancelled by context")
		return nil, ctx.Err()
	}
}

// ReleaseLock releases a lock on a resource
func (m *EnhancedLockManager) ReleaseLock(ctx context.Context, resourcePath string, agentID string, lockID string) (bool, error) {
	startTime := time.Now()
	resourceType := extractResourceType(resourcePath)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if lock exists
	existingLock, exists := m.locks[resourcePath]
	if !exists {
		m.metrics.LockReleases.WithLabelValues(resourceType, "false").Inc()
		return false, nil
	}

	// Verify ownership
	if existingLock.HolderAgent != agentID {
		m.metrics.LockReleases.WithLabelValues(resourceType, "false").Inc()
		m.metrics.LockFailures.WithLabelValues("wrong_agent").Inc()
		return false, fmt.Errorf("lock is held by another agent")
	}

	// Verify lock ID
	if existingLock.LockId != lockID {
		m.metrics.LockReleases.WithLabelValues(resourceType, "false").Inc()
		m.metrics.LockFailures.WithLabelValues("lock_id_mismatch").Inc()
		return false, fmt.Errorf("lock ID mismatch")
	}

	// Record lock hold duration
	holdDuration := time.Since(existingLock.ExpiresAt.AsTime().Add(-m.config.DefaultTTL))
	m.metrics.LockHoldDuration.WithLabelValues(resourceType).Observe(holdDuration.Seconds())

	// Remove from memory
	delete(m.locks, resourcePath)

	// Remove from agent locks
	if agentLocks, exists := m.agentLocks[agentID]; exists {
		delete(agentLocks, resourcePath)
		if len(agentLocks) == 0 {
			delete(m.agentLocks, agentID)
		}
	}

	// Update active locks metric
	m.metrics.ActiveLocks.Dec()
	m.metrics.LockReleases.WithLabelValues(resourceType, "true").Inc()

	// Release in storage asynchronously with retries
	go func() {
		req := &proto.ReleaseLockRequest{
			ResourcePath: resourcePath,
			AgentId:      agentID,
			LockId:       lockID,
		}

		success := false
		retryDelay := m.config.RetryDelay

		for retries := 0; retries <= m.config.MaxRetries; retries++ {
			if retries > 0 {
				// Add jitter to retry delay
				jitter := time.Duration(float64(m.config.RetryJitter) * (2.0*rand.Float64() - 1.0))
				time.Sleep(retryDelay + jitter)
				retryDelay = time.Duration(float64(retryDelay) * m.config.RetryBackoff)
			}

			released, err := m.storage.ReleaseLock(context.Background(), req)
			if err == nil && released {
				success = true
				break
			}

			m.logger.Warn().Err(err).
				Str("resource", resourcePath).
				Str("agent", agentID).
				Int("retry", retries+1).
				Int("max_retries", m.config.MaxRetries).
				Msg("Failed to release lock in storage, retrying")
		}

		if !success {
			m.logger.Error().
				Str("resource", resourcePath).
				Str("agent", agentID).
				Msg("Failed to release lock in storage after retries")
		}

		// Notify waiting requests regardless of storage operation success
		// since the lock is released in memory
		m.notifyWaitingRequests(resourcePath)
	}()

	m.logger.Debug().
		Str("resource", resourcePath).
		Str("agent", agentID).
		Str("lock_id", lockID).
		Dur("hold_duration", holdDuration).
		Msg("Lock released")

	return true, nil
}

// GetLock retrieves information about a lock
func (m *EnhancedLockManager) GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, exists := m.locks[resourcePath]
	if !exists {
		return nil, fmt.Errorf("no lock found for resource: %s", resourcePath)
	}

	// Check if lock is expired
	if lock.ExpiresAt.AsTime().Before(time.Now()) {
		return nil, fmt.Errorf("lock found but expired for resource: %s", resourcePath)
	}

	return lock, nil
}

// AcquireMultipleLocks acquires multiple locks in a deadlock-free manner
func (m *EnhancedLockManager) AcquireMultipleLocks(ctx context.Context, resources []string, agentID string, ttl time.Duration) ([]*proto.LockHandle, error) {
	if len(resources) == 0 {
		return nil, nil
	}

	// Sort resources to avoid deadlocks
	sortedResources := make([]string, len(resources))
	copy(sortedResources, resources)
	sort.Strings(sortedResources)

	// Try to acquire all locks
	locks := make([]*proto.LockHandle, 0, len(sortedResources))
	var acquiredLockIDs []string
	
	for _, resource := range sortedResources {
		lock, err := m.AcquireLock(ctx, resource, agentID, ttl)
		if err != nil {
			// Failed to acquire lock, release any locks we've already acquired
			for i, acquiredResource := range sortedResources[:len(locks)] {
				m.ReleaseLock(ctx, acquiredResource, agentID, acquiredLockIDs[i])
			}
			return nil, fmt.Errorf("failed to acquire lock on '%s': %w", resource, err)
		}
		locks = append(locks, lock)
		acquiredLockIDs = append(acquiredLockIDs, lock.LockId)
	}

	return locks, nil
}

// ListLocks returns all active locks (for monitoring and debugging)
func (m *EnhancedLockManager) ListLocks(ctx context.Context) ([]*proto.LockHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	result := make([]*proto.LockHandle, 0, len(m.locks))

	for _, lock := range m.locks {
		// Only include non-expired locks
		if lock.ExpiresAt.AsTime().After(now) {
			// Create a copy to avoid mutation
			lockCopy := *lock
			result = append(result, &lockCopy)
		}
	}

	return result, nil
}

// GetAgentLocks returns all locks held by a specific agent
func (m *EnhancedLockManager) GetAgentLocks(ctx context.Context, agentID string) ([]*proto.LockHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	result := make([]*proto.LockHandle, 0)

	agentLocks, exists := m.agentLocks[agentID]
	if !exists {
		return result, nil
	}

	for resourcePath := range agentLocks {
		if lock, exists := m.locks[resourcePath]; exists && lock.ExpiresAt.AsTime().After(now) {
			// Create a copy to avoid mutation
			lockCopy := *lock
			result = append(result, &lockCopy)
		}
	}

	return result, nil
}

// GetLockCount returns the count of active locks
func (m *EnhancedLockManager) GetLockCount(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	count := 0

	for _, lock := range m.locks {
		if lock.ExpiresAt.AsTime().After(now) {
			count++
		}
	}

	return count, nil
}

// ExtendLock extends the TTL of an existing lock
func (m *EnhancedLockManager) ExtendLock(ctx context.Context, resourcePath, agentID, lockID string, extension time.Duration) (*proto.LockHandle, error) {
	resourceType := extractResourceType(resourcePath)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if lock exists
	existingLock, exists := m.locks[resourcePath]
	if !exists {
		m.metrics.LockFailures.WithLabelValues("not_found").Inc()
		return nil, fmt.Errorf("lock not found for resource: %s", resourcePath)
	}

	// Verify ownership
	if existingLock.HolderAgent != agentID {
		m.metrics.LockFailures.WithLabelValues("wrong_agent").Inc()
		return nil, fmt.Errorf("lock is held by another agent")
	}

	// Verify lock ID
	if existingLock.LockId != lockID {
		m.metrics.LockFailures.WithLabelValues("lock_id_mismatch").Inc()
		return nil, fmt.Errorf("lock ID mismatch")
	}

	// Check if lock is expired
	if existingLock.ExpiresAt.AsTime().Before(time.Now()) {
		m.metrics.LockFailures.WithLabelValues("expired").Inc()
		return nil, fmt.Errorf("lock is expired")
	}

	// Extend the lock
	newExpiry := time.Now().Add(extension)
	existingLock.ExpiresAt = timestamppb.New(newExpiry)

	// Update in storage asynchronously
	go func() {
		req := &proto.AcquireLockRequest{
			ResourcePath: resourcePath,
			AgentId:      agentID,
			TtlSeconds:   int32(extension.Seconds()),
		}

		if _, err := m.storage.AcquireLock(context.Background(), req); err != nil {
			m.logger.Error().Err(err).
				Str("resource", resourcePath).
				Str("agent", agentID).
				Msg("Failed to persist lock extension")
		}
	}()

	m.logger.Debug().
		Str("resource", resourcePath).
		Str("agent", agentID).
		Str("lock_id", lockID).
		Time("new_expiry", newExpiry).
		Msg("Lock extended")

	return existingLock, nil
}

// Shutdown performs cleanup and stops the lock manager
func (m *EnhancedLockManager) Shutdown(ctx context.Context) error {
	m.logger.Info().Msg("Shutting down lock manager")
	// No special cleanup needed, just return
	return nil
}

// For random jitter in retries
var rand = struct {
	sync.Mutex
	*rand.Rand
}{
	Rand: rand.New(rand.NewSource(time.Now().UnixNano())),
}

func (r *rand) Float64() float64 {
	r.Lock()
	defer r.Unlock()
	return r.Rand.Float64()
}