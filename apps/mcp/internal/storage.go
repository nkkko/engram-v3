package internal

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Known message types for MCP work units
const (
	MessageTypeUser        = "user"
	MessageTypeAssistant   = "assistant"
	MessageTypeMemory      = "memory"
	MessageTypeSearch      = "search"
)

// ConversationMessage represents a message in a conversation
type ConversationMessage struct {
	ID          string                 `json:"id"`
	ContextID   string                 `json:"contextId"`    // Conversation identifier
	AgentID     string                 `json:"agentId"`      // Model/assistant identifier
	Type        string                 `json:"type"`         // Message type (user/assistant/memory/search)
	Content     string                 `json:"content"`      // Message content
	Metadata    map[string]interface{} `json:"metadata"`     // Additional metadata
	Timestamp   int64                  `json:"timestamp"`    // Unix timestamp
	ParentID    string                 `json:"parentId"`     // Optional reference to parent message
	Annotations []Annotation           `json:"annotations"`  // Optional annotations
}

// Annotation represents metadata attached to a message
type Annotation struct {
	Type    string                 `json:"type"`
	Content string                 `json:"content,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// SearchQuery represents parameters for conversation search
type SearchQuery struct {
	ContextID string   `json:"contextId"`           // Conversation identifier
	Query     string   `json:"query"`               // Search query text
	Types     []string `json:"types,omitempty"`     // Optional filter by message types
	Limit     int      `json:"limit,omitempty"`     // Max results to return (default 10)
	Offset    int      `json:"offset,omitempty"`    // Results to skip (default 0)
}

// StorageClient interface defines the operations needed by the MCP server
type StorageClient interface {
	// Store a message in the conversation history
	StoreMessage(msg *ConversationMessage) error
	
	// Retrieve recent messages for a conversation
	GetRecentMessages(contextID string, limit int) ([]ConversationMessage, error)
	
	// Search messages in a conversation
	SearchMessages(query SearchQuery) ([]ConversationMessage, error)
	
	// Get a specific message by ID
	GetMessage(id string) (*ConversationMessage, error)
	
	// Close the storage connection
	Close() error
}

// NewConversationMessage creates a new message with generated ID and timestamp
func NewConversationMessage(contextID, agentID, msgType, content string) *ConversationMessage {
	return &ConversationMessage{
		ID:        uuid.New().String(),
		ContextID: contextID,
		AgentID:   agentID,
		Type:      msgType,
		Content:   content,
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

// ToJSON converts a ConversationMessage to JSON bytes
func (msg *ConversationMessage) ToJSON() ([]byte, error) {
	return json.Marshal(msg)
}

// FromJSON converts JSON bytes to a ConversationMessage
func FromJSON(data []byte) (*ConversationMessage, error) {
	var msg ConversationMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}