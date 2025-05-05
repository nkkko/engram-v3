package badger

import (
	"time"
)

// OptimizedConfig contains enhanced storage configuration with optimized settings
type OptimizedConfig struct {
	// Basic settings
	DataDir string

	// WAL settings
	WALSyncInterval          time.Duration
	WALBatchSize             int
	WALMaxSizeMB             int
	WALCompactThresholdRatio float64

	// Badger settings
	MemTableSize     int64  // Size of each memtable in bytes
	NumMemtables     int    // Number of memtables to keep in memory
	NumLevelZeroTables int  // Number of level 0 tables before compaction
	NumLevelZeroTablesStall int // Number of level 0 tables that trigger a stall
	BaseLevelSize     int64   // Base level size in bytes
	ValueLogFileSize  int64   // Value log file size in bytes
	ValueLogMaxEntries uint   // Maximum number of entries in value log
	
	// GC settings
	GCInterval         time.Duration // How often to run garbage collection
	GCDiscardRatio     float64       // Discard ratio for GC (0.5 means discard if 50% is garbage)
	GCMinValueLogSize  int64         // Minimum value log size to run GC

	// Compaction settings
	CompactionInterval time.Duration // How often to check for compaction
	
	// Performance tweaks
	SyncWrites             bool   // Whether to sync writes (false for better performance, true for durability)
	NumCompactors          int    // Number of compaction goroutines
	VerifyValueChecksum    bool   // Whether to verify checksums on reads
	NumVersionsToKeep      int    // Number of old versions to keep per key
	DetectConflicts        bool   // Whether to detect conflicts in transactions
	IndexCacheSize         int64  // Size of block cache for index blocks
	BlockCacheSize         int64  // Size of block cache for data blocks
	
	// Cache settings
	CacheEnabled        bool
	WorkUnitCacheSize   int
	ContextCacheSize    int
	LockCacheSize       int
	CacheExpiration     time.Duration
}

// OptimizedDefaultConfig returns optimized configuration for Badger
func OptimizedDefaultConfig() OptimizedConfig {
	return OptimizedConfig{
		// Basic settings
		DataDir: "./data",

		// WAL settings
		WALSyncInterval:          5 * time.Millisecond,   // Slightly increased from 2ms for better batching
		WALBatchSize:             128,                    // Doubled from 64 for better throughput
		WALMaxSizeMB:             512,                    // Max WAL size before forcing rotation
		WALCompactThresholdRatio: 0.5,                    // Compact WAL when 50% space can be reclaimed

		// Badger settings - optimized for performance
		MemTableSize:             64 << 20,              // 64MB (default is 64MB)
		NumMemtables:             5,                     // Default is 5
		NumLevelZeroTables:       10,                    // Default is 5, increased for write-heavy workloads
		NumLevelZeroTablesStall:  15,                    // Default is 10, increased for write-heavy workloads
		BaseLevelSize:            10 << 20,              // 10MB (default is 10MB)
		ValueLogFileSize:         1 << 30,               // 1GB (default is 1GB)
		ValueLogMaxEntries:       1000000,               // Default is 1000000
		
		// GC settings
		GCInterval:               10 * time.Minute,      // Run GC every 10 minutes
		GCDiscardRatio:           0.5,                   // Discard if 50% is garbage (default)
		GCMinValueLogSize:        1 << 20,               // 1MB minimum size to consider for GC
		
		// Compaction settings
		CompactionInterval:       5 * time.Minute,       // Check for compaction every 5 minutes
		
		// Performance tweaks
		SyncWrites:               false,                 // We handle syncing via our WAL
		NumCompactors:            4,                     // Number of compaction goroutines (default is 2)
		VerifyValueChecksum:      false,                 // Skip checksums for better performance
		NumVersionsToKeep:        1,                     // Only keep one version per key
		DetectConflicts:          true,                  // Keep conflict detection for safety
		IndexCacheSize:           100 << 20,             // 100MB for index cache
		BlockCacheSize:           256 << 20,             // 256MB for block cache
		
		// Cache settings
		CacheEnabled:             true,
		WorkUnitCacheSize:        20000,                 // Doubled from 10000
		ContextCacheSize:         2000,                  // Doubled from 1000
		LockCacheSize:            10000,                 // Doubled from 5000
		CacheExpiration:          2 * time.Minute,       // Increased from 30s
	}
}