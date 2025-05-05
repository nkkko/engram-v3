package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nkkko/engram-v3/apps/mcp/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// JSONRPC message structure
type Message struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error object for JSONRPC
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Main server state
type MCPServer struct {
	name          string
	version       string
	storageClient internal.StorageClient
	toolHandler   *internal.MCPToolHandler
	contextID     string
	agentID       string
	initialized   bool
	lock          sync.Mutex
	// compatMode indicates if running in Claude Desktop compatibility mode
	compatMode    bool
}

func main() {
	// Set up debug logging to stderr
	fmt.Fprintf(os.Stderr, "DEBUG: Starting Engram MCP server\n")

	// Check for Claude Desktop mode flag
	claudeDesktopMode := os.Getenv("ENGRAM_CLAUDE_DESKTOP_MODE") == "1"
	if claudeDesktopMode {
		fmt.Fprintf(os.Stderr, "DEBUG: Running in Claude Desktop compatibility mode\n")
	}

	// Get directories from environment or use defaults
	logDir := os.Getenv("ENGRAM_LOG_DIR")
	if logDir == "" {
		logDir = "./logs"
	}

	dataDir := os.Getenv("ENGRAM_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Configure logging - fallback to stderr if file logging fails
	logWriter := os.Stderr

	// Try to setup file logging, but don't fail if it's not possible
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logFile, err := os.OpenFile(filepath.Join(logDir, "mcp-server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logWriter = logFile
			// Don't close logFile - it will be used throughout the program's lifetime
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to open log file, using stderr: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to create log directory %s: %v\n", logDir, err)
	}

	// Setup logger to write to the selected output
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(logWriter).With().Timestamp().Logger()

	// Initialize storage with fallback to in-memory mode
	var storage internal.StorageClient

	// Try to create data directory, but don't fail if not possible
	if createErr := os.MkdirAll(dataDir, 0755); createErr != nil {
		log.Warn().Err(createErr).Str("dir", dataDir).Msg("Failed to create data directory, using in-memory storage")
		storage = internal.NewMemoryStorageClient()
	} else {
		// Initialize BadgerDB storage
		var initErr error
		storage, initErr = internal.NewBadgerStorageClient(dataDir)
		if initErr != nil {
			log.Warn().Err(initErr).Msg("Failed to initialize BadgerDB storage, falling back to in-memory storage")
			storage = internal.NewMemoryStorageClient()
		}
	}
	defer storage.Close()

	// Initialize tool handler
	toolHandler := internal.NewMCPToolHandler(storage)

	// Create server instance
	server := &MCPServer{
		name:          "Engram Memory",
		version:       "0.1.0",
		storageClient: storage,
		toolHandler:   toolHandler,
		compatMode:    claudeDesktopMode,
	}

	// Run the server
	server.run()
}

func (s *MCPServer) run() {
	// Use unbuffered stdout for direct writing
	// This approach avoids potential buffer issues with Claude Desktop
	writer := os.Stdout

	// Print initial debug info
	fmt.Fprintf(os.Stderr, "DEBUG: Server starting, waiting for input...\n")
	fmt.Fprintf(os.Stderr, "DEBUG: Using unbuffered writer for direct output\n")

	// Create a signal channel to keep the program alive
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a long-lived heartbeat timer to keep the server alive
	// even if there is no communication from the client
	heartbeatTicker := time.NewTicker(60 * time.Second)
	defer heartbeatTicker.Stop()

	// Set up a channel for incoming messages
	msgChan := make(chan string, 10)
	errChan := make(chan error, 1)

	// Start a goroutine to read from stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		// Set a larger buffer size
		const maxCapacity = 10 * 1024 * 1024 // 10MB
		buf := make([]byte, 0, maxCapacity)
		scanner.Buffer(buf, maxCapacity)

		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				msgChan <- line
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- err
		} else {
			errChan <- io.EOF
		}
	}()

	// Add a timeout for initialization - if client doesn't send 'initialized' notification,
	// we'll proceed anyway
	initTimeoutTimer := time.NewTimer(5 * time.Second)
	defer initTimeoutTimer.Stop()

	// Mark if we've been initialized
	s.lock.Lock()
	wasAlreadyInitialized := s.initialized
	s.lock.Unlock()

	// Main server loop
	running := true
	for running {
		select {
       case sig := <-sigChan:
           // Handle interrupt signal
           s.lock.Lock()
           initDone := s.initialized
           s.lock.Unlock()
           if s.compatMode && !initDone {
               fmt.Fprintf(os.Stderr, "DEBUG: Received signal %v during initialization, ignoring due to compatibility mode\n", sig)
               continue
           }
           fmt.Fprintf(os.Stderr, "DEBUG: Received interrupt signal, terminating server\n")
           running = false

       case err := <-errChan:
           // Handle read errors
           s.lock.Lock()
           initDone := s.initialized
           s.lock.Unlock()
           if err == io.EOF {
               if s.compatMode && !initDone {
                   fmt.Fprintf(os.Stderr, "DEBUG: EOF reached during initialization, ignoring due to compatibility mode\n")
                   continue
               }
               fmt.Fprintf(os.Stderr, "DEBUG: EOF reached, client closed connection\n")
           } else {
               if s.compatMode && !initDone {
                   fmt.Fprintf(os.Stderr, "ERROR: Failed to read from stdin: %v. Ignoring due to compatibility mode\n", err)
                   continue
               }
               fmt.Fprintf(os.Stderr, "ERROR: Failed to read from stdin: %v\n", err)
           }
           running = false

		case line := <-msgChan:
			// Handle incoming message
			fmt.Fprintf(os.Stderr, "DEBUG: Received: %s\n", line)

			// Parse the message
			var message Message
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				// Send error response for parsing errors
				fmt.Fprintf(os.Stderr, "ERROR: Failed to parse message: %v\n", err)
				s.sendError(writer, nil, -32700, "Parse error", err.Error())
				continue
			}

			// Handle the message
			s.handleMessage(writer, message)

			// After handling the message, check if we should exit
			if message.Method == "shutdown" {
				fmt.Fprintf(os.Stderr, "DEBUG: Received shutdown request, terminating server\n")
				// Send an optional shutdown response if ID is present
				if message.ID != nil {
					s.sendResponse(writer, message.ID, nil)
				}
				running = false
			}

			// If this is the initialize message, reset the timeout timer
			if message.Method == "initialize" {
				initTimeoutTimer.Reset(5 * time.Second)
			}

			// If this is the initialized notification, mark as initialized
			if message.Method == "initialized" || message.Method == "notifications/initialized" {
				s.lock.Lock()
				s.initialized = true
				s.lock.Unlock()

				// Stop the initialization timeout
				if !initTimeoutTimer.Stop() {
					<-initTimeoutTimer.C
				}
			}

       case <-initTimeoutTimer.C:
           // If we haven't received an 'initialized' notification after timeout,
           // assume we're initialized anyway and continue
           s.lock.Lock()
           if !s.initialized && !wasAlreadyInitialized {
               s.initialized = true
               fmt.Fprintf(os.Stderr, "DEBUG: No 'initialized' notification received after timeout, assuming initialized anyway\n")
               // In compatibility mode, send initial tools list notification after timeout
               if s.compatMode {
                   tools := internal.GetToolDefinitions()
                   notif := Message{
                       JSONRPC: "2.0",
                       Method:  "notifications/tools/list_changed",
                       Params:  map[string]interface{}{ "tools": tools },
                   }
                   fmt.Fprintf(os.Stderr, "DEBUG: Sending tools list notification after timeout: %+v\n", tools)
                   s.sendMessage(writer, notif)
               }
           }
           s.lock.Unlock()

		case <-heartbeatTicker.C:
			// Just a heartbeat to keep us alive, no action needed
			fmt.Fprintf(os.Stderr, "DEBUG: Server heartbeat - still running\n")
		}
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Server shutting down\n")
}

func (s *MCPServer) handleMessage(writer io.Writer, msg Message) {
	// Dispatch based on method
	switch msg.Method {
	case "initialize":
		s.handleInitialize(writer, msg)
	case "initialized", "notifications/initialized":
		s.handleInitialized(writer, msg)
	case "tools/list":
		s.handleToolsList(writer, msg)
	case "tools/call":
		s.handleToolsCall(writer, msg)
	case "shutdown":
		s.handleShutdown(writer, msg)
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Unsupported method: %s\n", msg.Method)
		s.sendError(writer, msg.ID, -32601, "Method not found", fmt.Sprintf("Method '%s' not supported", msg.Method))
	}
}

// handleShutdown handles the shutdown request
func (s *MCPServer) handleShutdown(writer io.Writer, msg Message) {
	fmt.Fprintf(os.Stderr, "DEBUG: Received shutdown request, terminating server\n")

	// Send an empty response for the shutdown request
	if msg.ID != nil {
		s.sendResponse(writer, msg.ID, nil)
	}
}

func (s *MCPServer) handleInitialize(writer io.Writer, msg Message) {
	// Parse the parameters according to MCP protocol
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
		Capabilities map[string]interface{} `json:"capabilities"`
		// The following are MCP extensions that might be present
		ContextID string `json:"contextId"`
		AgentID   string `json:"agentId"`
	}

	// Log the raw params for debugging Claude Desktop issues
	rawParamsData, _ := json.Marshal(msg.Params)
	fmt.Fprintf(os.Stderr, "DEBUG: Raw initialize params: %s\n", string(rawParamsData))

	if err := parseParams(msg.Params, &params); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to parse initialize parameters: %v\n", err)
		s.sendError(writer, msg.ID, -32602, "Invalid params", err.Error())
		return
	}

	// Store context ID and agent ID
	s.lock.Lock()
	if params.ContextID != "" {
		s.contextID = params.ContextID
	} else {
		// Generate a new context ID if not provided
		s.contextID = uuid.New().String()
		fmt.Fprintf(os.Stderr, "DEBUG: Generated new context ID: %s\n", s.contextID)
	}

	if params.AgentID != "" {
		s.agentID = params.AgentID
	} else {
		// Use client name as agent ID if not provided
		s.agentID = params.ClientInfo.Name
		fmt.Fprintf(os.Stderr, "DEBUG: Using client name as agent ID: %s\n", s.agentID)
	}
	s.lock.Unlock()

	// Log client info and protocol version
	fmt.Fprintf(os.Stderr, "DEBUG: Client connected - %s %s (Protocol: %s)\n",
		params.ClientInfo.Name, params.ClientInfo.Version, params.ProtocolVersion)

	// Prepare initialize response according to MCP protocol
	// Include protocol version, advertise support for tools and listChanged notifications
	response := map[string]interface{}{
		"protocolVersion": params.ProtocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{ "listChanged": true },
		},
		"serverInfo": map[string]interface{}{
			"name":    s.name,
			"version": s.version,
		},
	}

	// Send response
	s.sendResponse(writer, msg.ID, response)

	// Important: After initialize, wait for the client to send "initialized" notification
	fmt.Fprintf(os.Stderr, "DEBUG: Initialization response sent, waiting for 'initialized' notification\n")
	// Note: If running in Claude Desktop mode, we may not receive this notification
	// and will proceed after the timeout expires
   fmt.Fprintf(os.Stderr, "DEBUG: If no 'initialized' notification is received within 5 seconds, server will proceed anyway\n")

}

func (s *MCPServer) handleInitialized(writer io.Writer, msg Message) {
	// Mark as initialized
	s.lock.Lock()
	s.initialized = true
	s.lock.Unlock()

	fmt.Fprintf(os.Stderr, "DEBUG: Server initialized - ready to handle tool requests\n")

   // No response needed for notifications (method calls without an ID)
   // This is important: MCP notifications don't expect responses

   // In compatibility mode, send initial tools list notification after initialization
   if s.compatMode {
       tools := internal.GetToolDefinitions()
       notif := Message{
           JSONRPC: "2.0",
           Method:  "notifications/tools/list_changed",
           Params:  map[string]interface{}{ "tools": tools },
       }
       fmt.Fprintf(os.Stderr, "DEBUG: Sending tools list notification after initialized: %+v\n", tools)
       s.sendMessage(writer, notif)
   }
}

func (s *MCPServer) handleToolsList(writer io.Writer, msg Message) {
	// Get tool definitions
	tools := internal.GetToolDefinitions()

	// Send response
	s.sendResponse(writer, msg.ID, map[string]interface{}{
		"tools": tools,
	})
}

func (s *MCPServer) handleToolsCall(writer io.Writer, msg Message) {
	// Parse parameters
	var params struct {
		Name       string                 `json:"name"`
		Parameters map[string]interface{} `json:"parameters,omitempty"`
		Arguments  map[string]interface{} `json:"arguments,omitempty"`
	}

	if err := parseParams(msg.Params, &params); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to parse tool call parameters: %v\n", err)
		s.sendError(writer, msg.ID, -32602, "Invalid params", err.Error())
		return
	}

	// Determine argument map (supports both "parameters" and legacy "arguments")
	var argMap map[string]interface{}
	if params.Parameters != nil {
		argMap = params.Parameters
	} else {
		argMap = params.Arguments
	}

	// Validate tool name
	if params.Name == "" {
		s.sendError(writer, msg.ID, -32602, "Invalid params", "Missing tool name")
		return
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Tool call: %s\n", params.Name)

	// Handle tool call
	result := s.toolHandler.HandleToolCall(
		params.Name,
		s.contextID,
		s.agentID,
		argMap,
	)

	// Convert ToolResult to MCP result structure
	if result.IsError {
		mcpResult := map[string]interface{}{
			"isError": true,
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": result.Message,
				},
			},
		}
		s.sendResponse(writer, msg.ID, mcpResult)
	} else {
		// Marshal the data to a JSON string for human-readable text content
		jsonBytes, _ := json.MarshalIndent(result.Data, "", "  ")
		mcpResult := map[string]interface{}{
			"isError": false,
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": string(jsonBytes),
				},
			},
		}
		s.sendResponse(writer, msg.ID, mcpResult)
	}
}

// Helper functions

func parseParams(params interface{}, target interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

func (s *MCPServer) sendResponse(writer io.Writer, id interface{}, result interface{}) {
	response := Message{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	s.sendMessage(writer, response)
}

func (s *MCPServer) sendError(writer io.Writer, id interface{}, code int, message, data string) {
	response := Message{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	s.sendMessage(writer, response)
}

func (s *MCPServer) sendMessage(writer io.Writer, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to marshal message: %v\n", err)
		return
	}

	// Log outgoing message
	fmt.Fprintf(os.Stderr, "DEBUG: Sending: %s\n", string(data))

	// Use os.Stdout directly for response sending
	// This ensures responses go directly to the client without potential buffer issues
	fmt.Fprintf(os.Stderr, "DEBUG: Writing response to os.Stdout directly\n")
	fmt.Fprintln(os.Stdout, string(data))

	// Force os.Stdout to flush immediately
	os.Stdout.Sync()

	fmt.Fprintf(os.Stderr, "DEBUG: Response sent successfully\n")
}