package internal

import (
	"testing"
	"time"
)

// MockStorageClient is a mock implementation of the StorageClient interface for testing
type MockStorageClient struct {
	messages      map[string]*ConversationMessage
	searchResults []ConversationMessage
	searchError   error
	storeError    error
	getMessage    *ConversationMessage
	getError      error
	recentMessages []ConversationMessage
	recentError   error
}

// NewMockStorageClient creates a new mock storage client for testing
func NewMockStorageClient() *MockStorageClient {
	return &MockStorageClient{
		messages: make(map[string]*ConversationMessage),
	}
}

func (m *MockStorageClient) StoreMessage(msg *ConversationMessage) error {
	if m.storeError != nil {
		return m.storeError
	}
	m.messages[msg.ID] = msg
	return nil
}

func (m *MockStorageClient) GetRecentMessages(contextID string, limit int) ([]ConversationMessage, error) {
	if m.recentError != nil {
		return nil, m.recentError
	}
	return m.recentMessages, nil
}

func (m *MockStorageClient) SearchMessages(query SearchQuery) ([]ConversationMessage, error) {
	if m.searchError != nil {
		return nil, m.searchError
	}
	return m.searchResults, nil
}

func (m *MockStorageClient) GetMessage(id string) (*ConversationMessage, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if m.getMessage != nil {
		return m.getMessage, nil
	}
	msg, ok := m.messages[id]
	if !ok {
		return nil, nil
	}
	return msg, nil
}

func (m *MockStorageClient) Close() error {
	return nil
}

// Test cases for the tool handler
func TestMCPToolHandler_HandleToolCall(t *testing.T) {
	// Test cases
	testCases := []struct {
		name      string
		toolName  string
		contextID string
		agentID   string
		params    map[string]interface{}
		setup     func(*MockStorageClient)
		validate  func(*testing.T, *ToolResult)
	}{
		{
			name:      "unknown_tool",
			toolName:  "unknown_tool",
			contextID: "test-context",
			agentID:   "test-agent",
			params:    map[string]interface{}{},
			setup:     func(m *MockStorageClient) {},
			validate: func(t *testing.T, result *ToolResult) {
				if !result.IsError {
					t.Errorf("Expected error for unknown tool")
				}
				if result.Message == "" {
					t.Errorf("Expected error message for unknown tool")
				}
			},
		},
		{
			name:      "search_conversation_missing_query",
			toolName:  ToolSearchConversation,
			contextID: "test-context",
			agentID:   "test-agent",
			params:    map[string]interface{}{},
			setup:     func(m *MockStorageClient) {},
			validate: func(t *testing.T, result *ToolResult) {
				if !result.IsError {
					t.Errorf("Expected error for missing query")
				}
				if result.Message != "Query parameter is required" {
					t.Errorf("Expected error message for missing query, got: %s", result.Message)
				}
			},
		},
		{
			name:      "search_conversation_success",
			toolName:  ToolSearchConversation,
			contextID: "test-context",
			agentID:   "test-agent",
			params: map[string]interface{}{
				"query": "test query",
				"limit": float64(5),
			},
			setup: func(m *MockStorageClient) {
				m.searchResults = []ConversationMessage{
					{
						ID:        "msg1",
						ContextID: "test-context",
						AgentID:   "test-agent",
						Type:      MessageTypeUser,
						Content:   "Test message 1",
						Timestamp: time.Now().Unix(),
					},
					{
						ID:        "msg2",
						ContextID: "test-context",
						AgentID:   "test-agent",
						Type:      MessageTypeAssistant,
						Content:   "Test response 1",
						Timestamp: time.Now().Unix(),
					},
				}
			},
			validate: func(t *testing.T, result *ToolResult) {
				if result.IsError {
					t.Errorf("Unexpected error: %s", result.Message)
				}
				
				messages, ok := result.Data.([]ConversationMessage)
				if !ok {
					t.Errorf("Expected result data to be []ConversationMessage")
					return
				}
				
				if len(messages) != 2 {
					t.Errorf("Expected 2 messages, got %d", len(messages))
				}
			},
		},
		{
			name:      "store_memory_missing_content",
			toolName:  ToolStoreMemory,
			contextID: "test-context",
			agentID:   "test-agent",
			params:    map[string]interface{}{},
			setup:     func(m *MockStorageClient) {},
			validate: func(t *testing.T, result *ToolResult) {
				if !result.IsError {
					t.Errorf("Expected error for missing content")
				}
				if result.Message != "Content parameter is required" {
					t.Errorf("Expected error message for missing content, got: %s", result.Message)
				}
			},
		},
		{
			name:      "store_memory_success",
			toolName:  ToolStoreMemory,
			contextID: "test-context",
			agentID:   "test-agent",
			params: map[string]interface{}{
				"content": "Test memory content",
				"metadata": map[string]interface{}{
					"importance": "high",
				},
			},
			setup: func(m *MockStorageClient) {},
			validate: func(t *testing.T, result *ToolResult) {
				if result.IsError {
					t.Errorf("Unexpected error: %s", result.Message)
				}
				
				data, ok := result.Data.(map[string]interface{})
				if !ok {
					t.Errorf("Expected result data to be map[string]interface{}")
					return
				}
				
				if _, ok := data["id"]; !ok {
					t.Errorf("Expected result to contain memory ID")
				}
				
				if _, ok := data["timestamp"]; !ok {
					t.Errorf("Expected result to contain timestamp")
				}
			},
		},
		{
			name:      "retrieve_memory_missing_id",
			toolName:  ToolRetrieveMemory,
			contextID: "test-context",
			agentID:   "test-agent",
			params:    map[string]interface{}{},
			setup:     func(m *MockStorageClient) {},
			validate: func(t *testing.T, result *ToolResult) {
				if !result.IsError {
					t.Errorf("Expected error for missing ID")
				}
				if result.Message != "ID parameter is required" {
					t.Errorf("Expected error message for missing ID, got: %s", result.Message)
				}
			},
		},
		{
			name:      "retrieve_memory_success",
			toolName:  ToolRetrieveMemory,
			contextID: "test-context",
			agentID:   "test-agent",
			params: map[string]interface{}{
				"id": "test-memory-id",
			},
			setup: func(m *MockStorageClient) {
				m.getMessage = &ConversationMessage{
					ID:        "test-memory-id",
					ContextID: "test-context",
					AgentID:   "test-agent",
					Type:      MessageTypeMemory,
					Content:   "Test memory content",
					Timestamp: time.Now().Unix(),
					Metadata: map[string]interface{}{
						"importance": "high",
					},
				}
			},
			validate: func(t *testing.T, result *ToolResult) {
				if result.IsError {
					t.Errorf("Unexpected error: %s", result.Message)
				}
				
				memory, ok := result.Data.(*ConversationMessage)
				if !ok {
					t.Errorf("Expected result data to be *ConversationMessage")
					return
				}
				
				if memory.ID != "test-memory-id" {
					t.Errorf("Expected memory ID to be 'test-memory-id', got: %s", memory.ID)
				}
				
				if memory.Type != MessageTypeMemory {
					t.Errorf("Expected memory type to be '%s', got: %s", MessageTypeMemory, memory.Type)
				}
			},
		},
		{
			name:      "list_recent_messages_success",
			toolName:  ToolListRecentMessages,
			contextID: "test-context",
			agentID:   "test-agent",
			params: map[string]interface{}{
				"limit": float64(3),
				"types": []interface{}{MessageTypeUser, MessageTypeAssistant},
			},
			setup: func(m *MockStorageClient) {
				m.recentMessages = []ConversationMessage{
					{
						ID:        "msg1",
						ContextID: "test-context",
						AgentID:   "test-agent",
						Type:      MessageTypeUser,
						Content:   "Test message 1",
						Timestamp: time.Now().Unix(),
					},
					{
						ID:        "msg2",
						ContextID: "test-context",
						AgentID:   "test-agent",
						Type:      MessageTypeAssistant,
						Content:   "Test response 1",
						Timestamp: time.Now().Unix(),
					},
				}
			},
			validate: func(t *testing.T, result *ToolResult) {
				if result.IsError {
					t.Errorf("Unexpected error: %s", result.Message)
				}
				
				messages, ok := result.Data.([]ConversationMessage)
				if !ok {
					t.Errorf("Expected result data to be []ConversationMessage")
					return
				}
				
				if len(messages) != 2 {
					t.Errorf("Expected 2 messages, got %d", len(messages))
				}
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock storage
			mockStorage := NewMockStorageClient()
			tc.setup(mockStorage)
			
			// Create tool handler
			handler := NewMCPToolHandler(mockStorage)
			
			// Execute tool call
			result := handler.HandleToolCall(tc.toolName, tc.contextID, tc.agentID, tc.params)
			
			// Validate result
			tc.validate(t, result)
		})
	}
}

// Test tool definitions
func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()
	
	// Verify all expected tools are present
	expectedTools := []string{
		ToolSearchConversation,
		ToolStoreMemory,
		ToolRetrieveMemory,
		ToolListRecentMessages,
	}
	
	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}
	
	// Check each tool has the required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Errorf("Tool name is empty")
		}
		
		if tool.Description == "" {
			t.Errorf("Tool description is empty for %s", tool.Name)
		}
		
		if tool.Schema == nil {
			t.Errorf("Tool schema is nil for %s", tool.Name)
		}
	}
}