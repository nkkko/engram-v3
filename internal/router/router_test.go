package router

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixed UUID generation for testing
func init() {
	// Replace the generateID function with a deterministic version for testing
	// We need to generate unique IDs for each test, but deterministic ones
	var counter int
	generateID = func() string {
		counter++
		return fmt.Sprintf("test-subscription-id-%d", counter)
	}
}

// Helper to create test work units
func createTestWorkUnit(contextID string) *proto.WorkUnit {
	return &proto.WorkUnit{
		Id:        uuid.NewString(),
		ContextId: contextID,
		AgentId:   "test-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Payload:   []byte("test payload"),
	}
}

func TestRouterSubscribe(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Subscribe to a context
	sub := router.Subscribe("context-1")
	
	// Verify subscription properties
	assert.Contains(t, sub.ID, "test-subscription-id")
	assert.Equal(t, []string{"context-1"}, sub.ContextIDs)
	assert.NotNil(t, sub.Events)
	
	// Verify internal state
	router.mu.RLock()
	defer router.mu.RUnlock()
	
	// Check subscription is stored
	assert.Contains(t, router.subscriptions, sub.ID)
	
	// Check context mapping
	assert.Contains(t, router.contextSubs, "context-1")
	assert.Contains(t, router.contextSubs["context-1"], sub.ID)
}

func TestRouterMultiContextSubscribe(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Subscribe to multiple contexts
	sub := router.Subscribe("context-1", "context-2", "context-3")
	
	// Verify subscription properties
	assert.Equal(t, []string{"context-1", "context-2", "context-3"}, sub.ContextIDs)
	
	// Verify internal state
	router.mu.RLock()
	defer router.mu.RUnlock()
	
	// Check context mappings
	for _, ctx := range sub.ContextIDs {
		assert.Contains(t, router.contextSubs, ctx)
		assert.Contains(t, router.contextSubs[ctx], sub.ID)
	}
}

func TestRouterUnsubscribe(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Subscribe to a context
	sub := router.Subscribe("context-1")
	
	// Verify subscription exists
	router.mu.RLock()
	assert.Contains(t, router.subscriptions, sub.ID)
	assert.Contains(t, router.contextSubs, "context-1")
	router.mu.RUnlock()
	
	// Unsubscribe
	router.Unsubscribe(sub.ID)
	
	// Verify subscription is removed
	router.mu.RLock()
	defer router.mu.RUnlock()
	
	assert.NotContains(t, router.subscriptions, sub.ID)
	assert.NotContains(t, router.contextSubs, "context-1")
}

func TestRouterUpdateSubscription(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Subscribe to initial contexts
	sub := router.Subscribe("context-1", "context-2")
	
	// Update subscription
	err := router.UpdateSubscription(sub.ID, "context-3", "context-4")
	require.NoError(t, err)
	
	// Verify updated subscription
	router.mu.RLock()
	defer router.mu.RUnlock()
	
	// Check old contexts are removed
	assert.NotContains(t, router.contextSubs, "context-1")
	assert.NotContains(t, router.contextSubs, "context-2")
	
	// Check new contexts are added
	assert.Contains(t, router.contextSubs, "context-3")
	assert.Contains(t, router.contextSubs, "context-4")
	assert.Contains(t, router.contextSubs["context-3"], sub.ID)
	assert.Contains(t, router.contextSubs["context-4"], sub.ID)
	
	// Check subscription is updated
	updatedSub := router.subscriptions[sub.ID]
	assert.Equal(t, []string{"context-3", "context-4"}, updatedSub.ContextIDs)
}

func TestRouterUpdateNonExistentSubscription(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Try to update non-existent subscription
	err := router.UpdateSubscription("non-existent", "context-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription not found")
}

func TestRouterEventRouting(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Create two subscribers for different contexts
	sub1 := router.Subscribe("context-1")
	sub2 := router.Subscribe("context-2")
	
	// Create work units for each context
	wu1 := createTestWorkUnit("context-1")
	wu2 := createTestWorkUnit("context-2")
	wu3 := createTestWorkUnit("context-3") // No subscribers for this context
	
	// Give the subscriptions a moment to be fully registered
	time.Sleep(10 * time.Millisecond)
	
	// Route events
	router.routeEvent(wu1)
	router.routeEvent(wu2)
	router.routeEvent(wu3)
	
	// Verify each subscriber received the correct events
	var receivedEvent1 *proto.WorkUnit
	var receivedEvent2 *proto.WorkUnit
	
	// Use a timeout covering both channel reads
	timeout := time.After(100 * time.Millisecond)
	
	// Create done channels for each subscriber
	done1 := make(chan bool)
	done2 := make(chan bool)
	
	// Read from sub1 in a goroutine
	go func() {
		receivedEvent1 = <-sub1.Events
		done1 <- true
	}()
	
	// Read from sub2 in a goroutine
	go func() {
		receivedEvent2 = <-sub2.Events
		done2 <- true
	}()
	
	// Wait for both reads or timeout
	for i := 0; i < 2; i++ {
		select {
		case <-done1:
			assert.Equal(t, wu1.Id, receivedEvent1.Id)
			assert.Equal(t, "context-1", receivedEvent1.ContextId)
		case <-done2:
			assert.Equal(t, wu2.Id, receivedEvent2.Id)
			assert.Equal(t, "context-2", receivedEvent2.ContextId)
		case <-timeout:
			t.Fatal("Timeout waiting for events")
		}
	}
	
	// Verify no more events are in the channels
	select {
	case event := <-sub1.Events:
		t.Fatalf("Unexpected event received on sub1: %v", event)
	case event := <-sub2.Events:
		t.Fatalf("Unexpected event received on sub2: %v", event)
	case <-time.After(50 * time.Millisecond):
		// This is the expected case - no more events
	}
}

func TestRouterEventRouting_MultipleSubscribers(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Create multiple subscribers for the same context
	sub1 := router.Subscribe("shared-context")
	sub2 := router.Subscribe("shared-context")
	sub3 := router.Subscribe("shared-context")
	
	// Give the subscriptions a moment to be fully registered
	time.Sleep(10 * time.Millisecond)
	
	// Create a work unit for the shared context
	wu := createTestWorkUnit("shared-context")
	
	// Route the event
	router.routeEvent(wu)
	
	// Create a wait group to sync all subscribers
	var wg sync.WaitGroup
	wg.Add(3)
	
	// We'll track any failures
	errorCh := make(chan string, 3)
	
	// Check all subscribers in parallel
	for i, sub := range []*Subscription{sub1, sub2, sub3} {
		go func(idx int, s *Subscription) {
			defer wg.Done()
			
			select {
			case event := <-s.Events:
				if event.Id != wu.Id {
					errorCh <- fmt.Sprintf("Incorrect ID for subscriber %d", idx)
				}
				if event.ContextId != "shared-context" {
					errorCh <- fmt.Sprintf("Incorrect context for subscriber %d", idx)
				}
			case <-time.After(100 * time.Millisecond):
				errorCh <- fmt.Sprintf("Timeout waiting for event on subscription %d", idx)
			}
		}(i, sub)
	}
	
	// Wait for all subscribers to be checked
	wg.Wait()
	
	// Check for errors
	if len(errorCh) > 0 {
		var errors []string
		for len(errorCh) > 0 {
			errors = append(errors, <-errorCh)
		}
		for _, err := range errors {
			t.Error(err)
		}
	}
}

func TestRouterFullBuffer(t *testing.T) {
	// Create router with very small buffer size
	router := NewRouter(Config{MaxBufferSize: 1})
	
	// Subscribe to a context
	sub := router.Subscribe("context-1")
	
	// Create multiple work units
	wu1 := createTestWorkUnit("context-1")
	wu2 := createTestWorkUnit("context-1")
	
	// Route the first event
	router.routeEvent(wu1)
	
	// Don't read from the channel to fill the buffer
	
	// Route the second event - this should not block but log a warning
	router.routeEvent(wu2)
	
	// Verify only the first event is in the channel
	select {
	case event := <-sub.Events:
		assert.Equal(t, wu1.Id, event.Id)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event")
	}
	
	// Verify no more events are in the channel
	select {
	case event := <-sub.Events:
		t.Fatalf("Unexpected event received: %v", event)
	case <-time.After(100 * time.Millisecond):
		// This is expected - second event was dropped
	}
}

func TestRouterStart(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create a channel to send events to the router
	eventCh := make(chan *proto.WorkUnit, 10)
	
	// Start the router in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	var routerErr error
	
	go func() {
		defer wg.Done()
		routerErr = router.Start(ctx, eventCh)
	}()
	
	// Create a subscriber
	sub := router.Subscribe("test-context")
	
	// Send an event
	wu := createTestWorkUnit("test-context")
	eventCh <- wu
	
	// Wait for the event to be routed
	var receivedEvent *proto.WorkUnit
	select {
	case receivedEvent = <-sub.Events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event")
	}
	
	// Verify the event
	assert.Equal(t, wu.Id, receivedEvent.Id)
	
	// Cancel the context to stop the router
	cancel()
	
	// Wait for the router to stop
	wg.Wait()
	
	// Check if router stopped due to context cancellation
	assert.Error(t, routerErr)
	assert.Equal(t, context.Canceled, routerErr)
}

func TestRouterPerformCleanup(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Override max idle time for testing
	const maxIdleTime = 5 * time.Minute
	
	// Subscribe to a context
	sub := router.Subscribe("context-1")
	
	// Manually set LastAccessed to a time in the past
	sub.mu.Lock()
	sub.LastAccessed = time.Now().Add(-maxIdleTime - time.Second)
	sub.mu.Unlock()
	
	// Perform cleanup
	router.performCleanup()
	
	// Verify subscription is removed
	router.mu.RLock()
	_, exists := router.subscriptions[sub.ID]
	router.mu.RUnlock()
	
	assert.False(t, exists, "Subscription should be removed after cleanup")
}

func TestRouterShutdown(t *testing.T) {
	// Create router with default config
	router := NewRouter()
	
	// Create several subscriptions
	sub1 := router.Subscribe("context-1")
	sub2 := router.Subscribe("context-2") 
	sub3 := router.Subscribe("context-3")
	
	// Make sure we have three different subscriptions
	assert.NotEqual(t, sub1.ID, sub2.ID)
	assert.NotEqual(t, sub1.ID, sub3.ID)
	assert.NotEqual(t, sub2.ID, sub3.ID)
	
	// Verify subscriptions exist
	router.mu.RLock()
	count := len(router.subscriptions)
	contextCount := len(router.contextSubs)
	router.mu.RUnlock()
	
	assert.Equal(t, 3, count, "Should have 3 subscriptions")
	assert.Equal(t, 3, contextCount, "Should have 3 context subscriptions")
	
	// Shutdown the router
	err := router.Shutdown(context.Background())
	require.NoError(t, err)
	
	// Verify all subscriptions are removed
	router.mu.RLock()
	assert.Empty(t, router.subscriptions)
	assert.Empty(t, router.contextSubs)
	router.mu.RUnlock()
}