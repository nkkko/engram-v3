package search

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewSearch(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "search-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create search with the test directory
	config := Config{
		DataDir: tempDir,
	}
	search, err := NewSearch(config)
	require.NoError(t, err, "Failed to create search")
	defer search.Shutdown(context.Background())

	// Verify search instance
	assert.NotNil(t, search.db, "Database should be initialized")
	assert.Equal(t, config, search.config, "Config should be set correctly")
	assert.NotNil(t, search.indexQueue, "Index queue should be initialized")
}

func TestIndexAndSearch(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "search-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create search with the test directory
	config := Config{
		DataDir: tempDir,
	}
	search, err := NewSearch(config)
	require.NoError(t, err, "Failed to create search")
	defer search.Shutdown(context.Background())

	// Create test work units
	now := time.Now()
	units := []*proto.WorkUnit{
		{
			Id:        "test-unit-1",
			ContextId: "context-1",
			AgentId:   "agent-1",
			Type:      proto.WorkUnitType_MESSAGE,
			Ts:        timestamppb.New(now),
			Payload:   []byte("This is a test message with keyword apple"),
			Meta: map[string]string{
				"category": "fruit",
				"priority": "high",
			},
		},
		{
			Id:        "test-unit-2",
			ContextId: "context-1",
			AgentId:   "agent-2",
			Type:      proto.WorkUnitType_SYSTEM,
			Ts:        timestamppb.New(now.Add(1 * time.Minute)),
			Payload:   []byte("Another message with keyword banana"),
			Meta: map[string]string{
				"category": "fruit",
				"priority": "medium",
			},
		},
		{
			Id:        "test-unit-3",
			ContextId: "context-2",
			AgentId:   "agent-1",
			Type:      proto.WorkUnitType_SYSTEM,
			Ts:        timestamppb.New(now.Add(2 * time.Minute)),
			Payload:   []byte("System message about carrot"),
			Meta: map[string]string{
				"category": "vegetable",
				"priority": "low",
			},
		},
	}

	// Index the work units
	for _, unit := range units {
		search.indexBatch([]*proto.WorkUnit{unit})
	}

	// Test cases
	testCases := []struct {
		name         string
		query        string
		contextID    string
		agentIDs     []string
		types        []proto.WorkUnitType
		metaFilters  map[string]string
		limit        int
		offset       int
		expectedIDs  []string
		expectedCnt  int
		expectError  bool
	}{
		{
			name:        "Search by keyword apple",
			query:       "apple",
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-1"},
			expectedCnt: 1,
		},
		{
			name:        "Search by context",
			query:       "",
			contextID:   "context-1",
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-2", "test-unit-1"},
			expectedCnt: 2,
		},
		{
			name:        "Search by agent",
			query:       "",
			agentIDs:    []string{"agent-1"},
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-3", "test-unit-1"},
			expectedCnt: 2,
		},
		{
			name:        "Search by type",
			query:       "",
			types:       []proto.WorkUnitType{proto.WorkUnitType_SYSTEM},
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-2"},
			expectedCnt: 1,
		},
		{
			name:        "Search by metadata",
			query:       "",
			metaFilters: map[string]string{"category": "vegetable"},
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-3"},
			expectedCnt: 1,
		},
		{
			name:        "Combined search",
			query:       "fruit",
			contextID:   "context-1",
			agentIDs:    []string{"agent-2"},
			limit:       10,
			offset:      0,
			expectedIDs: []string{"test-unit-2"},
			expectedCnt: 1,
		},
		{
			name:        "Test pagination",
			query:       "",
			limit:       1,
			offset:      1,
			expectedIDs: []string{"test-unit-2"},
			expectedCnt: 3, // Total count should be 3
		},
		{
			name:        "No results",
			query:       "nonexistent",
			limit:       10,
			offset:      0,
			expectedIDs: []string{},
			expectedCnt: 0,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			results, count, err := search.Search(ctx, tc.query, tc.contextID, tc.agentIDs, tc.types, tc.metaFilters, tc.limit, tc.offset)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedCnt, count, "Total count should match")
				assert.Equal(t, len(tc.expectedIDs), len(results), "Number of results should match")
				
				for i, expectedID := range tc.expectedIDs {
					if i < len(results) {
						assert.Equal(t, expectedID, results[i], "Result ID should match")
					}
				}
			}
		})
	}
}

func TestIsTextPayload(t *testing.T) {
	testCases := []struct {
		name     string
		payload  []byte
		expected bool
	}{
		{
			name:     "Text payload",
			payload:  []byte("This is a text payload"),
			expected: true,
		},
		{
			name:     "Text with newlines",
			payload:  []byte("Text with\nnewlines\r\nand carriage returns"),
			expected: true,
		},
		{
			name:     "Text with tabs",
			payload:  []byte("Text with\ttabs"),
			expected: true,
		},
		{
			name:     "Binary payload",
			payload:  []byte{0x00, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "Mixed payload",
			payload:  []byte("Text with binary\x00data"),
			expected: false,
		},
		{
			name:     "Empty payload",
			payload:  []byte{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTextPayload(tc.payload)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSearchWithEvents(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "search-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create search with the test directory
	config := Config{
		DataDir: tempDir,
	}
	search, err := NewSearch(config)
	require.NoError(t, err, "Failed to create search")
	defer search.Shutdown(context.Background())

	// Create a channel for events
	eventCh := make(chan *proto.WorkUnit, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the search indexer
	err = search.Start(ctx, eventCh)
	require.NoError(t, err, "Failed to start search")

	// Create test work units
	now := time.Now()
	units := []*proto.WorkUnit{
		{
			Id:        "test-event-1",
			ContextId: "context-event",
			AgentId:   "agent-1",
			Type:      proto.WorkUnitType_MESSAGE,
			Ts:        timestamppb.New(now),
			Payload:   []byte("Event message with keyword elephant"),
			Meta: map[string]string{
				"category": "animal",
			},
		},
		{
			Id:        "test-event-2",
			ContextId: "context-event",
			AgentId:   "agent-2",
			Type:      proto.WorkUnitType_MESSAGE,
			Ts:        timestamppb.New(now.Add(1 * time.Minute)),
			Payload:   []byte("Event message with keyword giraffe"),
			Meta: map[string]string{
				"category": "animal",
			},
		},
	}

	// Send events to the channel
	for _, unit := range units {
		eventCh <- unit
	}

	// Wait for indexing to complete
	time.Sleep(500 * time.Millisecond)

	// Test search with events
	results, count, err := search.Search(ctx, "elephant", "", nil, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []string{"test-event-1"}, results)

	results, count, err = search.Search(ctx, "giraffe", "", nil, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []string{"test-event-2"}, results)

	results, count, err = search.Search(ctx, "", "context-event", nil, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, results, 2)
}

// BenchmarkSearch benchmarks search performance
func BenchmarkSearch(b *testing.B) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "search-benchmark-*")
	require.NoError(b, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create search with the test directory
	config := Config{
		DataDir: tempDir,
	}
	search, err := NewSearch(config)
	require.NoError(b, err, "Failed to create search")
	defer search.Shutdown(context.Background())

	// Create test work units
	const numUnits = 1000
	now := time.Now()
	units := make([]*proto.WorkUnit, numUnits)
	
	keywords := []string{"apple", "banana", "carrot", "date", "eggplant"}
	
	for i := 0; i < numUnits; i++ {
		keyword := keywords[i%len(keywords)]
		units[i] = &proto.WorkUnit{
			Id:        fmt.Sprintf("bench-unit-%d", i),
			ContextId: fmt.Sprintf("context-%d", i%10),
			AgentId:   fmt.Sprintf("agent-%d", i%5),
			Type:      proto.WorkUnitType(i % 3),
			Ts:        timestamppb.New(now.Add(time.Duration(i) * time.Second)),
			Payload:   []byte(fmt.Sprintf("This is benchmark message %d with keyword %s", i, keyword)),
			Meta: map[string]string{
				"category": keyword,
				"priority": fmt.Sprintf("%d", i%3),
			},
		}
	}

	// Index the work units
	for _, unit := range units {
		search.indexBatch([]*proto.WorkUnit{unit})
	}

	// Benchmark keyword search
	b.Run("SearchByKeyword", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			keyword := keywords[i%len(keywords)]
			_, _, err := search.Search(ctx, keyword, "", nil, nil, nil, 10, 0)
			require.NoError(b, err)
		}
	})

	// Benchmark context search
	b.Run("SearchByContext", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			contextID := fmt.Sprintf("context-%d", i%10)
			_, _, err := search.Search(ctx, "", contextID, nil, nil, nil, 10, 0)
			require.NoError(b, err)
		}
	})

	// Benchmark metadata search
	b.Run("SearchByMetadata", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			keyword := keywords[i%len(keywords)]
			metaFilters := map[string]string{"category": keyword}
			_, _, err := search.Search(ctx, "", "", nil, nil, metaFilters, 10, 0)
			require.NoError(b, err)
		}
	})

	// Benchmark complex search
	b.Run("ComplexSearch", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			keyword := keywords[i%len(keywords)]
			contextID := fmt.Sprintf("context-%d", i%10)
			agentIDs := []string{fmt.Sprintf("agent-%d", i%5)}
			types := []proto.WorkUnitType{proto.WorkUnitType(i % 3)}
			metaFilters := map[string]string{"priority": fmt.Sprintf("%d", i%3)}
			
			_, _, err := search.Search(ctx, keyword, contextID, agentIDs, types, metaFilters, 10, 0)
			require.NoError(b, err)
		}
	})
}