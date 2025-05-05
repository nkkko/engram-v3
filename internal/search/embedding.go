package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// EmbeddingService handles generating vector embeddings
type EmbeddingService struct {
	model    string
	url      string
	apiKey   string
	cache    map[string][]float32
	cacheMu  sync.RWMutex
	client   *http.Client
	logger   zerolog.Logger
	disabled bool
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(model, url, apiKey string) *EmbeddingService {
	if model == "" {
		model = "openai/text-embedding-3-small"
	}

	logger := log.With().Str("component", "embedding").Logger()

	// If no URL or API key, run in disabled mode
	disabled := url == "" || apiKey == ""
	if disabled {
		logger.Warn().Msg("Embedding service disabled due to missing URL or API key")
	}

	return &EmbeddingService{
		model:    model,
		url:      url,
		apiKey:   apiKey,
		cache:    make(map[string][]float32),
		client:   &http.Client{Timeout: 10 * time.Second},
		logger:   logger,
		disabled: disabled,
	}
}

// GetEmbedding returns a vector embedding for a text
func (s *EmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if s.disabled {
		// Return a default embedding with zeros when disabled
		return make([]float32, 0), nil
	}

	// Check cache
	s.cacheMu.RLock()
	if embedding, ok := s.cache[text]; ok {
		s.cacheMu.RUnlock()
		return embedding, nil
	}
	s.cacheMu.RUnlock()

	// Prepare request based on model
	var err error
	var embedding []float32

	if s.model == "openai/text-embedding-3-small" || s.model == "openai/text-embedding-3-large" {
		embedding, err = s.getOpenAIEmbedding(ctx, text)
	} else {
		err = fmt.Errorf("unsupported embedding model: %s", s.model)
	}

	if err != nil {
		return nil, err
	}

	// Store in cache
	s.cacheMu.Lock()
	s.cache[text] = embedding
	s.cacheMu.Unlock()

	return embedding, nil
}

// getOpenAIEmbedding gets an embedding from OpenAI API
func (s *EmbeddingService) getOpenAIEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Prepare request
	type openAIRequest struct {
		Input string `json:"input"`
		Model string `json:"model"`
	}

	// Extract model name without provider prefix
	modelName := s.model
	if len(modelName) > 7 && modelName[:7] == "openai/" {
		modelName = modelName[7:]
	}

	reqBody, err := json.Marshal(openAIRequest{
		Input: text,
		Model: modelName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", s.url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	type openAIResponse struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	var result openAIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

// Close cleans up resources
func (s *EmbeddingService) Close() {
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
}