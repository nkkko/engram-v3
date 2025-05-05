package notifier

import (
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
)

// Subscription represents a client's subscription to events (simplified, may need full domain.Subscription if used internally)
type Subscription struct {
	ID           string
	ContextIDs   []string
	Events       chan *proto.WorkUnit
	LastAccessed time.Time
}

// EventRouter defines the interface required by the Notifier for event routing.
type EventRouter interface {
	// Subscribe creates a new subscription for the specified contexts
	Subscribe(contextIDs ...string) *Subscription

	// Unsubscribe removes a subscription
	Unsubscribe(subID string)

	// UpdateSubscription updates the contexts for a subscription
	// UpdateSubscription(subID string, contextIDs ...string) error // Add if needed by Notifier
}
