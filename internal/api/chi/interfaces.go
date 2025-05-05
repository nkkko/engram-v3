package chi

import (
	"context"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// StorageEngine defines the interface required by the Chi API for storage operations.
// Method signatures copied from domain.StorageEngine.
type StorageEngine interface {
	// Work unit operations
	CreateWorkUnit(ctx context.Context, req *proto.CreateWorkUnitRequest) (*proto.WorkUnit, error)
	GetWorkUnit(ctx context.Context, id string) (*proto.WorkUnit, error)
	ListWorkUnits(ctx context.Context, contextID string, limit int, cursor string, reverse bool) ([]*proto.WorkUnit, string, error)

	// Context operations
	UpdateContext(ctx context.Context, req *proto.UpdateContextRequest) (*proto.Context, error)
	GetContext(ctx context.Context, id string) (*proto.Context, error)

	// Lock operations (Assuming lock ops are handled directly by LockManager, but if ChiAPI calls them via storage, add them here)
	ReleaseLock(ctx context.Context, req *proto.ReleaseLockRequest) (bool, error)
	AcquireLock(ctx context.Context, req *proto.AcquireLockRequest) (*proto.LockHandle, error)
	GetLock(ctx context.Context, resourcePath string) (*proto.LockHandle, error)
}

// EventRouter defines the interface required by the Chi API for event routing.
// Method signatures should match the ones needed from domain.EventRouter.
type EventRouter interface {
	// Subscribe creates a new subscription for the specified contexts
	// Subscribe(contextIDs ...string) *Subscription // Assuming domain.Subscription is not needed here
	// Add other methods used by ChiAPI handlers as needed
}
