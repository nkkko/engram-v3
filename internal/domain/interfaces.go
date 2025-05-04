package domain

import (
	"context"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// StorageEngine defines the interface for storage implementations
type StorageEngine interface {
	// Start begins the storage engine operation
	Start(ctx context.Context) error

	// Shutdown stops the storage engine
	Shutdown(ctx context.Context) error

	// EventStream returns a channel of work unit events
	EventStream() <-chan *proto.WorkUnit

	// Work unit operations
	CreateWorkUnit(ctx context.Context, req *proto.CreateWorkUnitRequest) (*proto.WorkUnit, error)
	GetWorkUnit(ctx context.Context, id string) (*proto.WorkUnit, error)
	ListWorkUnits(ctx context.Context, contextID string, limit int, cursor string, reverse bool) ([]*proto.WorkUnit, string, error)

	// Context operations
	UpdateContext(ctx context.Context, req *proto.UpdateContextRequest) (*proto.Context, error)
	GetContext(ctx context.Context, id string) (*proto.Context, error)

	// Lock operations
	AcquireLock(ctx context.Context, req *proto.AcquireLockRequest) (*proto.LockHandle, error)
	GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error)
	ReleaseLock(ctx context.Context, req *proto.ReleaseLockRequest) (bool, error)
}

// EventRouter defines the interface for event routing implementations
type EventRouter interface {
	// Start begins processing events from the provided stream
	Start(ctx context.Context, events <-chan *proto.WorkUnit) error

	// Shutdown performs cleanup and stops the router
	Shutdown(ctx context.Context) error

	// Subscribe creates a new subscription for the specified contexts
	Subscribe(contextIDs ...string) *Subscription

	// Unsubscribe removes a subscription
	Unsubscribe(subID string) 

	// UpdateSubscription updates the contexts for a subscription
	UpdateSubscription(subID string, contextIDs ...string) error
}

// Subscription represents a client's subscription to events
type Subscription struct {
	ID           string
	ContextIDs   []string
	Events       chan *proto.WorkUnit
	LastAccessed time.Time
}

// SearchEngine defines the interface for search implementations
type SearchEngine interface {
	// Start begins the search engine operation with an event stream
	Start(ctx context.Context, events <-chan *proto.WorkUnit) error

	// Shutdown performs cleanup and stops the search engine
	Shutdown(ctx context.Context) error

	// Search performs a search with the given parameters
	Search(ctx context.Context, query string, contextID string, agentIDs []string, 
	       types []proto.WorkUnitType, metaFilters map[string]string, limit, offset int) ([]*proto.WorkUnit, int, error)
}

// NotifierEngine defines the interface for real-time notification implementations
type NotifierEngine interface {
	// Start begins the notifier operation with a subscription
	Start(ctx context.Context, subscription *Subscription) error

	// Shutdown performs cleanup and stops the notifier
	Shutdown(ctx context.Context) error

	// Subscribe adds a new client to receive notifications
	Subscribe(clientID string, contextID string) (chan *proto.WorkUnit, error)

	// Unsubscribe removes a client subscription
	Unsubscribe(clientID string) error
}

// LockManager defines the interface for lock management implementations
type LockManager interface {
	// Start begins the lock manager operation
	Start(ctx context.Context) error

	// Shutdown performs cleanup and stops the lock manager
	Shutdown(ctx context.Context) error

	// AcquireLock attempts to acquire a lock on a resource
	AcquireLock(ctx context.Context, resourcePath string, agentID string, ttl time.Duration) (*proto.LockHandle, error)

	// GetLock retrieves information about a lock
	GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error)

	// ReleaseLock releases a lock on a resource
	ReleaseLock(ctx context.Context, resourcePath string, agentID string, lockID string) (bool, error)
}

// APIEngine defines the interface for API implementations
type APIEngine interface {
	// Start initializes and runs the API server
	Start(ctx context.Context) error

	// Shutdown stops the API server
	Shutdown(ctx context.Context) error
}