package internal

import (
	"fmt"
)

// Tool definitions
const (
	ToolSearchConversation = "search_conversation"
	ToolStoreMemory        = "store_memory"
	ToolRetrieveMemory     = "retrieve_memory"
	ToolListRecentMessages = "list_recent_messages"
)

// ToolDefinition represents a tool's schema and metadata
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	IsError bool        `json:"isError"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// GetToolDefinitions returns all tools supported by the MCP server
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolSearchConversation,
			Description: "Search for content within the conversation history",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query text",
					},
					"types": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Optional filter by message types (e.g., user, assistant, memory)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default 10)",
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Results to skip (default 0)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        ToolStoreMemory,
			Description: "Store a memory or note for future reference",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to store",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Optional metadata to associate with the memory",
					},
				},
				"required": []string{"content"},
			},
		},
		{
			Name:        ToolRetrieveMemory,
			Description: "Retrieve a specific memory by ID",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the memory to retrieve",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        ToolListRecentMessages,
			Description: "List recent messages in the conversation",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of messages to return (default 10)",
					},
					"types": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Optional filter by message types (e.g., user, assistant, memory)",
					},
				},
			},
		},
	}
}

// MCPToolHandler handles tool requests for the MCP server
type MCPToolHandler struct {
	storage StorageClient
}

// NewMCPToolHandler creates a new tool handler
func NewMCPToolHandler(storage StorageClient) *MCPToolHandler {
	return &MCPToolHandler{
		storage: storage,
	}
}

// HandleToolCall processes a tool call and returns the result
func (h *MCPToolHandler) HandleToolCall(toolName string, contextID string, agentID string, params map[string]interface{}) *ToolResult {
	switch toolName {
	case ToolSearchConversation:
		return h.handleSearchConversation(contextID, params)
	case ToolStoreMemory:
		return h.handleStoreMemory(contextID, agentID, params)
	case ToolRetrieveMemory:
		return h.handleRetrieveMemory(params)
	case ToolListRecentMessages:
		return h.handleListRecentMessages(contextID, params)
	default:
		return &ToolResult{
			IsError: true,
			Message: fmt.Sprintf("Unknown tool: %s", toolName),
		}
	}
}

// handleSearchConversation implements the search_conversation tool
func (h *MCPToolHandler) handleSearchConversation(contextID string, params map[string]interface{}) *ToolResult {
	// Parse parameters
	query, _ := params["query"].(string)
	if query == "" {
		return &ToolResult{
			IsError: true,
			Message: "Query parameter is required",
		}
	}

	// Parse optional parameters
	limit := 10
	if limitParam, ok := params["limit"].(float64); ok {
		limit = int(limitParam)
	}

	offset := 0
	if offsetParam, ok := params["offset"].(float64); ok {
		offset = int(offsetParam)
	}

	// Parse types filter
	var types []string
	if typesParam, ok := params["types"].([]interface{}); ok {
		for _, t := range typesParam {
			if typeStr, ok := t.(string); ok {
				types = append(types, typeStr)
			}
		}
	}

	// Create search query
	searchQuery := SearchQuery{
		ContextID: contextID,
		Query:     query,
		Types:     types,
		Limit:     limit,
		Offset:    offset,
	}

	// Execute search
	results, err := h.storage.SearchMessages(searchQuery)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Message: fmt.Sprintf("Search failed: %v", err),
		}
	}

	// Return results
	return &ToolResult{
		IsError: false,
		Data:    results,
	}
}

// handleStoreMemory implements the store_memory tool
func (h *MCPToolHandler) handleStoreMemory(contextID string, agentID string, params map[string]interface{}) *ToolResult {
	// Parse parameters
	content, _ := params["content"].(string)
	if content == "" {
		return &ToolResult{
			IsError: true,
			Message: "Content parameter is required",
		}
	}

	// Parse optional metadata
	metadata := make(map[string]interface{})
	if metaParam, ok := params["metadata"].(map[string]interface{}); ok {
		metadata = metaParam
	}

	// Create memory message
	memory := NewConversationMessage(contextID, agentID, MessageTypeMemory, content)
	memory.Metadata = metadata

	// Store the memory
	err := h.storage.StoreMessage(memory)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Message: fmt.Sprintf("Failed to store memory: %v", err),
		}
	}

	// Return success with memory ID
	return &ToolResult{
		IsError: false,
		Data: map[string]interface{}{
			"id":        memory.ID,
			"timestamp": memory.Timestamp,
		},
	}
}

// handleRetrieveMemory implements the retrieve_memory tool
func (h *MCPToolHandler) handleRetrieveMemory(params map[string]interface{}) *ToolResult {
	// Parse parameters
	id, _ := params["id"].(string)
	if id == "" {
		return &ToolResult{
			IsError: true,
			Message: "ID parameter is required",
		}
	}

	// Retrieve the memory
	memory, err := h.storage.GetMessage(id)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Message: fmt.Sprintf("Failed to retrieve memory: %v", err),
		}
	}

	// Return the memory
	return &ToolResult{
		IsError: false,
		Data:    memory,
	}
}

// handleListRecentMessages implements the list_recent_messages tool
func (h *MCPToolHandler) handleListRecentMessages(contextID string, params map[string]interface{}) *ToolResult {
	// Parse optional parameters
	limit := 10
	if limitParam, ok := params["limit"].(float64); ok {
		limit = int(limitParam)
	}

	// Parse types filter
	var types []string
	if typesParam, ok := params["types"].([]interface{}); ok {
		for _, t := range typesParam {
			if typeStr, ok := t.(string); ok {
				types = append(types, typeStr)
			}
		}
	}

	// Get recent messages
	messages, err := h.storage.GetRecentMessages(contextID, limit)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Message: fmt.Sprintf("Failed to retrieve messages: %v", err),
		}
	}

	// Filter by type if needed
	if len(types) > 0 {
		filtered := make([]ConversationMessage, 0, len(messages))
		for _, msg := range messages {
			for _, t := range types {
				if msg.Type == t {
					filtered = append(filtered, msg)
					break
				}
			}
		}
		messages = filtered
	}

	// Return messages
	return &ToolResult{
		IsError: false,
		Data:    messages,
	}
}