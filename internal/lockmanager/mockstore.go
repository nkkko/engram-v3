package lockmanager

import (
	"context"
	"sync"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// MockStorage is a simple mock implementation of the Storage interface
// for testing the lock manager
type MockStorage struct {
	mu            sync.Mutex
	locks         map[string]*proto.LockHandle
	acquireCalled int
	releaseCalled int
	failAcquire   bool
	failRelease   bool
}

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		locks: make(map[string]*proto.LockHandle),
	}
}

// SetFailAcquire configures the mock to fail acquire operations
func (m *MockStorage) SetFailAcquire(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failAcquire = fail
}

// SetFailRelease configures the mock to fail release operations
func (m *MockStorage) SetFailRelease(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failRelease = fail
}

// AcquireLock mocks the storage operation
func (m *MockStorage) AcquireLock(ctx context.Context, req *proto.AcquireLockRequest) (*proto.AcquireLockResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.acquireCalled++
	
	if m.failAcquire {
		return nil, ErrMockStorageFailed
	}
	
	// No actual implementation needed for testing
	return &proto.AcquireLockResponse{}, nil
}

// ReleaseLock mocks the storage operation
func (m *MockStorage) ReleaseLock(ctx context.Context, req *proto.ReleaseLockRequest) (*proto.ReleaseLockResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.releaseCalled++
	
	if m.failRelease {
		return nil, ErrMockStorageFailed
	}
	
	// No actual implementation needed for testing
	return &proto.ReleaseLockResponse{}, nil
}

// AcquireCallCount returns the number of times AcquireLock was called
func (m *MockStorage) AcquireCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acquireCalled
}

// ReleaseCallCount returns the number of times ReleaseLock was called
func (m *MockStorage) ReleaseCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.releaseCalled
}

// Reset resets the mock
func (m *MockStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locks = make(map[string]*proto.LockHandle)
	m.acquireCalled = 0
	m.releaseCalled = 0
	m.failAcquire = false
	m.failRelease = false
}

// Error for mocked failures
var ErrMockStorageFailed = proto.NewError("mock storage operation failed")