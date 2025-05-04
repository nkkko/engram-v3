package notifier

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestBroadcastBuffer tests the zero-copy broadcast buffer functionality
func TestBroadcastBuffer(t *testing.T) {
	// Create a broadcast buffer
	bufferSize := 10
	flushInterval := 50 * time.Millisecond
	buffer := NewBroadcastBuffer(bufferSize, flushInterval)
	defer buffer.Close()

	// Create test work unit
	workUnit := &proto.WorkUnit{
		Id:        "test-unit-1",
		ContextId: "test-context",
		AgentId:   "test-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Ts:        timestamppb.Now(),
		Payload:   []byte("Test message"),
	}

	// Subscribe multiple clients
	const numSubscribers = 5
	channels := make([]chan *proto.WorkUnit, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		channels[i] = buffer.Subscribe(fmt.Sprintf("subscriber-%d", i), 10)
	}

	// Publish a work unit
	buffer.Publish(workUnit)

	// Wait for flush to happen
	time.Sleep(flushInterval * 2)

	// Verify all subscribers received the work unit
	for i, ch := range channels {
		select {
		case received := <-ch:
			assert.Equal(t, workUnit.Id, received.Id, "Subscriber %d should receive correct work unit", i)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for subscriber %d", i)
		}
	}

	// Unsubscribe one client
	buffer.Unsubscribe("subscriber-0")

	// Publish another work unit
	workUnit2 := &proto.WorkUnit{
		Id:        "test-unit-2",
		ContextId: "test-context",
		AgentId:   "test-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Ts:        timestamppb.Now(),
		Payload:   []byte("Second test message"),
	}
	buffer.Publish(workUnit2)

	// Wait for flush to happen
	time.Sleep(flushInterval * 2)

	// Verify unsubscribed client doesn't receive (would block on recv), but others do
	for i := 1; i < numSubscribers; i++ {
		select {
		case received := <-channels[i]:
			assert.Equal(t, workUnit2.Id, received.Id, "Subscriber %d should receive second work unit", i)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for subscriber %d for second message", i)
		}
	}
}

// TestBufferFlushTriggers tests automatic flush triggers
func TestBufferFlushTriggers(t *testing.T) {
	// Create a broadcast buffer with short flush interval
	bufferSize := 5
	flushInterval := 50 * time.Millisecond
	buffer := NewBroadcastBuffer(bufferSize, flushInterval)
	defer buffer.Close()

	// Subscribe a client
	ch := buffer.Subscribe("test-client", 10)

	// Test trigger: flush interval
	workUnit1 := &proto.WorkUnit{
		Id:        "interval-unit",
		ContextId: "test-context",
		AgentId:   "test-agent",
		Type:      proto.WorkUnitType_MESSAGE,
	}
	buffer.Publish(workUnit1)

	// Wait for time-based flush
	select {
	case received := <-ch:
		assert.Equal(t, workUnit1.Id, received.Id, "Should receive work unit after interval flush")
	case <-time.After(flushInterval * 3):
		t.Fatal("Timeout waiting for interval-based flush")
	}

	// Test trigger: buffer full
	workUnits := make([]*proto.WorkUnit, bufferSize)
	for i := 0; i < bufferSize; i++ {
		workUnits[i] = &proto.WorkUnit{
			Id:        fmt.Sprintf("buffer-unit-%d", i),
			ContextId: "test-context",
			AgentId:   "test-agent",
			Type:      proto.WorkUnitType_MESSAGE,
		}
		buffer.Publish(workUnits[i])
	}

	// Should trigger an immediate flush when buffer is full
	for i := 0; i < bufferSize; i++ {
		select {
		case received := <-ch:
			foundMatch := false
			for _, expected := range workUnits {
				if expected.Id == received.Id {
					foundMatch = true
					break
				}
			}
			assert.True(t, foundMatch, "Received message should be one of the published ones")
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Timeout waiting for buffer-full flush after receiving %d/%d messages", i, bufferSize)
		}
	}
}

// TestBufferChannelFull tests handling of full subscriber channels
func TestBufferChannelFull(t *testing.T) {
	// Create a buffer with very short flush interval for testing
	bufferSize := 5
	flushInterval := 10 * time.Millisecond
	buffer := NewBroadcastBuffer(bufferSize, flushInterval)
	defer buffer.Close()

	// Create a subscriber with a very small channel buffer
	ch := buffer.Subscribe("subscriber", 1)

	// Publish more units than the channel can hold
	for i := 0; i < 5; i++ {
		buffer.Publish(&proto.WorkUnit{
			Id:        fmt.Sprintf("overflow-unit-%d", i),
			ContextId: "test-context",
			AgentId:   "test-agent",
			Type:      proto.WorkUnitType_MESSAGE,
		})
	}

	// Force a flush
	time.Sleep(flushInterval * 2)

	// Should receive at least one message
	select {
	case <-ch:
		// Successfully received a message
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should receive at least one message")
	}

	// Don't check for the others; they may or may not be dropped depending on timing
	// The important thing is that the buffer doesn't block or crash when channels are full
}

// BenchmarkBroadcastBuffer benchmarks the zero-copy broadcast buffer
func BenchmarkBroadcastBuffer(b *testing.B) {
	// Create work units ahead of time
	numWorkUnits := 1000
	workUnits := make([]*proto.WorkUnit, numWorkUnits)
	for i := 0; i < numWorkUnits; i++ {
		workUnits[i] = &proto.WorkUnit{
			Id:        fmt.Sprintf("bench-unit-%d", i),
			ContextId: "bench-context",
			AgentId:   "bench-agent",
			Type:      proto.WorkUnitType_MESSAGE,
			Ts:        timestamppb.Now(),
			Payload:   []byte(fmt.Sprintf("Benchmark message %d", i)),
		}
	}

	// Benchmark variations
	benchCases := []struct {
		name         string
		bufferSize   int
		flushMs      int
		subscribers  int
		channelDepth int
	}{
		{"Small_1Sub", 10, 20, 1, 10},
		{"Small_10Subs", 10, 20, 10, 10},
		{"Medium_1Sub", 100, 50, 1, 100},
		{"Medium_10Subs", 100, 50, 10, 100},
		{"Large_1Sub", 500, 100, 1, 500},
		{"Large_10Subs", 500, 100, 10, 500},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			// Create broadcast buffer with specified parameters
			buffer := NewBroadcastBuffer(
				bc.bufferSize,
				time.Duration(bc.flushMs)*time.Millisecond,
			)
			defer buffer.Close()

			// Create subscribers
			var channels []chan *proto.WorkUnit
			for i := 0; i < bc.subscribers; i++ {
				channels = append(channels, buffer.Subscribe(
					fmt.Sprintf("bench-sub-%d", i),
					bc.channelDepth,
				))
			}

			// Use an atomic counter to track received messages
			var received int64

			// Start goroutines to consume messages
			var wg sync.WaitGroup
			for i := 0; i < bc.subscribers; i++ {
				wg.Add(1)
				go func(ch <-chan *proto.WorkUnit) {
					defer wg.Done()
					for range ch {
						atomic.AddInt64(&received, 1)
					}
				}(channels[i])
			}

			// Reset timer before sending messages
			b.ResetTimer()

			// Publish b.N work units
			for i := 0; i < b.N; i++ {
				buffer.Publish(workUnits[i%numWorkUnits])
			}

			// Stop timer while waiting for messages to be processed
			b.StopTimer()

			// Force a final flush and allow time for processing
			time.Sleep(time.Duration(bc.flushMs*2) * time.Millisecond)

			// Close subscriber channels to stop consumer goroutines
			for i := 0; i < bc.subscribers; i++ {
				buffer.Unsubscribe(fmt.Sprintf("bench-sub-%d", i))
			}

			// Wait for all consumers to finish
			wg.Wait()

			// Log how many messages were successfully received
			totalPossible := int64(b.N * bc.subscribers)
			receivePercentage := float64(received) / float64(totalPossible) * 100
			b.Logf(
				"Received %d/%d messages (%.2f%%)",
				received, totalPossible, receivePercentage,
			)
		})
	}
}

// BenchmarkBroadcastVsDirectSend compares broadcasting with direct channel sends
func BenchmarkBroadcastVsDirectSend(b *testing.B) {
	// Create work units ahead of time
	workUnit := &proto.WorkUnit{
		Id:        "bench-unit",
		ContextId: "bench-context",
		AgentId:   "bench-agent",
		Type:      proto.WorkUnitType_MESSAGE,
		Ts:        timestamppb.Now(),
		Payload:   []byte("Benchmark message"),
	}

	// Number of subscribers
	const numSubscribers = 10

	// Benchmark using broadcast buffer
	b.Run("ZeroCopyBroadcast", func(b *testing.B) {
		// Create broadcast buffer
		buffer := NewBroadcastBuffer(100, 50*time.Millisecond)
		defer buffer.Close()

		// Create subscribers
		var channels []chan *proto.WorkUnit
		for i := 0; i < numSubscribers; i++ {
			channels = append(channels, buffer.Subscribe(
				fmt.Sprintf("sub-%d", i),
				1000,
			))
		}

		// Reset timer before sending messages
		b.ResetTimer()

		// Publish b.N work units
		for i := 0; i < b.N; i++ {
			buffer.Publish(workUnit)
		}

		// Reset timer while waiting for flush
		b.StopTimer()

		// Force flush and give time for processing
		time.Sleep(100 * time.Millisecond)
	})

	// Benchmark using direct channel sends (copying to each channel)
	b.Run("DirectSendWithCopy", func(b *testing.B) {
		// Create subscriber channels
		channels := make([]chan *proto.WorkUnit, numSubscribers)
		for i := 0; i < numSubscribers; i++ {
			channels[i] = make(chan *proto.WorkUnit, 1000)
		}

		// Start goroutines to consume messages
		var wg sync.WaitGroup
		for i := 0; i < numSubscribers; i++ {
			wg.Add(1)
			go func(ch <-chan *proto.WorkUnit) {
				defer wg.Done()
				for range ch {
					// Consume messages
				}
			}(channels[i])
		}

		// Reset timer before sending messages
		b.ResetTimer()

		// Send b.N work units
		for i := 0; i < b.N; i++ {
			// Send to each channel (with potential blocking)
			for j := 0; j < numSubscribers; j++ {
				select {
				case channels[j] <- workUnit:
					// Successfully sent
				default:
					// Channel full, skip
				}
			}
		}

		// Reset timer while cleaning up
		b.StopTimer()

		// Close channels to stop goroutines
		for i := 0; i < numSubscribers; i++ {
			close(channels[i])
		}

		// Wait for all goroutines to finish
		wg.Wait()
	})
}