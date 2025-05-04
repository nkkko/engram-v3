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
	fmt.Println("Starting Claude Desktop integration test...")

	// Start the MCP server - using absolute path to the binary
	cmd := exec.Command("/Users/nikola/dev/codex/codex-cli/engram-v3/bin/mcp-server")
	
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

	// Redirect stderr to our stdout for debugging
	cmd.Stderr = os.Stdout

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
			
			// Print raw response first
			fmt.Printf("Raw response: %s", line)
			
			// Try to parse and pretty print
			var msg Message
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				fmt.Printf("Error parsing response: %v\n", err)
				continue
			}
			
			// Pretty print the response
			prettyJSON, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Printf("\nServer response (pretty):\n%s\n", string(prettyJSON))
		}
	}()

	// Send initialize message (using Claude Desktop format)
	initMsg := Message{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "claude-ai",
				"version": "0.1.0",
			},
		},
	}
	
	sendTestMessage(stdin, initMsg)
	fmt.Println("Initialize message sent, waiting...")
	time.Sleep(1 * time.Second)
	
	// Send initialized notification
	initNotify := Message{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}
	
	sendTestMessage(stdin, initNotify)
	fmt.Println("Initialized notification sent, waiting...")
	time.Sleep(1 * time.Second)
	
	// Request tools list
	toolsListMsg := Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}
	
	sendTestMessage(stdin, toolsListMsg)
	fmt.Println("Tools list requested, waiting...")
	time.Sleep(1 * time.Second)
	
	// Store a memory
	storeMemoryMsg := Message{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "store_memory",
			"parameters": map[string]interface{}{
				"content": "This memory was created in the Claude Desktop integration test.",
				"metadata": map[string]interface{}{
					"source":   "claude-test",
					"priority": "high",
				},
			},
		},
	}
	
	sendTestMessage(stdin, storeMemoryMsg)
	fmt.Println("Memory stored, waiting...")
	time.Sleep(1 * time.Second)
	
	// Keep the server running for 5 seconds to make sure it stays alive
	fmt.Println("Keeping server alive for 5 seconds...")
	time.Sleep(5 * time.Second)
	
	// Send shutdown message
	shutdownMsg := Message{
		JSONRPC: "2.0",
		ID:      99,
		Method:  "shutdown",
		Params:  map[string]interface{}{},
	}
	
	sendTestMessage(stdin, shutdownMsg)
	fmt.Println("Shutdown message sent, waiting...")
	time.Sleep(1 * time.Second)
	
	// Terminate server if still running
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
	
	// Write the message followed by a newline
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	}
}