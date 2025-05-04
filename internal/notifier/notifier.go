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
	"github.com/nkkko/engram-v3/internal/router"
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
	ID           string
	ContextIDs   []string             // Contexts the client is subscribed to
	LastActive   time.Time
	conn         *websocket.Conn
	sseChannel   chan []byte
	eventsChannel chan *proto.WorkUnit // Channel for receiving events
	isSSE        bool
	mu           sync.Mutex
}

// Notifier handles real-time notifications to clients
type Notifier struct {
	config            Config
	clients           map[string]*Client
	mu                sync.RWMutex
	logger            zerolog.Logger
	heartbeatInterval time.Duration
	router            *router.Router
	broadcastBuffer   *BroadcastBuffer
	metrics           *metrics.Metrics
}

// NewNotifier creates a new notification manager
func NewNotifier(config Config, r *router.Router) *Notifier {
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
func (n *Notifier) Start(ctx context.Context, subscription *router.Subscription) error {
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
		contextID := c.Query("context", "")
		if contextID == "" {
			c.WriteJSON(fiber.Map{
				"error": "Context ID is required",
			})
			c.Close()
			return
		}
		
		n.handleWebSocketClient(c, contextID)
	}))
}

// RegisterSSEHandler registers the Server-Sent Events handler with a Fiber app
func (n *Notifier) RegisterSSEHandler(app *fiber.App) {
	app.Get("/stream-sse", func(c *fiber.Ctx) error {
		// Get context ID from query parameters
		contextID := c.Query("context", "")
		if contextID == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "Context ID is required",
			})
		}
		
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
		ID:           clientID,
		ContextIDs:   []string{contextID},
		LastActive:   time.Now(),
		conn:         conn,
		eventsChannel: eventsChannel,
		isSSE:        false,
	}
	
	// Register client
	n.mu.Lock()
	n.clients[clientID] = client
	n.mu.Unlock()
	
	// Handle client messages
	go func() {
		defer n.removeClient(clientID)
		
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				n.logger.Debug().Err(err).Str("client_id", clientID).Msg("WebSocket read error")
				break
			}
			
			// Update last active time
			client.mu.Lock()
			client.LastActive = time.Now()
			client.mu.Unlock()
			
			// Process client message (e.g., change subscription contexts)
			if messageType == websocket.TextMessage {
				n.processClientMessage(client, message)
			}
		}
	}()
	
	// Send events to client
	go func() {
		for event := range eventsChannel {
			// Check if this event is for one of the client's contexts
			isRelevant := false
			for _, cid := range client.ContextIDs {
				if event.ContextId == cid {
					isRelevant = true
					break
				}
			}
			
			if !isRelevant {
				continue
			}
			
			// Convert event to JSON
			jsonData, err := json.Marshal(event)
			if err != nil {
				n.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to marshal event")
				continue
			}
			
			// Send to client
			if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
				n.logger.Debug().Err(err).Str("client_id", clientID).Msg("WebSocket write error")
				break
			}
			
			// Track event delivery metrics
			n.metrics.NotifierEventsPublished.WithLabelValues("websocket").Inc()
		}
	}()
}

// createSSEClient creates a new SSE client
func (n *Notifier) createSSEClient(contextID string) *Client {
	// Create client
	clientID := generateID()
	
	// Create events channel and subscribe to broadcast buffer
	eventsChannel := n.broadcastBuffer.Subscribe(clientID, 100)
	
	client := &Client{
		ID:           clientID,
		ContextIDs:   []string{contextID},
		LastActive:   time.Now(),
		sseChannel:   make(chan []byte, 100),
		eventsChannel: eventsChannel,
		isSSE:        true,
	}
	
	// Register client
	n.mu.Lock()
	n.clients[clientID] = client
	n.mu.Unlock()
	
	// Process events and send to client
	go func() {
		for event := range eventsChannel {
			// Check if this event is for one of the client's contexts
			isRelevant := false
			for _, cid := range client.ContextIDs {
				if event.ContextId == cid {
					isRelevant = true
					break
				}
			}
			
			if !isRelevant {
				continue
			}
			
			// Convert event to JSON
			jsonData, err := json.Marshal(event)
			if err != nil {
				n.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to marshal event")
				continue
			}
			
			// Send to client's SSE channel
			select {
			case client.sseChannel <- jsonData:
				// Successfully sent
				n.metrics.NotifierEventsPublished.WithLabelValues("sse").Inc()
			default:
				// Channel buffer full, log warning
				n.logger.Warn().Str("client_id", clientID).Msg("SSE channel buffer full, dropping event")
			}
		}
	}()
	
	return client
}

// processClientMessage handles messages from clients
func (n *Notifier) processClientMessage(client *Client, message []byte) {
	var request struct {
		Action     string   `json:"action"`
		ContextIDs []string `json:"context_ids,omitempty"`
	}
	
	if err := json.Unmarshal(message, &request); err != nil {
		n.logger.Error().Err(err).Str("client_id", client.ID).Msg("Failed to parse client message")
		return
	}
	
	switch request.Action {
	case "subscribe":
		// Update client's context IDs
		if len(request.ContextIDs) > 0 {
			client.mu.Lock()
			client.ContextIDs = request.ContextIDs
			client.mu.Unlock()
			
			n.logger.Debug().
				Str("client_id", client.ID).
				Strs("context_ids", request.ContextIDs).
				Msg("Client updated subscription contexts")
		}
		
	case "ping":
		// Client is just keeping the connection alive, no action needed
		
	default:
		n.logger.Debug().
			Str("client_id", client.ID).
			Str("action", request.Action).
			Msg("Unknown client action")
	}
}

// broadcastEvent sends an event to all interested clients
func (n *Notifier) broadcastEvent(event *proto.WorkUnit) {
	// Publish to the broadcast buffer for zero-copy distribution
	n.broadcastBuffer.Publish(event)
}

// removeClient removes a client
func (n *Notifier) removeClient(clientID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	client, exists := n.clients[clientID]
	if !exists {
		return
	}
	
	// Clean up resources
	if client.isSSE {
		close(client.sseChannel)
	} else if client.conn != nil {
		client.conn.Close()
	}
	
	// Unsubscribe from broadcast buffer
	n.broadcastBuffer.Unsubscribe(clientID)
	
	// Update metrics
	n.metrics.NotifierConnectionsActive.Dec()
	
	// Remove from clients map
	delete(n.clients, clientID)
	
	n.logger.Debug().Str("client_id", clientID).Msg("Client removed")
}

// cleanupIdleClients periodically removes idle clients
func (n *Notifier) cleanupIdleClients(ctx context.Context) {
	ticker := time.NewTicker(n.config.MaxIdleTime / 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			n.performClientCleanup()
		case <-ctx.Done():
			return
		}
	}
}

// performClientCleanup removes clients that have been idle for too long
func (n *Notifier) performClientCleanup() {
	now := time.Now()
	var idleClients []string
	
	n.mu.RLock()
	for id, client := range n.clients {
		client.mu.Lock()
		lastActive := client.LastActive
		client.mu.Unlock()
		
		if now.Sub(lastActive) > n.config.MaxIdleTime {
			idleClients = append(idleClients, id)
		}
	}
	n.mu.RUnlock()
	
	// Remove idle clients
	for _, id := range idleClients {
		n.removeClient(id)
		n.logger.Debug().Str("client_id", id).Msg("Removed idle client")
	}
}

// sendHeartbeats periodically sends heartbeat messages to clients
func (n *Notifier) sendHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()
	
	heartbeat := []byte(`{"type":"heartbeat","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)
	
	for {
		select {
		case <-ticker.C:
			n.mu.RLock()
			for _, client := range n.clients {
				if client.isSSE {
					select {
					case client.sseChannel <- []byte(`{"type":"heartbeat"}`):
						// Heartbeat sent
					default:
						// Channel full, skip
					}
				} else if client.conn != nil {
					// Update heartbeat timestamp
					heartbeat = []byte(`{"type":"heartbeat","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)
					client.conn.WriteMessage(websocket.TextMessage, heartbeat)
				}
			}
			n.mu.RUnlock()
			
		case <-ctx.Done():
			return
		}
	}
}

// Shutdown performs cleanup and stops the notifier
func (n *Notifier) Shutdown(ctx context.Context) error {
	n.logger.Info().Msg("Shutting down notifier")
	
	// Close broadcast buffer
	if n.broadcastBuffer != nil {
		if err := n.broadcastBuffer.Close(); err != nil {
			n.logger.Error().Err(err).Msg("Error closing broadcast buffer")
		}
	}
	
	// Close all client connections
	n.mu.Lock()
	defer n.mu.Unlock()
	
	for id, client := range n.clients {
		if client.isSSE {
			close(client.sseChannel)
		} else if client.conn != nil {
			client.conn.Close()
		}
		
		delete(n.clients, id)
	}
	
	n.logger.Info().Int("closed_clients", len(n.clients)).Msg("All client connections closed")
	n.clients = make(map[string]*Client)
	
	return nil
}

// generateID creates a unique client ID
func generateID() string {
	return uuid.NewString()
}