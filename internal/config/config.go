package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Router    RouterConfig    `yaml:"router"`
	Notifier  NotifierConfig  `yaml:"notifier"`
	Locks     LocksConfig     `yaml:"locks"`
	Search    SearchConfig    `yaml:"search"`
	Auth      AuthConfig      `yaml:"auth"`
	Logging   LoggingConfig   `yaml:"logging"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Addr         string `yaml:"addr"`
	MaxBodySize  int    `yaml:"max_body_size"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
	IdleTimeout  int    `yaml:"idle_timeout"`
}

// StorageConfig contains storage engine settings
type StorageConfig struct {
	DataDir                string `yaml:"data_dir"`
	WALSyncIntervalMs      int    `yaml:"wal_sync_interval_ms"`
	WALBatchSize           int    `yaml:"wal_batch_size"`
	MaxWALSizeMB           int    `yaml:"max_wal_size_mb"`
	CacheEnabled           bool   `yaml:"cache_enabled"`
	WorkUnitCacheSize      int    `yaml:"work_unit_cache_size"`
	ContextCacheSize       int    `yaml:"context_cache_size"`
	LockCacheSize          int    `yaml:"lock_cache_size"`
	CacheExpirationSeconds int    `yaml:"cache_expiration_seconds"`
	
	// Storage implementation selection
	StorageType            string `yaml:"storage_type"`
	
	// Optimized storage settings
	OptimizedEnabled       bool   `yaml:"optimized_enabled"`
	MemTableSize           int    `yaml:"mem_table_size"`
	NumMemTables           int    `yaml:"num_mem_tables"`
	NumLevelZeroTables     int    `yaml:"num_level_zero_tables"`
	ValueLogFileSize       int    `yaml:"value_log_file_size"`
	NumCompactors          int    `yaml:"num_compactors"`
	GCIntervalMinutes      int    `yaml:"gc_interval_minutes"`
	IndexCacheMB           int    `yaml:"index_cache_mb"`
	BlockCacheMB           int    `yaml:"block_cache_mb"`
}

// RouterConfig contains event router settings
type RouterConfig struct {
	MaxBufferSize   int `yaml:"max_buffer_size"`
	CleanupInterval int `yaml:"cleanup_interval"`
}

// NotifierConfig contains real-time notification settings
type NotifierConfig struct {
	MaxIdleTime        int `yaml:"max_idle_time"`
	HeartbeatInterval  int `yaml:"heartbeat_interval"`
	MaxConnections     int `yaml:"max_connections"`
	BroadcastBufferSize int `yaml:"broadcast_buffer_size"`
	BroadcastFlushIntervalMs int `yaml:"broadcast_flush_interval_ms"`
}

// LocksConfig contains lock manager settings
type LocksConfig struct {
	DefaultTTL         int `yaml:"default_ttl"`
	CleanupInterval    int `yaml:"cleanup_interval"`
	MaxLocksPerAgent   int `yaml:"max_locks_per_agent"`
}

// SearchConfig contains search engine settings
type SearchConfig struct {
	Engine           string          `yaml:"engine"`
	UpdateIntervalMs int             `yaml:"update_interval_ms"`
	MaxBatchSize     int             `yaml:"max_batch_size"`
	DefaultLimit     int             `yaml:"default_limit"`
	MaxLimit         int             `yaml:"max_limit"`
	Vector           VectorSearchConfig `yaml:"vector"`
}

// VectorSearchConfig contains vector search settings
type VectorSearchConfig struct {
	Enabled        bool   `yaml:"enabled"`
	WeaviateURL    string `yaml:"weaviate_url"`
	WeaviateAPIKey string `yaml:"weaviate_api_key"`
	ClassName      string `yaml:"class_name"`
	EmbeddingModel string `yaml:"embedding_model"`
	EmbeddingURL   string `yaml:"embedding_url"`
	EmbeddingKey   string `yaml:"embedding_key"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Enabled              bool   `yaml:"enabled"`
	JWTSecret            string `yaml:"jwt_secret"`
	JWTExpirationMinutes int    `yaml:"jwt_expiration_minutes"`
	AllowAnonymous       bool   `yaml:"allow_anonymous"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level           string            `yaml:"level"`
	Format          string            `yaml:"format"`
	IncludeCaller   bool              `yaml:"include_caller"`
	IncludeTrace    bool              `yaml:"include_trace"`
	GlobalFields    map[string]string `yaml:"global_fields"`
}

// TelemetryConfig contains OpenTelemetry settings
type TelemetryConfig struct {
	Enabled       bool              `yaml:"enabled"`
	ServiceName   string            `yaml:"service_name"`
	Endpoint      string            `yaml:"endpoint"`
	SamplingRatio float64           `yaml:"sampling_ratio"`
	Attributes    map[string]string `yaml:"attributes"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	Interval int    `yaml:"interval"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled            bool `yaml:"enabled"`
	RPM                int  `yaml:"rpm"`
	RPS                int  `yaml:"rps"`
	WSMessagesPerSecond int  `yaml:"ws_messages_per_second"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:         ":8080",
			MaxBodySize:  1048576, // 1MB
			ReadTimeout:  5,
			WriteTimeout: 10,
			IdleTimeout:  120,
		},
		Storage: StorageConfig{
			DataDir:                "./data",
			WALSyncIntervalMs:      2,
			WALBatchSize:           64,
			MaxWALSizeMB:           512,
			CacheEnabled:           true,
			WorkUnitCacheSize:      10000,
			ContextCacheSize:       1000,
			LockCacheSize:          5000,
			CacheExpirationSeconds: 30,
			StorageType:            "badger",
			OptimizedEnabled:       false,
			MemTableSize:           64 * 1024 * 1024,      // 64MB
			NumMemTables:           5,
			NumLevelZeroTables:     10,
			ValueLogFileSize:       1024 * 1024 * 1024,    // 1GB
			NumCompactors:          4,
			GCIntervalMinutes:      10,
			IndexCacheMB:           100,
			BlockCacheMB:           256,
		},
		Router: RouterConfig{
			MaxBufferSize:   100,
			CleanupInterval: 30,
		},
		Notifier: NotifierConfig{
			MaxIdleTime:             30,
			HeartbeatInterval:       5,
			MaxConnections:          10000,
			BroadcastBufferSize:     200,
			BroadcastFlushIntervalMs: 50,
		},
		Locks: LocksConfig{
			DefaultTTL:       60,
			CleanupInterval:  60,
			MaxLocksPerAgent: 100,
		},
		Search: SearchConfig{
			Engine:           "badger",
			UpdateIntervalMs: 200,
			MaxBatchSize:     100,
			DefaultLimit:     100,
			MaxLimit:         1000,
			Vector: VectorSearchConfig{
				Enabled:        false,
				WeaviateURL:    "http://localhost:8080",
				ClassName:      "WorkUnit",
				EmbeddingModel: "openai/text-embedding-3-small",
			},
		},
		Auth: AuthConfig{
			Enabled:              false,
			JWTSecret:            "change-me-in-production",
			JWTExpirationMinutes: 60,
			AllowAnonymous:       false,
		},
		Logging: LoggingConfig{
			Level:         "info",
			Format:        "json",
			IncludeCaller: true,
			IncludeTrace:  true,
			GlobalFields:  map[string]string{},
		},
		Telemetry: TelemetryConfig{
			Enabled:       false,
			ServiceName:   "engram",
			Endpoint:      "localhost:4317",
			SamplingRatio: 0.1,
			Attributes:    map[string]string{},
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Endpoint: "/metrics",
			Interval: 15,
		},
		RateLimit: RateLimitConfig{
			Enabled:            false,
			RPM:                600,
			RPS:                10,
			WSMessagesPerSecond: 20,
		},
	}
}

// LoadConfigFromFile loads configuration from a YAML file
func LoadConfigFromFile(filePath string) (*Config, error) {
	// Start with default configuration
	config := DefaultConfig()

	// Read and parse configuration file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn().Str("file", filePath).Msg("Configuration file not found, using defaults")
			return config, nil
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return config, nil
}

// LoadConfig loads configuration from file, environment variables, and flags
func LoadConfig(configFile string, dataDir string, serverAddr string, logLevel string) (*Config, error) {
	var config *Config
	var err error

	// Load from file if specified
	if configFile != "" {
		config, err = LoadConfigFromFile(configFile)
		if err != nil {
			return nil, err
		}
	} else {
		// Use default config
		config = DefaultConfig()
	}

	// Override with environment variables
	applyEnvOverrides(config)

	// Override with command line flags (highest priority)
	if dataDir != "" {
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for data directory: %w", err)
		}
		config.Storage.DataDir = absDataDir
	}

	if serverAddr != "" {
		config.Server.Addr = serverAddr
	}

	if logLevel != "" {
		config.Logging.Level = logLevel
	}

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(config *Config) {
	// Server config overrides
	if addr := os.Getenv("ENGRAM_SERVER_ADDR"); addr != "" {
		config.Server.Addr = addr
	}

	// Storage config overrides
	if dataDir := os.Getenv("ENGRAM_STORAGE_DATA_DIR"); dataDir != "" {
		config.Storage.DataDir = dataDir
	}
	if syncIntervalStr := os.Getenv("ENGRAM_STORAGE_WAL_SYNC_INTERVAL_MS"); syncIntervalStr != "" {
		if val, err := strconv.Atoi(syncIntervalStr); err == nil {
			config.Storage.WALSyncIntervalMs = val
		}
	}
	if batchSizeStr := os.Getenv("ENGRAM_STORAGE_WAL_BATCH_SIZE"); batchSizeStr != "" {
		if val, err := strconv.Atoi(batchSizeStr); err == nil {
			config.Storage.WALBatchSize = val
		}
	}

	// Logging config overrides
	if level := os.Getenv("ENGRAM_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("ENGRAM_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}

	// Add more environment variable overrides as needed...
}

// ToEngineConfig converts the central config to an engine config
func (c *Config) ToEngineConfig() EngramEngineConfig {
	return EngramEngineConfig{
		DataDir:                 c.Storage.DataDir,
		ServerAddr:              c.Server.Addr,
		LogLevel:                c.Logging.Level,
		WALSyncIntervalMs:       c.Storage.WALSyncIntervalMs,
		WALBatchSize:            c.Storage.WALBatchSize,
		DefaultLockTTLSeconds:   c.Locks.DefaultTTL,
		MaxIdleConnectionSeconds: c.Notifier.MaxIdleTime,
	}
}

// EngramEngineConfig represents the engine configuration
// This is maintained for compatibility with the current engine implementation
type EngramEngineConfig struct {
	// Data directory for storage
	DataDir string

	// HTTP server address
	ServerAddr string

	// Log level
	LogLevel string

	// WAL sync interval in milliseconds
	WALSyncIntervalMs int

	// WAL batch size (number of work units)
	WALBatchSize int

	// Lock TTL in seconds
	DefaultLockTTLSeconds int

	// Maximum idle connection time in seconds
	MaxIdleConnectionSeconds int
}