package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper function to create a test message
func createTestMessage(contextID, agentID, msgType, content string) *ConversationMessage {
	msg := NewConversationMessage(contextID, agentID, msgType, content)
	msg.Timestamp = time.Now().Unix()
	return msg
}

// testStorageImplementation defines common tests for any storage implementation
func testStorageImplementation(t *testing.T, storage StorageClient) {
	// Test message creation and storage
	contextID := "test-context"
	agentID := "test-agent"
	
	// Create test messages
	userMsg := createTestMessage(contextID, agentID, MessageTypeUser, "This is a user message")
	assistantMsg := createTestMessage(contextID, agentID, MessageTypeAssistant, "This is an assistant response")
	memoryMsg := createTestMessage(contextID, agentID, MessageTypeMemory, "This is an important memory")
	
	// Add some metadata to the memory message
	memoryMsg.Metadata = map[string]interface{}{
		"importance": "high",
		"category":   "test",
	}
	
	// Test storing messages
	err := storage.StoreMessage(userMsg)
	if err != nil {
		t.Fatalf("Failed to store user message: %v", err)
	}
	
	err = storage.StoreMessage(assistantMsg)
	if err != nil {
		t.Fatalf("Failed to store assistant message: %v", err)
	}
	
	err = storage.StoreMessage(memoryMsg)
	if err != nil {
		t.Fatalf("Failed to store memory message: %v", err)
	}
	
	// Test retrieving messages
	t.Run("GetRecentMessages", func(t *testing.T) {
		messages, err := storage.GetRecentMessages(contextID, 10)
		if err != nil {
			t.Fatalf("Failed to get recent messages: %v", err)
		}
		
		if len(messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(messages))
		}
		
		// Messages should be in reverse chronological order (newest first)
		if len(messages) >= 3 && messages[0].ID != memoryMsg.ID {
			t.Errorf("Expected first message to be the memory message")
		}
	})
	
	// Test message retrieval by ID
	t.Run("GetMessage", func(t *testing.T) {
		msg, err := storage.GetMessage(memoryMsg.ID)
		if err != nil {
			t.Fatalf("Failed to get message by ID: %v", err)
		}
		
		if msg == nil {
			t.Fatalf("Message not found")
		}
		
		if msg.ID != memoryMsg.ID {
			t.Errorf("Expected message ID to be %s, got %s", memoryMsg.ID, msg.ID)
		}
		
		if msg.Content != memoryMsg.Content {
			t.Errorf("Expected content to be %s, got %s", memoryMsg.Content, msg.Content)
		}
		
		if msg.Type != MessageTypeMemory {
			t.Errorf("Expected type to be %s, got %s", MessageTypeMemory, msg.Type)
		}
		
		// Check metadata
		importance, ok := msg.Metadata["importance"].(string)
		if !ok || importance != "high" {
			t.Errorf("Expected metadata importance to be 'high', got %v", msg.Metadata["importance"])
		}
	})
	
	// Test search
	t.Run("SearchMessages", func(t *testing.T) {
		// Search by content
		query := SearchQuery{
			ContextID: contextID,
			Query:     "important",
			Limit:     10,
		}
		
		results, err := storage.SearchMessages(query)
		if err != nil {
			t.Fatalf("Failed to search messages: %v", err)
		}
		
		if len(results) != 1 {
			t.Errorf("Expected 1 result for query 'important', got %d", len(results))
		}
		
		if len(results) > 0 && results[0].ID != memoryMsg.ID {
			t.Errorf("Expected result to be the memory message")
		}
		
		// Search by type
		typeQuery := SearchQuery{
			ContextID: contextID,
			Types:     []string{MessageTypeUser},
			Limit:     10,
		}
		
		typeResults, err := storage.SearchMessages(typeQuery)
		if err != nil {
			t.Fatalf("Failed to search messages by type: %v", err)
		}
		
		if len(typeResults) != 1 {
			t.Errorf("Expected 1 result for type filter, got %d", len(typeResults))
		}
		
		if len(typeResults) > 0 && typeResults[0].ID != userMsg.ID {
			t.Errorf("Expected result to be the user message")
		}
		
		// Search with limit
		limitQuery := SearchQuery{
			ContextID: contextID,
			Limit:     1,
		}
		
		limitResults, err := storage.SearchMessages(limitQuery)
		if err != nil {
			t.Fatalf("Failed to search messages with limit: %v", err)
		}
		
		if len(limitResults) != 1 {
			t.Errorf("Expected 1 result with limit 1, got %d", len(limitResults))
		}
	})
}

// Test the MemoryStorageClient implementation
func TestMemoryStorageClient(t *testing.T) {
	storage := NewMemoryStorageClient()
	defer storage.Close()
	
	testStorageImplementation(t, storage)
}

// Test the BadgerStorageClient implementation
func TestBadgerStorageClient(t *testing.T) {
	// Create temporary directory for BadgerDB
	tempDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create BadgerDB storage
	storage, err := NewBadgerStorageClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create BadgerDB storage: %v", err)
	}
	defer storage.Close()
	
	testStorageImplementation(t, storage)
}

// Test JSON serialization and deserialization of ConversationMessage
func TestConversationMessageSerialization(t *testing.T) {
	// Create test message
	msg := NewConversationMessage("test-context", "test-agent", MessageTypeMemory, "Test content")
	msg.Metadata = map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
		"nested": map[string]interface{}{
			"subkey": "subvalue",
		},
	}
	msg.ParentID = "parent-message-id"
	msg.Annotations = []Annotation{
		{
			Type:    "category",
			Content: "test",
			Data: map[string]interface{}{
				"confidence": 0.95,
			},
		},
	}
	
	// Serialize to JSON
	data, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}
	
	// Deserialize from JSON
	parsedMsg, err := FromJSON(data)
	if err != nil {
		t.Fatalf("Failed to deserialize message: %v", err)
	}
	
	// Verify message fields
	if parsedMsg.ID != msg.ID {
		t.Errorf("Expected ID %s, got %s", msg.ID, parsedMsg.ID)
	}
	
	if parsedMsg.ContextID != msg.ContextID {
		t.Errorf("Expected ContextID %s, got %s", msg.ContextID, parsedMsg.ContextID)
	}
	
	if parsedMsg.AgentID != msg.AgentID {
		t.Errorf("Expected AgentID %s, got %s", msg.AgentID, parsedMsg.AgentID)
	}
	
	if parsedMsg.Type != msg.Type {
		t.Errorf("Expected Type %s, got %s", msg.Type, parsedMsg.Type)
	}
	
	if parsedMsg.Content != msg.Content {
		t.Errorf("Expected Content %s, got %s", msg.Content, parsedMsg.Content)
	}
	
	if parsedMsg.Timestamp != msg.Timestamp {
		t.Errorf("Expected Timestamp %d, got %d", msg.Timestamp, parsedMsg.Timestamp)
	}
	
	if parsedMsg.ParentID != msg.ParentID {
		t.Errorf("Expected ParentID %s, got %s", msg.ParentID, parsedMsg.ParentID)
	}
	
	if len(parsedMsg.Annotations) != len(msg.Annotations) {
		t.Errorf("Expected %d annotations, got %d", len(msg.Annotations), len(parsedMsg.Annotations))
	}
	
	// Check metadata (only for top-level keys for simplicity)
	for key, expected := range msg.Metadata {
		if key == "nested" {
			continue // Skip nested for simplicity
		}
		
		actual, ok := parsedMsg.Metadata[key]
		if !ok {
			t.Errorf("Missing metadata key: %s", key)
			continue
		}
		
		if actual != expected {
			t.Errorf("Metadata value mismatch for %s: expected %v, got %v", key, expected, actual)
		}
	}
}

// Test utility functions for BadgerDB keys
func TestBadgerDBKeyFunctions(t *testing.T) {
	contextID := "test-context"
	messageID := "test-message"
	msgType := "user"
	timestamp := int64(1620000000)
	
	// Test message key
	msgKey := makeMessageKey(contextID, messageID)
	expectedMsgKeyPrefix := []byte(prefixMessages + contextID + ":")
	if string(msgKey[:len(expectedMsgKeyPrefix)]) != string(expectedMsgKeyPrefix) {
		t.Errorf("Message key prefix mismatch: expected %s, got %s", 
			string(expectedMsgKeyPrefix), string(msgKey[:len(expectedMsgKeyPrefix)]))
	}
	
	// Test time index key
	timeKey := makeTimeIndexKey(contextID, timestamp, messageID)
	extractedID := extractMessageIDFromTimeKey(timeKey)
	if extractedID != messageID {
		t.Errorf("Extracted message ID mismatch: expected %s, got %s", messageID, extractedID)
	}
	
	// Test type index key
	typeKey := makeTypeIndexKey(contextID, msgType, timestamp, messageID)
	extractedTypeID := extractMessageIDFromTypeKey(typeKey)
	if extractedTypeID != messageID {
		t.Errorf("Extracted message ID from type key mismatch: expected %s, got %s", 
			messageID, extractedTypeID)
	}
	
	// Test index prefixes
	timePrefix := makeTimeIndexPrefix(contextID)
	if !contains(timeKey, timePrefix) {
		t.Errorf("Time key doesn't contain time prefix")
	}
	
	typePrefix := makeTypeIndexPrefix(contextID, msgType)
	if !contains(typeKey, typePrefix) {
		t.Errorf("Type key doesn't contain type prefix")
	}
}

// Helper function to check if a byte slice contains another byte slice
func contains(haystack, needle []byte) bool {
	needleLen := len(needle)
	if needleLen > len(haystack) {
		return false
	}
	
	for i := 0; i <= len(haystack)-needleLen; i++ {
		if string(haystack[i:i+needleLen]) == string(needle) {
			return true
		}
	}
	return false
}