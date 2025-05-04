package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BenchmarkWALBatching compares different WAL batch sizes
func BenchmarkWALBatching(b *testing.B) {
	// Test different batch sizes
	batchSizes := []int{1, 8, 16, 32, 64, 128, 256}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			// Create temporary directory for test data
			tempDir, err := os.MkdirTemp("", "storage-bench-*")
			require.NoError(b, err, "Failed to create temp directory")
			defer os.RemoveAll(tempDir)
			
			// Create storage with specified batch size
			config := Config{
				DataDir:           tempDir,
				WALSyncInterval:   100 * time.Millisecond,
				WALBatchSize:      batchSize,
				CacheEnabled:      false, // Disable cache for WAL testing
				WorkUnitCacheSize: 100,
				ContextCacheSize:  100,
				LockCacheSize:     100,
				CacheExpiration:   30 * time.Second,
			}
			
			storage, err := NewStorage(config)
			require.NoError(b, err, "Failed to create storage")
			
			// Start storage
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			go func() {
				err := storage.Start(ctx)
				if err != nil && err != context.Canceled {
					b.Errorf("Storage error: %v", err)
				}
			}()
			
			// Wait for storage to initialize
			time.Sleep(100 * time.Millisecond)
			
			// Reset timer before benchmark
			b.ResetTimer()
			
			// Run benchmark
			for i := 0; i < b.N; i++ {
				// Create work unit for writing to WAL
				workUnit := createBenchmarkWorkUnit(i, "context-1")
				
				// Send work unit to WAL - We need to access this through storage interface
				// Since we don't have direct access to walEntries in our interface, we'll use
				// the CreateWorkUnit method as a proxy
				req := &proto.CreateWorkUnitRequest{
					ContextId: workUnit.ContextId,
					AgentId:   workUnit.AgentId,
					Type:      workUnit.Type,
					Meta:      workUnit.Meta,
					Payload:   workUnit.Payload,
				}

				_, err := storage.CreateWorkUnit(ctx, req)
				require.NoError(b, err)
			}
			
			// Clean up
			b.StopTimer()
			err = storage.Shutdown(ctx)
			require.NoError(b, err)
		})
	}
}

// BenchmarkCaching compares operations with and without caching
func BenchmarkCaching(b *testing.B) {
	// Create a common test context for all benchmarks
	ctx := context.Background()
	contextID := "benchmark-context"
	
	// Function to set up storage with cache enabled or disabled
	setupStorage := func(cacheEnabled bool) (Storage, string, func()) {
		// Create temporary directory for test data
		tempDir, err := os.MkdirTemp("", "cache-bench-*")
		require.NoError(b, err, "Failed to create temp directory")
		
		// Create storage with or without cache
		config := Config{
			DataDir:           tempDir,
			WALSyncInterval:   50 * time.Millisecond,
			WALBatchSize:      64,
			CacheEnabled:      cacheEnabled,
			WorkUnitCacheSize: 10000,
			ContextCacheSize:  1000,
			LockCacheSize:     1000,
			CacheExpiration:   30 * time.Second,
		}
		
		storage, err := NewStorage(config)
		require.NoError(b, err, "Failed to create storage")
		
		// Start storage
		ctx, cancel := context.WithCancel(context.Background())
		
		go func() {
			err := storage.Start(ctx)
			if err != nil && err != context.Canceled {
				b.Errorf("Storage error: %v", err)
			}
		}()
		
		// Wait for storage to initialize
		time.Sleep(100 * time.Millisecond)
		
		// Create a cleanup function
		cleanup := func() {
			cancel()
			storage.Shutdown(context.Background())
			os.RemoveAll(tempDir)
		}
		
		return storage, tempDir, cleanup
	}
	
	// Benchmark create and get work unit with cache disabled
	b.Run("CreateGetWorkUnit_NoCache", func(b *testing.B) {
		storage, _, cleanup := setupStorage(false)
		defer cleanup()
		
		// Pre-create work units for retrieval
		workUnitIDs := make([]string, b.N)
		for i := 0; i < b.N; i++ {
			req := &proto.CreateWorkUnitRequest{
				ContextId: contextID,
				AgentId:   "benchmark-agent",
				Type:      proto.WorkUnitType_MESSAGE,
				Payload:   []byte("Benchmark test payload"),
			}
			
			workUnit, err := storage.CreateWorkUnit(ctx, req)
			require.NoError(b, err)
			workUnitIDs[i] = workUnit.Id
		}
		
		// Reset timer before benchmark
		b.ResetTimer()
		
		// Benchmark retrieving work units
		for i := 0; i < b.N; i++ {
			_, err := storage.GetWorkUnit(ctx, workUnitIDs[i%len(workUnitIDs)])
			require.NoError(b, err)
		}
	})
	
	// Benchmark create and get work unit with cache enabled
	b.Run("CreateGetWorkUnit_WithCache", func(b *testing.B) {
		storage, _, cleanup := setupStorage(true)
		defer cleanup()
		
		// Pre-create work units for retrieval
		workUnitIDs := make([]string, b.N)
		for i := 0; i < b.N; i++ {
			req := &proto.CreateWorkUnitRequest{
				ContextId: contextID,
				AgentId:   "benchmark-agent",
				Type:      proto.WorkUnitType_MESSAGE,
				Payload:   []byte("Benchmark test payload"),
			}
			
			workUnit, err := storage.CreateWorkUnit(ctx, req)
			require.NoError(b, err)
			workUnitIDs[i] = workUnit.Id
		}
		
		// Reset timer before benchmark
		b.ResetTimer()
		
		// Benchmark retrieving work units (should use cache)
		for i := 0; i < b.N; i++ {
			_, err := storage.GetWorkUnit(ctx, workUnitIDs[i%len(workUnitIDs)])
			require.NoError(b, err)
		}
	})
	
	// Benchmark with repeated reads (to clearly show cache benefit)
	b.Run("RepeatedReads_NoCache", func(b *testing.B) {
		storage, _, cleanup := setupStorage(false)
		defer cleanup()
		
		// Create a single work unit to retrieve repeatedly
		req := &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "benchmark-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte("Benchmark test payload for repeated reads"),
		}
		
		workUnit, err := storage.CreateWorkUnit(ctx, req)
		require.NoError(b, err)
		
		// Reset timer before benchmark
		b.ResetTimer()
		
		// Repeatedly retrieve the same work unit
		for i := 0; i < b.N; i++ {
			_, err := storage.GetWorkUnit(ctx, workUnit.Id)
			require.NoError(b, err)
		}
	})
	
	b.Run("RepeatedReads_WithCache", func(b *testing.B) {
		storage, _, cleanup := setupStorage(true)
		defer cleanup()
		
		// Create a single work unit to retrieve repeatedly
		req := &proto.CreateWorkUnitRequest{
			ContextId: contextID,
			AgentId:   "benchmark-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Payload:   []byte("Benchmark test payload for repeated reads"),
		}
		
		workUnit, err := storage.CreateWorkUnit(ctx, req)
		require.NoError(b, err)
		
		// Do one initial read to populate the cache
		_, err = storage.GetWorkUnit(ctx, workUnit.Id)
		require.NoError(b, err)
		
		// Reset timer before benchmark
		b.ResetTimer()
		
		// Repeatedly retrieve the same work unit (should use cache)
		for i := 0; i < b.N; i++ {
			_, err := storage.GetWorkUnit(ctx, workUnit.Id)
			require.NoError(b, err)
		}
	})
}

// createBenchmarkWorkUnit creates a work unit for benchmarking
func createBenchmarkWorkUnit(id int, contextID string) *proto.WorkUnit {
	return &proto.WorkUnit{
		Id:        fmt.Sprintf("bench-unit-%d", id),
		ContextId: contextID,
		AgentId:   "benchmark-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Ts:        timestamppb.Now(),
		Payload:   []byte(fmt.Sprintf("Benchmark payload for work unit %d", id)),
		Meta: map[string]string{
			"benchmark": "true",
			"index":     fmt.Sprintf("%d", id),
		},
	}
}