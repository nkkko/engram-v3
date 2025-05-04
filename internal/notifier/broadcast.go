package notifier

import (
	"sync"
	"time"

	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog/log"
)

// BroadcastBuffer is a zero-copy broadcast buffer for efficient event distribution
type BroadcastBuffer struct {
	// Configuration
	bufferSize    int
	flushInterval time.Duration
	
	// Subscription management
	subscribers     map[string]chan *proto.WorkUnit
	subscribersLock sync.RWMutex
	
	// Broadcast buffers for zero-copy distribution
	currentBuffer     []*proto.WorkUnit
	currentBufferLock sync.RWMutex
	
	// Control channels
	forceFlush chan struct{}
	close      chan struct{}
	
	// Metrics
	metrics *metrics.Metrics
}

// NewBroadcastBuffer creates a new broadcast buffer
func NewBroadcastBuffer(bufferSize int, flushInterval time.Duration) *BroadcastBuffer {
	b := &BroadcastBuffer{
		bufferSize:     bufferSize,
		flushInterval:  flushInterval,
		subscribers:    make(map[string]chan *proto.WorkUnit),
		currentBuffer:  make([]*proto.WorkUnit, 0, bufferSize),
		forceFlush:     make(chan struct{}),
		close:          make(chan struct{}),
		metrics:        metrics.GetMetrics(),
	}
	
	// Start buffer flush loop
	go b.bufferFlushLoop()
	
	return b
}

// Subscribe adds a new subscriber to the broadcast
func (b *BroadcastBuffer) Subscribe(id string, buffer int) chan *proto.WorkUnit {
	b.subscribersLock.Lock()
	defer b.subscribersLock.Unlock()
	
	// Create subscriber channel
	channel := make(chan *proto.WorkUnit, buffer)
	b.subscribers[id] = channel
	
	// Update subscriber count metric
	b.metrics.NotifierConnectionsActive.Inc()
	
	return channel
}

// Unsubscribe removes a subscriber from the broadcast
func (b *BroadcastBuffer) Unsubscribe(id string) {
	b.subscribersLock.Lock()
	defer b.subscribersLock.Unlock()
	
	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
		
		// Update subscriber count metric
		b.metrics.NotifierConnectionsActive.Dec()
	}
}

// Publish adds a work unit to the broadcast buffer
func (b *BroadcastBuffer) Publish(workUnit *proto.WorkUnit) {
	b.currentBufferLock.Lock()
	defer b.currentBufferLock.Unlock()
	
	// Add to current buffer
	b.currentBuffer = append(b.currentBuffer, workUnit)
	
	// If buffer is full, trigger flush
	if len(b.currentBuffer) >= b.bufferSize {
		// Non-blocking send to force flush channel
		select {
		case b.forceFlush <- struct{}{}:
		default:
			// Channel is full, flush will happen soon anyway
		}
	}
}

// bufferFlushLoop periodically flushes the buffer to all subscribers
func (b *BroadcastBuffer) bufferFlushLoop() {
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.forceFlush:
			b.flush()
		case <-b.close:
			// Final flush
			b.flush()
			return
		}
	}
}

// flush sends buffered work units to all subscribers
func (b *BroadcastBuffer) flush() {
	// Swap buffers to minimize lock contention
	b.currentBufferLock.Lock()
	buffer := b.currentBuffer
	if len(buffer) == 0 {
		b.currentBufferLock.Unlock()
		return
	}
	
	// Reset current buffer
	b.currentBuffer = make([]*proto.WorkUnit, 0, b.bufferSize)
	b.currentBufferLock.Unlock()
	
	// Get a snapshot of subscribers to avoid long lock holding
	b.subscribersLock.RLock()
	subscriptions := make(map[string]chan *proto.WorkUnit, len(b.subscribers))
	for id, ch := range b.subscribers {
		subscriptions[id] = ch
	}
	b.subscribersLock.RUnlock()
	
	// Nothing to do if no subscribers
	if len(subscriptions) == 0 {
		return
	}
	
	// Record metrics
	start := time.Now()
	delivered := 0
	skipped := 0
	
	// Send to each subscriber
	for id, ch := range subscriptions {
		// Send all buffer items in a non-blocking manner
		sent := 0
		for _, workUnit := range buffer {
			select {
			case ch <- workUnit:
				sent++
			default:
				// Channel is full, skip
				skipped++
				
				// Log warning for subscriptions with high drop rates
				if skipped > 100 {
					log.Warn().
						Str("subscriber_id", id).
						Int("dropped", skipped).
						Msg("Subscriber channel is full, dropping events")
				}
			}
		}
		delivered += sent
		
		// Update metrics
		b.metrics.NotifierEventsPublished.WithLabelValues("broadcast").Add(float64(sent))
	}
	
	// Record broadcast delay
	delay := time.Since(start).Seconds()
	b.metrics.NotifierEventDelay.Observe(delay)
	
	// Log high latency broadcasts
	if delay > 0.1 { // 100ms threshold
		log.Warn().
			Float64("delay_seconds", delay).
			Int("events", len(buffer)).
			Int("subscribers", len(subscriptions)).
			Int("delivered", delivered).
			Int("skipped", skipped).
			Msg("High latency in broadcast buffer flush")
	}
}

// Close shuts down the broadcast buffer
func (b *BroadcastBuffer) Close() error {
	close(b.close)
	
	// Close all subscriber channels
	b.subscribersLock.Lock()
	defer b.subscribersLock.Unlock()
	
	for id, ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, id)
	}
	
	return nil
}