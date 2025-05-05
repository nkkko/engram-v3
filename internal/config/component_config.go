package config

import (
	"time"

	"github.com/nkkko/engram-v3/internal/api"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/logging"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/router"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/internal/storage/badger"
	"github.com/nkkko/engram-v3/internal/telemetry"
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

// ToStorageFactoryConfig converts to storage factory config
func (c *Config) ToStorageFactoryConfig() storage.FactoryConfig {
	// Create basic config
	basicConfig := storage.Config{
		DataDir:          c.Storage.DataDir,
		WALSyncInterval:  time.Duration(c.Storage.WALSyncIntervalMs) * time.Millisecond,
		WALBatchSize:     c.Storage.WALBatchSize,
		CacheEnabled:     c.Storage.CacheEnabled,
		WorkUnitCacheSize: c.Storage.WorkUnitCacheSize,
		ContextCacheSize: c.Storage.ContextCacheSize,
		LockCacheSize:    c.Storage.LockCacheSize,
		CacheExpiration:  time.Duration(c.Storage.CacheExpirationSeconds) * time.Second,
	}
	
	// Create optimized config
	optimizedConfig := badger.OptimizedConfig{
		DataDir:                c.Storage.DataDir,
		WALSyncInterval:        time.Duration(c.Storage.WALSyncIntervalMs) * time.Millisecond,
		WALBatchSize:           c.Storage.WALBatchSize,
		WALMaxSizeMB:           c.Storage.MaxWALSizeMB,
		CacheEnabled:           c.Storage.CacheEnabled,
		WorkUnitCacheSize:      c.Storage.WorkUnitCacheSize,
		ContextCacheSize:       c.Storage.ContextCacheSize,
		LockCacheSize:          c.Storage.LockCacheSize,
		CacheExpiration:        time.Duration(c.Storage.CacheExpirationSeconds) * time.Second,
		
		// Advanced settings
		MemTableSize:           int64(c.Storage.MemTableSize),
		NumMemtables:           c.Storage.NumMemTables,
		NumLevelZeroTables:     c.Storage.NumLevelZeroTables,
		ValueLogFileSize:       int64(c.Storage.ValueLogFileSize),
		NumCompactors:          c.Storage.NumCompactors,
		GCInterval:             time.Duration(c.Storage.GCIntervalMinutes) * time.Minute,
		IndexCacheSize:         int64(c.Storage.IndexCacheMB) * 1024 * 1024,  // Convert MB to bytes
		BlockCacheSize:         int64(c.Storage.BlockCacheMB) * 1024 * 1024,  // Convert MB to bytes
	}
	
	// Determine storage type
	storageType := storage.BadgerStorage
	if c.Storage.OptimizedEnabled {
		storageType = storage.OptimizedBadgerStorage
	} else if c.Storage.StorageType == "badger-optimized" {
		storageType = storage.OptimizedBadgerStorage
	}
	
	return storage.FactoryConfig{
		Type:            storageType,
		Config:          basicConfig,
		OptimizedConfig: optimizedConfig,
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

// ToVectorSearchConfig converts to vector search config
func (c *Config) ToVectorSearchConfig() search.VectorConfig {
	return search.VectorConfig{
		Enabled:        c.Search.Vector.Enabled,
		WeaviateURL:    c.Search.Vector.WeaviateURL,
		WeaviateAPIKey: c.Search.Vector.WeaviateAPIKey,
		ClassName:      c.Search.Vector.ClassName,
		BatchSize:      c.Search.MaxBatchSize,
		EmbeddingModel: c.Search.Vector.EmbeddingModel,
		EmbeddingURL:   c.Search.Vector.EmbeddingURL,
		EmbeddingKey:   c.Search.Vector.EmbeddingKey,
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

// ToLoggingConfig converts to logging config
func (c *Config) ToLoggingConfig() logging.Config {
	var level logging.LogLevel
	switch c.Logging.Level {
	case "debug":
		level = logging.LevelDebug
	case "info":
		level = logging.LevelInfo
	case "warn":
		level = logging.LevelWarn
	case "error":
		level = logging.LevelError
	default:
		level = logging.LevelInfo
	}

	var format logging.LogFormat
	switch c.Logging.Format {
	case "json":
		format = logging.FormatJSON
	case "console":
		format = logging.FormatConsole
	default:
		format = logging.FormatJSON
	}

	return logging.Config{
		Level:              level,
		Format:             format,
		IncludeCaller:      c.Logging.IncludeCaller,
		IncludeStacktrace:  true,
		IncludeTraceContext: c.Logging.IncludeTrace,
		GlobalFields:       c.Logging.GlobalFields,
	}
}

// ToTelemetryConfig converts to telemetry config
func (c *Config) ToTelemetryConfig() telemetry.Config {
	return telemetry.Config{
		Enabled:       c.Telemetry.Enabled,
		ServiceName:   c.Telemetry.ServiceName,
		Endpoint:      c.Telemetry.Endpoint,
		SamplingRatio: c.Telemetry.SamplingRatio,
		Timeout:       5 * time.Second,
		Attributes:    c.Telemetry.Attributes,
	}
}