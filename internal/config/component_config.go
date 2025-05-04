package config

import (
	"time"

	"github.com/nkkko/engram-v3/internal/api"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage/badger"
)

// ToStorageConfig converts to badger storage config
func (c *Config) ToStorageConfig() badger.Config {
	return badger.Config{
		DataDir:           c.Storage.DataDir,
		WALSyncInterval:   time.Duration(c.Storage.WALSyncIntervalMs) * time.Millisecond,
		WALBatchSize:      c.Storage.WALBatchSize,
		CacheEnabled:      c.Storage.CacheEnabled,
		WorkUnitCacheSize: c.Storage.WorkUnitCacheSize,
		ContextCacheSize:  c.Storage.ContextCacheSize,
		LockCacheSize:     c.Storage.LockCacheSize,
		CacheExpiration:   time.Duration(c.Storage.CacheExpirationSeconds) * time.Second,
	}
}

// ToRouterConfig converts to router config
func (c *Config) ToRouterConfig() router.Config {
	return router.Config{
		MaxBufferSize: c.Router.MaxBufferSize,
	}
}

// ToLockManagerConfig converts to lock manager config
func (c *Config) ToLockManagerConfig() lockmanager.Config {
	return lockmanager.Config{
		DefaultTTL:      time.Duration(c.Locks.DefaultTTL) * time.Second,
		CleanupInterval: time.Duration(c.Locks.CleanupInterval) * time.Second,
	}
}

// ToSearchConfig converts to search config
func (c *Config) ToSearchConfig() search.Config {
	return search.Config{
		DataDir: c.Storage.DataDir,
	}
}

// ToNotifierConfig converts to notifier config
func (c *Config) ToNotifierConfig() notifier.Config {
	return notifier.Config{
		MaxIdleTime:            time.Duration(c.Notifier.MaxIdleTime) * time.Second,
		BroadcastBufferSize:    c.Notifier.BroadcastBufferSize,
		BroadcastFlushInterval: time.Duration(c.Notifier.BroadcastFlushIntervalMs) * time.Millisecond,
	}
}

// ToAPIConfig converts to API config
func (c *Config) ToAPIConfig() api.Config {
	return api.Config{
		Addr: c.Server.Addr,
	}
}