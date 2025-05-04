package storage

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/pkg/proto"
)

// Cache is a caching layer for frequently accessed objects
type Cache struct {
	workUnits  *lru.TwoQueueCache
	contexts   *lru.TwoQueueCache
	locks      *lru.TwoQueueCache
	mutex      sync.RWMutex
	metrics    *metrics.Metrics
	expiration time.Duration
}

// cacheItem represents an item in the cache with an expiration time
type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewCache creates a new cache with the given capacity
func NewCache(workUnitCapacity, contextCapacity, lockCapacity int, expiration time.Duration) (*Cache, error) {
	workUnitsCache, err := lru.New2Q(workUnitCapacity)
	if err != nil {
		return nil, err
	}

	contextsCache, err := lru.New2Q(contextCapacity)
	if err != nil {
		return nil, err
	}

	locksCache, err := lru.New2Q(lockCapacity)
	if err != nil {
		return nil, err
	}

	return &Cache{
		workUnits:  workUnitsCache,
		contexts:   contextsCache,
		locks:      locksCache,
		metrics:    metrics.GetMetrics(),
		expiration: expiration,
	}, nil
}

// GetWorkUnit retrieves a work unit from the cache
func (c *Cache) GetWorkUnit(id string) (*proto.WorkUnit, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	value, found := c.workUnits.Get(id)
	if !found {
		c.metrics.StorageOperations.WithLabelValues("cache_miss_work_unit", "true").Inc()
		return nil, false
	}

	item := value.(cacheItem)
	if time.Now().After(item.expiration) {
		c.workUnits.Remove(id)
		c.metrics.StorageOperations.WithLabelValues("cache_expired_work_unit", "true").Inc()
		return nil, false
	}

	c.metrics.StorageOperations.WithLabelValues("cache_hit_work_unit", "true").Inc()
	return item.value.(*proto.WorkUnit), true
}

// SetWorkUnit adds a work unit to the cache
func (c *Cache) SetWorkUnit(workUnit *proto.WorkUnit) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.workUnits.Add(workUnit.Id, cacheItem{
		value:      workUnit,
		expiration: time.Now().Add(c.expiration),
	})
	c.metrics.StorageOperations.WithLabelValues("cache_set_work_unit", "true").Inc()
}

// GetContext retrieves a context from the cache
func (c *Cache) GetContext(id string) (*proto.Context, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	value, found := c.contexts.Get(id)
	if !found {
		c.metrics.StorageOperations.WithLabelValues("cache_miss_context", "true").Inc()
		return nil, false
	}

	item := value.(cacheItem)
	if time.Now().After(item.expiration) {
		c.contexts.Remove(id)
		c.metrics.StorageOperations.WithLabelValues("cache_expired_context", "true").Inc()
		return nil, false
	}

	c.metrics.StorageOperations.WithLabelValues("cache_hit_context", "true").Inc()
	return item.value.(*proto.Context), true
}

// SetContext adds a context to the cache
func (c *Cache) SetContext(context *proto.Context) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.contexts.Add(context.Id, cacheItem{
		value:      context,
		expiration: time.Now().Add(c.expiration),
	})
	c.metrics.StorageOperations.WithLabelValues("cache_set_context", "true").Inc()
}

// GetLock retrieves a lock from the cache
func (c *Cache) GetLock(resourcePath string) (*proto.LockHandle, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	value, found := c.locks.Get(resourcePath)
	if !found {
		c.metrics.StorageOperations.WithLabelValues("cache_miss_lock", "true").Inc()
		return nil, false
	}

	item := value.(cacheItem)
	if time.Now().After(item.expiration) {
		c.locks.Remove(resourcePath)
		c.metrics.StorageOperations.WithLabelValues("cache_expired_lock", "true").Inc()
		return nil, false
	}

	// Check if the lock itself is expired
	lock := item.value.(*proto.LockHandle)
	if lock.ExpiresAt.AsTime().Before(time.Now()) {
		c.locks.Remove(resourcePath)
		c.metrics.StorageOperations.WithLabelValues("cache_expired_lock", "true").Inc()
		return nil, false
	}

	c.metrics.StorageOperations.WithLabelValues("cache_hit_lock", "true").Inc()
	return lock, true
}

// SetLock adds a lock to the cache
func (c *Cache) SetLock(lock *proto.LockHandle) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate the cache expiration - use the lock's expiration if it's sooner
	expiration := time.Now().Add(c.expiration)
	if lock.ExpiresAt.AsTime().Before(expiration) {
		expiration = lock.ExpiresAt.AsTime()
	}

	c.locks.Add(lock.ResourcePath, cacheItem{
		value:      lock,
		expiration: expiration,
	})
	c.metrics.StorageOperations.WithLabelValues("cache_set_lock", "true").Inc()
}

// InvalidateWorkUnit removes a work unit from the cache
func (c *Cache) InvalidateWorkUnit(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.workUnits.Remove(id)
	c.metrics.StorageOperations.WithLabelValues("cache_invalidate_work_unit", "true").Inc()
}

// InvalidateContext removes a context from the cache
func (c *Cache) InvalidateContext(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.contexts.Remove(id)
	c.metrics.StorageOperations.WithLabelValues("cache_invalidate_context", "true").Inc()
}

// InvalidateLock removes a lock from the cache
func (c *Cache) InvalidateLock(resourcePath string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.locks.Remove(resourcePath)
	c.metrics.StorageOperations.WithLabelValues("cache_invalidate_lock", "true").Inc()
}

// Clear empties all caches
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.workUnits.Purge()
	c.contexts.Purge()
	c.locks.Purge()
	c.metrics.StorageOperations.WithLabelValues("cache_clear", "true").Inc()
}