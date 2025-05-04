package internal

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MemoryStorageClient implements the StorageClient interface using in-memory storage
type MemoryStorageClient struct {
	messages  map[string]*ConversationMessage // message ID -> message
	timeIndex map[string][]string             // contextID -> ordered list of message IDs by time
	typeIndex map[string]map[string][]string  // contextID+type -> list of message IDs
	mu        sync.RWMutex
	contextID string
}

// NewMemoryStorageClient creates a new in-memory storage client
func NewMemoryStorageClient() *MemoryStorageClient {
	return &MemoryStorageClient{
		messages:  make(map[string]*ConversationMessage),
		timeIndex: make(map[string][]string),
		typeIndex: make(map[string]map[string][]string),
	}
}

// StoreMessage stores a message in memory
func (c *MemoryStorageClient) StoreMessage(msg *ConversationMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store context ID for future operations
	c.contextID = msg.ContextID

	// Make a copy of the message
	msgCopy := *msg
	
	// Store the message by ID
	c.messages[msg.ID] = &msgCopy

	// Add to time index
	if _, ok := c.timeIndex[msg.ContextID]; !ok {
		c.timeIndex[msg.ContextID] = make([]string, 0)
	}
	c.timeIndex[msg.ContextID] = append(c.timeIndex[msg.ContextID], msg.ID)

	// Sort time index by timestamp (newest first)
	sort.SliceStable(c.timeIndex[msg.ContextID], func(i, j int) bool {
		idI := c.timeIndex[msg.ContextID][i]
		idJ := c.timeIndex[msg.ContextID][j]
		return c.messages[idI].Timestamp > c.messages[idJ].Timestamp
	})

	// Add to type index
	typeKey := msg.ContextID + ":" + msg.Type
	if _, ok := c.typeIndex[typeKey]; !ok {
		c.typeIndex[typeKey] = make(map[string][]string)
	}
	c.typeIndex[typeKey][msg.ID] = append(c.typeIndex[typeKey][msg.ID], msg.ID)

	return nil
}

// GetRecentMessages retrieves recent messages for a conversation
func (c *MemoryStorageClient) GetRecentMessages(contextID string, limit int) ([]ConversationMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if contextID == "" && c.contextID != "" {
		contextID = c.contextID
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Get IDs from time index
	ids, ok := c.timeIndex[contextID]
	if !ok {
		return []ConversationMessage{}, nil // No messages for this context
	}

	// Get messages up to the limit
	var messages []ConversationMessage
	for i := 0; i < len(ids) && i < limit; i++ {
		if msg, ok := c.messages[ids[i]]; ok {
			messages = append(messages, *msg)
		}
	}

	return messages, nil
}

// SearchMessages searches for messages in a conversation
func (c *MemoryStorageClient) SearchMessages(query SearchQuery) ([]ConversationMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	contextID := query.ContextID
	if contextID == "" && c.contextID != "" {
		contextID = c.contextID
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	var ids []string

	// Filter by type if specified
	if len(query.Types) > 0 {
		// Collect IDs from all requested types
		for _, msgType := range query.Types {
			typeKey := contextID + ":" + msgType
			if typeIDs, ok := c.typeIndex[typeKey]; ok {
				for id := range typeIDs {
					ids = append(ids, id)
				}
			}
		}
	} else {
		// Use all messages for the context
		ids = c.timeIndex[contextID]
	}

	// Filter by content if query is specified
	var matches []ConversationMessage
	for _, id := range ids {
		if len(matches) >= limit {
			break
		}

		msg, ok := c.messages[id]
		if !ok {
			continue
		}

		// If there's a search query, check if the message content contains it
		if query.Query != "" {
			if !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query.Query)) {
				continue
			}
		}

		matches = append(matches, *msg)
	}

	// Apply offset if needed
	if query.Offset > 0 && query.Offset < len(matches) {
		matches = matches[query.Offset:]
	}

	return matches, nil
}

// GetMessage retrieves a specific message by ID
func (c *MemoryStorageClient) GetMessage(id string) (*ConversationMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msg, ok := c.messages[id]
	if !ok {
		return nil, fmt.Errorf("message not found: %s", id)
	}

	// Return a copy of the message
	msgCopy := *msg
	return &msgCopy, nil
}

// Close does nothing for memory storage
func (c *MemoryStorageClient) Close() error {
	return nil
}