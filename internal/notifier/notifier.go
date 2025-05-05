package notifier

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/internal/metrics"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config contains notifier configuration
type Config struct {
	// Maximum idle time before dropping a connection
	MaxIdleTime time.Duration

	// Broadcast buffer size for batching events
	BroadcastBufferSize int

	// Flush interval for broadcast buffer
	BroadcastFlushInterval time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxIdleTime:            30 * time.Second,
		BroadcastBufferSize:    200,
		BroadcastFlushInterval: 50 * time.Millisecond,
	}
}

// Client represents a connected client
type Client struct {
	ID            string
	ContextIDs    []string // Contexts the client is subscribed to
	LastActive    time.Time
	conn          *websocket.Conn
	sseChannel    chan []byte
	eventsChannel chan *proto.WorkUnit // Channel for receiving events
	isSSE         bool
	mu            sync.Mutex
}

// Notifier handles real-time notifications to clients
type Notifier struct {
	config            Config
	clients           map[string]*Client
	mu                sync.RWMutex
	logger            zerolog.Logger
	heartbeatInterval time.Duration
	router            EventRouter
	broadcastBuffer   *BroadcastBuffer
	metrics           *metrics.Metrics
}

// NewNotifier creates a new notification manager
func NewNotifier(config Config, r EventRouter) *Notifier {
	logger := log.With().Str("component", "notifier").Logger()

	// Apply default configuration values if not provided
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = DefaultConfig().MaxIdleTime
	}

	if config.BroadcastBufferSize == 0 {
		config.BroadcastBufferSize = DefaultConfig().BroadcastBufferSize
	}

	if config.BroadcastFlushInterval == 0 {
		config.BroadcastFlushInterval = DefaultConfig().BroadcastFlushInterval
	}

	// Create broadcast buffer
	broadcastBuffer := NewBroadcastBuffer(
		config.BroadcastBufferSize,
		config.BroadcastFlushInterval,
	)

	return &Notifier{
		config:            config,
		clients:           make(map[string]*Client),
		logger:            logger,
		heartbeatInterval: 5 * time.Second,
		router:            r,
		broadcastBuffer:   broadcastBuffer,
		metrics:           metrics.GetMetrics(),
	}
}

// Start begins the notifier operation
func (n *Notifier) Start(ctx context.Context, subscription *Subscription) error {
	n.logger.Info().Msg("Starting event notifier")

	// Process events from router through broadcast buffer
	go func() {
		count := 0
		start := time.Now()

		for {
			select {
			case event, ok := <-subscription.Events:
				if !ok {
					n.logger.Info().Msg("Subscription channel closed, stopping notifier")
					return
				}

				// Publish to broadcast buffer
				n.broadcastBuffer.Publish(event)

				// Update metrics
				count++
				if count >= 1000 {
					elapsed := time.Since(start).Seconds()
					throughput := float64(count) / elapsed
					n.metrics.RouterEventsTotal.WithLabelValues("processed").Add(float64(count))
					n.logger.Debug().
						Float64("events_per_second", throughput).
						Int("events", count).
						Float64("elapsed_seconds", elapsed).
						Msg("Event processing throughput")
					count = 0
					start = time.Now()
				}

			case <-ctx.Done():
				n.logger.Info().Msg("Context canceled, stopping notifier")
				return
			}
		}
	}()

	// Start cleanup goroutine
	go n.cleanupIdleClients(ctx)

	// Start heartbeat goroutine
	go n.sendHeartbeats(ctx)

	return nil
}

// RegisterWebSocketHandler registers the WebSocket handler with a Fiber app
func (n *Notifier) RegisterWebSocketHandler(app *fiber.App) {
	// Middleware to upgrade connections to WebSocket
	app.Use("/stream", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket handler
	app.Get("/stream", websocket.New(func(c *websocket.Conn) {
		// Get context ID from query parameters
		contextID := c.Query("context", "*") // Default to wildcard

		// Validate contextID if needed

		n.handleWebSocketClient(c, contextID)
	}))
}

// RegisterSSEHandler registers the Server-Sent Events handler with a Fiber app
func (n *Notifier) RegisterSSEHandler(app *fiber.App) {
	app.Get("/stream-sse", func(c *fiber.Ctx) error {
		// Get context ID from query parameters
		contextID := c.Query("context", "*") // Default to wildcard

		// Validate contextID if needed

		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// Create client
		client := n.createSSEClient(contextID)

		// Send initial connection message
		connMsg := fmt.Sprintf("event: connected\ndata: {\"client_id\":\"%s\"}\n\n", client.ID)
		_, _ = c.WriteString(connMsg)

		// Handle client disconnect
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			for {
				select {
				case msg, ok := <-client.sseChannel:
					if !ok {
						// Channel closed, exit
						return
					}

					// Write SSE message
					fmt.Fprintf(w, "data: %s\n\n", msg)
					w.Flush()

					// Update last active time
					client.mu.Lock()
					client.LastActive = time.Now()
					client.mu.Unlock()

				case <-c.Context().Done():
					// Client disconnected
					n.removeClient(client.ID)
					return
				}
			}
		})

		return nil
	})
}

// handleWebSocketClient processes a WebSocket connection
func (n *Notifier) handleWebSocketClient(conn *websocket.Conn, contextID string) {
	// Create client
	clientID := generateID()

	// Create events channel and subscribe to broadcast buffer
	eventsChannel := n.broadcastBuffer.Subscribe(clientID, 100)

	client := &Client{
		ID:            clientID,
		ContextIDs:    []string{contextID},
		LastActive:    time.Now(),
		conn:          conn,
		eventsChannel: eventsChannel,
		isSSE:         false,
	}

	// Register client
	n.mu.Lock()
	n.clients[clientID] = client
	n.mu.Unlock()
	n.metrics.ConnectedClients.Inc()
	n.logger.Info().Str("client_id", clientID).Str("context_id", contextID).Str("addr", conn.RemoteAddr().String()).Msg("WebSocket client connected")

	// Inform the router about the new subscription (if needed)
	// Example: sub := n.router.Subscribe(contextID)
	// Store sub.ID with the client if needed for unsubscribe

	// Send connection confirmation
	conn.WriteJSON(fiber.Map{"event": "connected", "client_id": clientID})

	// Goroutine to send events to the client
	go func() {
		for event := range client.eventsChannel {
			// Check if client is subscribed to this event's context
			subscribed := false
			for _, ctxID := range client.ContextIDs {
				if ctxID == "*" || ctxID == event.ContextId {
					subscribed = true
					break
				}
			}
			if !subscribed {
				continue
			}

			// Send event
			if err := conn.WriteJSON(event); err != nil {
				n.logger.Warn().Err(err).Str("client_id", clientID).Msg("Error writing to WebSocket")
				// Assume client disconnected
				close(client.eventsChannel) // Signal stop
				return
			}
			client.mu.Lock()
			client.LastActive = time.Now()
			client.mu.Unlock()
			n.metrics.EventsSentTotal.WithLabelValues("websocket").Inc()
		}
		// If eventsChannel is closed externally (e.g., during shutdown or removeClient)
		n.removeClient(clientID)
	}()

	// Read messages from the client (e.g., subscription updates, heartbeats)
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				n.logger.Error().Err(err).Str("client_id", clientID).Msg("WebSocket read error")
			} else {
				n.logger.Info().Err(err).Str("client_id", clientID).Msg("WebSocket client disconnected")
			}
			// Ensure cleanup happens even if eventsChannel loop didn't exit first
			n.removeClient(clientID)
			break // Exit loop
		}

		if msgType == websocket.TextMessage {
			n.processClientMessage(client, msg)
		}
	}
}

// createSSEClient creates and registers a Server-Sent Events client
func (n *Notifier) createSSEClient(contextID string) *Client {
	clientID := generateID()
	eventsChannel := n.broadcastBuffer.Subscribe(clientID, 100)

	client := &Client{
		ID:            clientID,
		ContextIDs:    []string{contextID},
		LastActive:    time.Now(),
		sseChannel:    make(chan []byte, 100), // Buffer for SSE messages
		eventsChannel: eventsChannel,
		isSSE:         true,
	}

	n.mu.Lock()
	n.clients[clientID] = client
	n.mu.Unlock()
	n.metrics.ConnectedClients.Inc()
	n.logger.Info().Str("client_id", clientID).Str("context_id", contextID).Msg("SSE client connected")

	// Inform the router about the new subscription (if needed)
	// Example: sub := n.router.Subscribe(contextID)
	// Store sub.ID with the client if needed for unsubscribe

	// Goroutine to send events to the client's SSE channel
	go func() {
		for event := range client.eventsChannel {
			// Check if client is subscribed to this event's context
			subscribed := false
			for _, ctxID := range client.ContextIDs {
				if ctxID == "*" || ctxID == event.ContextId {
					subscribed = true
					break
				}
			}
			if !subscribed {
				continue
			}

			// Marshal event to JSON
			jsonData, err := json.Marshal(event)
			if err != nil {
				n.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to marshal event for SSE")
				continue
			}

			// Send to SSE channel (will be picked up by the handler)
			select {
			case client.sseChannel <- jsonData:
				n.metrics.EventsSentTotal.WithLabelValues("sse").Inc()
			default:
				n.logger.Warn().Str("client_id", clientID).Msg("SSE channel full, dropping event")
				n.metrics.EventsDroppedTotal.WithLabelValues("sse").Inc()
			}
		}
		// If eventsChannel is closed externally (e.g., during shutdown or removeClient)
		close(client.sseChannel) // Signal handler to stop
		n.removeClient(clientID) // Ensure cleanup
	}()

	return client
}

// processClientMessage handles messages received from WebSocket clients
func (n *Notifier) processClientMessage(client *Client, message []byte) {
	client.mu.Lock()
	client.LastActive = time.Now()
	client.mu.Unlock()

	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		n.logger.Warn().Err(err).Str("client_id", client.ID).Msg("Failed to parse client message")
		client.conn.WriteJSON(fiber.Map{"error": "Invalid message format"})
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		client.conn.WriteJSON(fiber.Map{"error": "Missing or invalid message type"})
		return
	}

	switch msgType {
	case "heartbeat":
		// Already updated LastActive
		client.conn.WriteJSON(fiber.Map{"type": "heartbeat_ack"})
	case "subscribe":
		// Example: Handle updating subscriptions
		contexts, ok := msg["contexts"].([]interface{})
		if !ok {
			client.conn.WriteJSON(fiber.Map{"error": "Invalid contexts format for subscribe"})
			return
		}
		contextIDs := []string{}
		for _, ctx := range contexts {
			if ctxStr, ok := ctx.(string); ok {
				contextIDs = append(contextIDs, ctxStr)
			} else {
				client.conn.WriteJSON(fiber.Map{"error": "Invalid context ID in list"})
				return
			}
		}
		client.mu.Lock()
		client.ContextIDs = contextIDs
		client.mu.Unlock()
		// Optionally, update the underlying router subscription if needed
		// err := n.router.UpdateSubscription(client.SubscriptionID, contextIDs...)
		client.conn.WriteJSON(fiber.Map{"type": "subscribe_ack", "contexts": contextIDs})
		n.logger.Info().Str("client_id", client.ID).Strs("contexts", contextIDs).Msg("Client updated subscription")
	default:
		client.conn.WriteJSON(fiber.Map{"error": "Unknown message type"})
	}
}

// broadcastEvent sends an event to all relevant connected clients
// This method is less efficient than using the BroadcastBuffer and might be removed.
// func (n *Notifier) broadcastEvent(event *proto.WorkUnit) {
// 	n.mu.RLock()
// 	defer n.mu.RUnlock()
//
// 	for _, client := range n.clients {
// 		// Check if client is subscribed to this context
// 		subscribed := false
// 		for _, ctxID := range client.ContextIDs {
// 			if ctxID == "*" || ctxID == event.ContextId {
// 				subscribed = true
// 				break
// 			}
// 		}
// 		if !subscribed {
// 			continue
// 		}
//
// 		// Send event non-blockingly
// 		select {
// 		case client.eventsChannel <- event:
// 			n.metrics.EventsSentTotal.Inc()
// 		default:
// 			n.logger.Warn().Str("client_id", client.ID).Msg("Client event channel full, dropping event")
// 			n.metrics.EventsDroppedTotal.Inc()
// 		}
// 	}
// }

// removeClient removes a client from the manager
func (n *Notifier) removeClient(clientID string) {
	n.mu.Lock()
	client, ok := n.clients[clientID]
	if ok {
		delete(n.clients, clientID)
		n.mu.Unlock() // Unlock earlier to avoid deadlock with broadcastBuffer

		// Close connection/channels
		if client.conn != nil {
			client.conn.Close()
		}
		if client.sseChannel != nil {
			// Closing eventsChannel signals the SSE goroutine to close sseChannel
		}
		if client.eventsChannel != nil {
			// Unsubscribe from broadcast buffer first
			n.broadcastBuffer.Unsubscribe(clientID)
			// Closing the channel signals read loops to stop
			// Note: Closing already closed channel is a no-op
			// close(client.eventsChannel) // Let broadcast buffer handle this via Unsubscribe
		}

		// Unsubscribe from the main router if needed
		// if client.SubscriptionID != "" {
		// 	n.router.Unsubscribe(client.SubscriptionID)
		// }

		n.metrics.ConnectedClients.Dec()
		n.logger.Info().Str("client_id", clientID).Msg("Client removed")
	} else {
		n.mu.Unlock()
		n.logger.Debug().Str("client_id", clientID).Msg("Attempted to remove non-existent client")
	}
}

// cleanupIdleClients periodically checks for and removes idle clients
func (n *Notifier) cleanupIdleClients(ctx context.Context) {
	ticker := time.NewTicker(n.config.MaxIdleTime / 2) // Check more frequently than timeout
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.performClientCleanup()
		case <-ctx.Done():
			n.logger.Info().Msg("Stopping idle client cleanup routine")
			return
		}
	}
}

// performClientCleanup iterates through clients and removes idle ones
func (n *Notifier) performClientCleanup() {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()
	clientIDsToRemove := []string{}

	for clientID, client := range n.clients {
		client.mu.Lock() // Lock individual client for reading LastActive
		idleDuration := now.Sub(client.LastActive)
		client.mu.Unlock()

		if idleDuration > n.config.MaxIdleTime {
			n.logger.Info().Str("client_id", clientID).Dur("idle_duration", idleDuration).Msg("Client timed out due to inactivity")
			clientIDsToRemove = append(clientIDsToRemove, clientID)
		}
	}

	// Remove clients outside the iteration loop
	// Need to release the main notifier lock before calling removeClient
	n.mu.Unlock()
	for _, clientID := range clientIDsToRemove {
		n.removeClient(clientID) // removeClient handles its own locking
	}
	n.mu.Lock() // Re-acquire lock before function exits
}

// sendHeartbeats periodically sends heartbeat messages to WebSocket clients
func (n *Notifier) sendHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()

	heartbeatMsg := fiber.Map{"type": "heartbeat"}

	for {
		select {
		case <-ticker.C:
			n.mu.RLock()
			clientsToSend := make([]*Client, 0, len(n.clients))
			for _, client := range n.clients {
				if !client.isSSE && client.conn != nil { // Only send to WebSocket clients
					clientsToSend = append(clientsToSend, client)
				}
			}
			n.mu.RUnlock()

			for _, client := range clientsToSend {
				if err := client.conn.WriteJSON(heartbeatMsg); err != nil {
					n.logger.Warn().Err(err).Str("client_id", client.ID).Msg("Failed to send heartbeat, assuming client disconnected")
					// Trigger removal in a separate goroutine to avoid blocking heartbeat loop
					go n.removeClient(client.ID)
				}
			}
		case <-ctx.Done():
			n.logger.Info().Msg("Stopping heartbeat routine")
			return
		}
	}
}

// Shutdown performs cleanup and stops the notifier
func (n *Notifier) Shutdown(ctx context.Context) error {
	n.logger.Info().Msg("Shutting down event notifier")

	n.mu.Lock()
	defer n.mu.Unlock()

	// Close broadcast buffer first
	n.broadcastBuffer.Close()

	// Close all client connections and channels
	for _, client := range n.clients {
		if client.conn != nil {
			client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down"))
			client.conn.Close()
		}
		// Channels will be closed by broadcastBuffer.Close() propagating
		// or by removeClient if called explicitly

		// Unsubscribe from router if tracking subscriptions explicitly
		// if client.SubscriptionID != "" {
		// 	n.router.Unsubscribe(client.SubscriptionID)
		// }
	}

	n.clients = make(map[string]*Client) // Clear clients map
	n.logger.Info().Msg("Notifier shutdown complete")
	return nil
}

// generateID creates a unique client ID
func generateID() string {
	return uuid.NewString()
}
