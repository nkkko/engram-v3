package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	
	// Check some default values
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, "./data", cfg.Storage.DataDir)
	assert.Equal(t, 2, cfg.Storage.WALSyncIntervalMs)
	assert.Equal(t, 64, cfg.Storage.WALBatchSize)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoadConfigFromFile(t *testing.T) {
	// Write a test config file
	testConfig := `server:
  addr: ":9090"
storage:
  data_dir: "./test-data"
  wal_sync_interval_ms: 5
logging:
  level: "debug"
`
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	
	err := os.WriteFile(configFile, []byte(testConfig), 0644)
	require.NoError(t, err)
	
	// Load the config
	cfg, err := LoadConfigFromFile(configFile)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Check the loaded values
	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, "./test-data", cfg.Storage.DataDir)
	assert.Equal(t, 5, cfg.Storage.WALSyncIntervalMs)
	assert.Equal(t, "debug", cfg.Logging.Level)
	
	// Default values should be used for unspecified fields
	assert.Equal(t, 64, cfg.Storage.WALBatchSize)
}

func TestLoadConfig(t *testing.T) {
	// Write a test config file
	testConfig := `server:
  addr: ":9090"
storage:
  data_dir: "./test-data"
`
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	
	err := os.WriteFile(configFile, []byte(testConfig), 0644)
	require.NoError(t, err)
	
	// Override with environment variables and command-line flags
	t.Setenv("ENGRAM_SERVER_ADDR", ":8888")
	
	cfg, err := LoadConfig(configFile, "./cli-data", "", "warn")
	require.NoError(t, err)
	
	// Command-line flags should take precedence over env vars and file
	absPath, _ := filepath.Abs("./cli-data")
	assert.Equal(t, absPath, cfg.Storage.DataDir)
	
	// Env vars should take precedence over file
	assert.Equal(t, ":8888", cfg.Server.Addr)
	
	// Command-line flag should take precedence
	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestToEngineConfig(t *testing.T) {
	cfg := DefaultConfig()
	engineCfg := cfg.ToEngineConfig()
	
	assert.Equal(t, cfg.Storage.DataDir, engineCfg.DataDir)
	assert.Equal(t, cfg.Server.Addr, engineCfg.ServerAddr)
	assert.Equal(t, cfg.Logging.Level, engineCfg.LogLevel)
	assert.Equal(t, cfg.Storage.WALSyncIntervalMs, engineCfg.WALSyncIntervalMs)
	assert.Equal(t, cfg.Storage.WALBatchSize, engineCfg.WALBatchSize)
	assert.Equal(t, cfg.Locks.DefaultTTL, engineCfg.DefaultLockTTLSeconds)
	assert.Equal(t, cfg.Notifier.MaxIdleTime, engineCfg.MaxIdleConnectionSeconds)
}

func TestComponentConfigs(t *testing.T) {
	cfg := DefaultConfig()
	
	// Test storage config conversion
	storageCfg := cfg.ToStorageConfig()
	assert.Equal(t, cfg.Storage.DataDir, storageCfg.DataDir)
	assert.Equal(t, cfg.Storage.WALBatchSize, storageCfg.WALBatchSize)
	
	// Test router config conversion
	routerCfg := cfg.ToRouterConfig()
	assert.Equal(t, cfg.Router.MaxBufferSize, routerCfg.MaxBufferSize)
	
	// Test lock manager config conversion
	lockCfg := cfg.ToLockManagerConfig()
	assert.Equal(t, int64(cfg.Locks.DefaultTTL), int64(lockCfg.DefaultTTL.Seconds()))
	
	// Test search config conversion
	searchCfg := cfg.ToSearchConfig()
	assert.Equal(t, cfg.Storage.DataDir, searchCfg.DataDir)
	
	// Test notifier config conversion
	notifierCfg := cfg.ToNotifierConfig()
	assert.Equal(t, int64(cfg.Notifier.MaxIdleTime), int64(notifierCfg.MaxIdleTime.Seconds()))
	
	// Test API config conversion
	apiCfg := cfg.ToAPIConfig()
	assert.Equal(t, cfg.Server.Addr, apiCfg.Addr)
}