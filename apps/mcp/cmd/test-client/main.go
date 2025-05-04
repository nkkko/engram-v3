package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// JSONRPC message structure (same as server)
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

func main() {
	// Start the MCP server
	cmd := exec.Command("go", "run", "../../main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("Error creating stdin pipe: %v\n", err)
		os.Exit(1)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}

	// Setup reader for server responses
	reader := bufio.NewReader(stdout)
	
	// Handle server output in a separate goroutine
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading from server: %v\n", err)
				}
				break
			}
			
			// Parse and print server response
			var msg Message
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				fmt.Printf("Error parsing response: %v\nRaw response: %s\n", err, line)
				continue
			}
			
			// Pretty print the response
			prettyJSON, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Printf("\nServer response:\n%s\n", string(prettyJSON))
		}
	}()

	// Send initialize message
	initMsg := Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"clientName":    "MCP Test Client",
			"clientVersion": "0.1.0",
			"contextId":     "test-context-1",
			"agentId":       "test-agent-1",
		},
	}
	
	sendTestMessage(stdin, initMsg)
	time.Sleep(500 * time.Millisecond)
	
	// Send initialized notification
	initNotify := Message{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}
	
	sendTestMessage(stdin, initNotify)
	time.Sleep(500 * time.Millisecond)
	
	// Request tools list
	toolsListMsg := Message{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}
	
	sendTestMessage(stdin, toolsListMsg)
	time.Sleep(500 * time.Millisecond)
	
	// Store a memory
	storeMemoryMsg := Message{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "store_memory",
			"parameters": map[string]interface{}{
				"content": "This is a test memory stored by the MCP test client.",
				"metadata": map[string]interface{}{
					"importance": "high",
					"category":   "test",
				},
			},
		},
	}
	
	sendTestMessage(stdin, storeMemoryMsg)
	time.Sleep(500 * time.Millisecond)
	
	// List recent messages
	listMessagesMsg := Message{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "list_recent_messages",
			"parameters": map[string]interface{}{
				"limit": 10,
			},
		},
	}
	
	sendTestMessage(stdin, listMessagesMsg)
	time.Sleep(500 * time.Millisecond)
	
	// Search conversation
	searchMsg := Message{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "search_conversation",
			"parameters": map[string]interface{}{
				"query": "test",
				"limit": 5,
			},
		},
	}
	
	sendTestMessage(stdin, searchMsg)
	time.Sleep(1 * time.Second)
	
	// Terminate server
	cmd.Process.Kill()
	fmt.Println("\nTest completed.")
}

// Helper function to send a message to the server
func sendTestMessage(w io.Writer, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error marshaling message: %v\n", err)
		return
	}
	
	fmt.Printf("\nSending message:\n%s\n", string(data))
	
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	}
}