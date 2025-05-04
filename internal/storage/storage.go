package storage

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// Storage interface defines the common interface for storage implementations
type Storage interface {
	// Start begins the storage engine operation
	Start(ctx context.Context) error
	
	// Shutdown stops the storage engine
	Shutdown(ctx context.Context) error
	
	// EventStream returns a channel of work unit events
	EventStream() <-chan *proto.WorkUnit
	
	// CreateWorkUnit creates a new work unit
	CreateWorkUnit(ctx context.Context, req *proto.CreateWorkUnitRequest) (*proto.WorkUnit, error)
	
	// GetWorkUnit retrieves a work unit by ID
	GetWorkUnit(ctx context.Context, id string) (*proto.WorkUnit, error)
	
	// ListWorkUnits retrieves work units for a context
	ListWorkUnits(ctx context.Context, contextID string, limit int, cursor string, reverse bool) ([]*proto.WorkUnit, string, error)
	
	// UpdateContext updates or creates a context
	UpdateContext(ctx context.Context, req *proto.UpdateContextRequest) (*proto.Context, error)
	
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, id string) (*proto.Context, error)
	
	// AcquireLock attempts to acquire a lock on a resource
	AcquireLock(ctx context.Context, req *proto.AcquireLockRequest) (*proto.LockHandle, error)
	
	// GetLock retrieves a lock by resource path
	GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error)
	
	// ReleaseLock releases a lock on a resource
	ReleaseLock(ctx context.Context, req *proto.ReleaseLockRequest) (bool, error)
}

// WALEntry represents a single write-ahead log entry
type WALEntry struct {
	Type      WALEntryType
	Timestamp time.Time
	Data      []byte
}

// WALEntryType defines the type of WAL entry
type WALEntryType uint8

const (
	WALWorkUnit WALEntryType = iota + 1
	WALContext
	WALLock
)

// Config contains storage configuration
type Config struct {
	// Base directory for data files
	DataDir string

	// WAL sync interval
	WALSyncInterval time.Duration

	// WAL batch size
	WALBatchSize int
	
	// Cache settings
	CacheEnabled bool
	WorkUnitCacheSize int
	ContextCacheSize int
	LockCacheSize int
	CacheExpiration time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		DataDir:               "./data",
		WALSyncInterval:       2 * time.Millisecond,
		WALBatchSize:          64,
		CacheEnabled:          true,
		WorkUnitCacheSize:     10000,
		ContextCacheSize:      1000,
		LockCacheSize:         5000,
		CacheExpiration:       30 * time.Second,
	}
}

// makeContextTimeKey creates a composite key for context-timestamp indexing
func makeContextTimeKey(contextID string, ts time.Time) []byte {
	key := make([]byte, len(contextID)+8)
	copy(key, contextID)
	binary.BigEndian.PutUint64(key[len(contextID):], uint64(ts.UnixNano()))
	return key
}

// getDirSize returns the size of a directory and its subdirectories in bytes
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}