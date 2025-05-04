package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
)

// Key prefixes for different types of MCP data
const (
	prefixMessages = "msg:"
	prefixContexts = "ctx:"
	prefixMemories = "mem:"
)

// BadgerStorageClient implements the StorageClient interface using BadgerDB directly
type BadgerStorageClient struct {
	db        *badger.DB
	dataDir   string
	contextID string
}

// NewBadgerStorageClient creates a new BadgerDB-based storage client
func NewBadgerStorageClient(dataDir string) (*BadgerStorageClient, error) {
	// Create Badger DB directory
	dbPath := filepath.Join(dataDir, "badger")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create badger data directory: %w", err)
	}

	// Configure Badger options
	options := badger.DefaultOptions(dbPath)
	options = options.WithLoggingLevel(badger.WARNING) // Reduce logging noise

	// Open the Badger database
	db, err := badger.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to open Badger: %w", err)
	}

	return &BadgerStorageClient{
		db:      db,
		dataDir: dataDir,
	}, nil
}

// StoreMessage stores a message in the conversation history
func (c *BadgerStorageClient) StoreMessage(msg *ConversationMessage) error {
	// Store context ID for future operations
	c.contextID = msg.ContextID

	// Convert to JSON
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create transaction
	txn := c.db.NewTransaction(true)
	defer txn.Discard()

	// Create key with proper prefix
	key := makeMessageKey(msg.ContextID, msg.ID)

	// Set data in transaction
	err = txn.Set(key, data)
	if err != nil {
		return fmt.Errorf("failed to set message data: %w", err)
	}

	// Create time-based index for easy retrieval
	timeKey := makeTimeIndexKey(msg.ContextID, msg.Timestamp, msg.ID)
	err = txn.Set(timeKey, []byte{1}) // Use a minimal value as we only need the key
	if err != nil {
		return fmt.Errorf("failed to set time index: %w", err)
	}

	// Add type index for faster filtering
	typeKey := makeTypeIndexKey(msg.ContextID, msg.Type, msg.Timestamp, msg.ID)
	err = txn.Set(typeKey, []byte{1})
	if err != nil {
		return fmt.Errorf("failed to set type index: %w", err)
	}

	// Commit the transaction
	if err := txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetRecentMessages retrieves recent messages for a conversation
func (c *BadgerStorageClient) GetRecentMessages(contextID string, limit int) ([]ConversationMessage, error) {
	if contextID == "" && c.contextID != "" {
		contextID = c.contextID
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	var messages []ConversationMessage

	// Start a read-only transaction
	err := c.db.View(func(txn *badger.Txn) error {
		// Create options for iteration
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // Sort newest to oldest
		it := txn.NewIterator(opts)
		defer it.Close()

		// Create prefix for time-based index
		prefix := makeTimeIndexPrefix(contextID)

		// Seek to the newest message
		for it.Seek(append(prefix, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)); it.ValidForPrefix(prefix); it.Next() {
			if len(messages) >= limit {
				break
			}

			// Extract message ID from the time index key
			key := it.Item().Key()
			msgID := extractMessageIDFromTimeKey(key)

			// Get the actual message
			msgKey := makeMessageKey(contextID, msgID)
			item, err := txn.Get(msgKey)
			if err != nil {
				log.Warn().Err(err).Str("id", msgID).Msg("Failed to retrieve message")
				continue
			}

			// Get message data
			var msgData []byte
			err = item.Value(func(val []byte) error {
				msgData = append([]byte{}, val...)
				return nil
			})
			if err != nil {
				log.Warn().Err(err).Str("id", msgID).Msg("Failed to read message data")
				continue
			}

			// Parse message
			var msg ConversationMessage
			if err := json.Unmarshal(msgData, &msg); err != nil {
				log.Warn().Err(err).Str("id", msgID).Msg("Failed to unmarshal message")
				continue
			}

			messages = append(messages, msg)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}

	return messages, nil
}

// SearchMessages searches for messages in a conversation
func (c *BadgerStorageClient) SearchMessages(query SearchQuery) ([]ConversationMessage, error) {
	contextID := query.ContextID
	if contextID == "" && c.contextID != "" {
		contextID = c.contextID
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	var matches []ConversationMessage

	// First, gather all candidate messages based on type filter
	err := c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // Most recent first
		it := txn.NewIterator(opts)
		defer it.Close()

		// If types are specified, use type-based index for more efficient filtering
		if len(query.Types) > 0 {
			for _, msgType := range query.Types {
				// Create prefix for the specific type
				prefix := makeTypeIndexPrefix(contextID, msgType)

				// Collect messages of this type
				for it.Seek(append(prefix, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)); it.ValidForPrefix(prefix); it.Next() {
					if len(matches) >= limit {
						break
					}

					// Extract message ID from the type index key
					key := it.Item().Key()
					msgID := extractMessageIDFromTypeKey(key)

					// Get the actual message
					msgKey := makeMessageKey(contextID, msgID)
					item, err := txn.Get(msgKey)
					if err != nil {
						log.Warn().Err(err).Str("id", msgID).Msg("Failed to retrieve message")
						continue
					}

					// Get message data
					var msgData []byte
					err = item.Value(func(val []byte) error {
						msgData = append([]byte{}, val...)
						return nil
					})
					if err != nil {
						log.Warn().Err(err).Str("id", msgID).Msg("Failed to read message data")
						continue
					}

					// Parse message
					var msg ConversationMessage
					if err := json.Unmarshal(msgData, &msg); err != nil {
						log.Warn().Err(err).Str("id", msgID).Msg("Failed to unmarshal message")
						continue
					}

					// If there's a search query, check if the message content contains it
					if query.Query != "" {
						if !containsIgnoreCase(msg.Content, query.Query) {
							continue
						}
					}

					matches = append(matches, msg)
				}
			}
		} else {
			// No type filter, use time-based index for all messages
			prefix := makeTimeIndexPrefix(contextID)

			for it.Seek(append(prefix, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)); it.ValidForPrefix(prefix); it.Next() {
				if len(matches) >= limit {
					break
				}

				// Extract message ID from the time index key
				key := it.Item().Key()
				msgID := extractMessageIDFromTimeKey(key)

				// Get the actual message
				msgKey := makeMessageKey(contextID, msgID)
				item, err := txn.Get(msgKey)
				if err != nil {
					log.Warn().Err(err).Str("id", msgID).Msg("Failed to retrieve message")
					continue
				}

				// Get message data
				var msgData []byte
				err = item.Value(func(val []byte) error {
					msgData = append([]byte{}, val...)
					return nil
				})
				if err != nil {
					log.Warn().Err(err).Str("id", msgID).Msg("Failed to read message data")
					continue
				}

				// Parse message
				var msg ConversationMessage
				if err := json.Unmarshal(msgData, &msg); err != nil {
					log.Warn().Err(err).Str("id", msgID).Msg("Failed to unmarshal message")
					continue
				}

				// If there's a search query, check if the message content contains it
				if query.Query != "" {
					if !containsIgnoreCase(msg.Content, query.Query) {
						continue
					}
				}

				matches = append(matches, msg)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	// Apply offset if needed
	if query.Offset > 0 && query.Offset < len(matches) {
		matches = matches[query.Offset:]
	}

	return matches, nil
}

// GetMessage retrieves a specific message by ID
func (c *BadgerStorageClient) GetMessage(id string) (*ConversationMessage, error) {
	var msg *ConversationMessage

	err := c.db.View(func(txn *badger.Txn) error {
		// If we don't know the context ID, we need to check all possible contexts
		// In a real implementation, we would have a global index of message IDs
		// For simplicity, we'll assume we know the context ID
		contextID := c.contextID
		if contextID == "" {
			return fmt.Errorf("context ID is required to retrieve message")
		}

		key := makeMessageKey(contextID, id)
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("failed to get message: %w", err)
		}

		// Get message data
		var msgData []byte
		err = item.Value(func(val []byte) error {
			msgData = append([]byte{}, val...)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to read message data: %w", err)
		}

		// Parse message
		msg = &ConversationMessage{}
		if err := json.Unmarshal(msgData, msg); err != nil {
			return fmt.Errorf("failed to unmarshal message: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return msg, nil
}

// Close closes the storage connection
func (c *BadgerStorageClient) Close() error {
	return c.db.Close()
}

// Key utility functions

// makeMessageKey creates a key for storing message data
func makeMessageKey(contextID, messageID string) []byte {
	return []byte(fmt.Sprintf("%s%s:%s", prefixMessages, contextID, messageID))
}

// makeTimeIndexKey creates a time-based index key for efficient retrieval
func makeTimeIndexKey(contextID string, timestamp int64, messageID string) []byte {
	return []byte(fmt.Sprintf("t:%s:%020d:%s", contextID, timestamp, messageID))
}

// makeTimeIndexPrefix creates a prefix for time-based index
func makeTimeIndexPrefix(contextID string) []byte {
	return []byte(fmt.Sprintf("t:%s:", contextID))
}

// makeTypeIndexKey creates a type-based index key for filtering by message type
func makeTypeIndexKey(contextID, msgType string, timestamp int64, messageID string) []byte {
	return []byte(fmt.Sprintf("type:%s:%s:%020d:%s", contextID, msgType, timestamp, messageID))
}

// makeTypeIndexPrefix creates a prefix for type-based index
func makeTypeIndexPrefix(contextID, msgType string) []byte {
	return []byte(fmt.Sprintf("type:%s:%s:", contextID, msgType))
}

// extractMessageIDFromTimeKey extracts message ID from a time index key
func extractMessageIDFromTimeKey(key []byte) string {
	parts := strings.Split(string(key), ":")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

// extractMessageIDFromTypeKey extracts message ID from a type index key
func extractMessageIDFromTypeKey(key []byte) string {
	parts := strings.Split(string(key), ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}

// Helper function for case-insensitive substring search
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}