package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// VectorSearchClient demonstrates vector search capabilities of Engram v3
type VectorSearchClient struct {
	baseURL  string
	agentID  string
	client   *http.Client
	contextID string
}

// NewVectorSearchClient creates a new vector search client
func NewVectorSearchClient(baseURL, agentID string) *VectorSearchClient {
	return &VectorSearchClient{
		baseURL: baseURL,
		agentID: agentID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateContext creates a new context in Engram
func (c *VectorSearchClient) CreateContext(displayName string) (string, error) {
	// Generate a random context ID
	contextID := fmt.Sprintf("ctx-%d", time.Now().UnixNano())
	
	// Prepare the request body
	reqBody := map[string]interface{}{
		"display_name": displayName,
		"add_agents":   []string{c.agentID},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}
	
	// Create the request
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/contexts/%s", c.baseURL, contextID), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", c.agentID)
	
	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}
	
	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error creating context: %s", string(respBody))
	}
	
	// Store the context ID
	c.contextID = contextID
	
	return contextID, nil
}

// AddDocuments adds multiple text documents to Engram
func (c *VectorSearchClient) AddDocuments(documents []string) ([]string, error) {
	if c.contextID == "" {
		return nil, fmt.Errorf("no context created, call CreateContext first")
	}
	
	workUnitIDs := make([]string, 0, len(documents))
	
	for _, doc := range documents {
		// Encode the document as base64
		encoded := base64.StdEncoding.EncodeToString([]byte(doc))
		
		// Prepare the work unit
		workUnit := map[string]interface{}{
			"context_id": c.contextID,
			"agent_id":   c.agentID,
			"type":       1, // MESSAGE type
			"meta": map[string]string{
				"content_type": "text/plain",
				"indexed":      "true",
			},
			"payload": encoded,
		}
		
		jsonData, err := json.Marshal(workUnit)
		if err != nil {
			return workUnitIDs, fmt.Errorf("error marshaling JSON: %w", err)
		}
		
		// Create the request
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/workunit", c.baseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			return workUnitIDs, fmt.Errorf("error creating request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-ID", c.agentID)
		
		// Send the request
		resp, err := c.client.Do(req)
		if err != nil {
			return workUnitIDs, fmt.Errorf("error sending request: %w", err)
		}
		
		// Read the response
		respBody, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return workUnitIDs, fmt.Errorf("error reading response: %w", err)
		}
		
		// Check if the request was successful
		if resp.StatusCode != http.StatusCreated {
			return workUnitIDs, fmt.Errorf("error adding document: %s", string(respBody))
		}
		
		// Extract the work unit ID
		var respData map[string]interface{}
		if err := json.Unmarshal(respBody, &respData); err != nil {
			return workUnitIDs, fmt.Errorf("error unmarshaling response: %w", err)
		}
		
		workUnitMap := respData["work_unit"].(map[string]interface{})
		workUnitID := workUnitMap["id"].(string)
		workUnitIDs = append(workUnitIDs, workUnitID)
		
		// Sleep briefly to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
	}
	
	return workUnitIDs, nil
}

// VectorSearchResult represents a result from a vector search
type VectorSearchResult struct {
	WorkUnit   map[string]interface{} `json:"work_unit"`
	Similarity float64                `json:"similarity"`
}

// VectorSearchResponse represents the API response for a vector search
type VectorSearchResponse struct {
	Results    []VectorSearchResult `json:"results"`
	TotalCount int                  `json:"total_count"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
}

// PerformVectorSearch performs a semantic search using vector embeddings
func (c *VectorSearchClient) PerformVectorSearch(query string, limit int, minSimilarity float64) ([]VectorSearchResult, error) {
	if c.contextID == "" {
		return nil, fmt.Errorf("no context created, call CreateContext first")
	}
	
	// Prepare the search request
	searchReq := map[string]interface{}{
		"query":         query,
		"context_id":    c.contextID,
		"limit":         limit,
		"min_similarity": minSimilarity,
	}
	
	jsonData, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling JSON: %w", err)
	}
	
	// Create the request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/search/vector", c.baseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", c.agentID)
	
	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	
	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error performing vector search: %s", string(respBody))
	}
	
	// Parse the response
	var searchResp VectorSearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}
	
	return searchResp.Results, nil
}

// GetWorkUnit retrieves a work unit by ID
func (c *VectorSearchClient) GetWorkUnit(workUnitID string) (map[string]interface{}, error) {
	// Create the request
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/workunit/%s", c.baseURL, workUnitID), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Set header
	req.Header.Set("X-Agent-ID", c.agentID)
	
	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	
	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting work unit: %s", string(respBody))
	}
	
	// Parse the response
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}
	
	workUnit := respData["work_unit"].(map[string]interface{})
	return workUnit, nil
}

// PrintWorkUnitContent prints the content of a work unit after decoding from base64
func PrintWorkUnitContent(workUnit map[string]interface{}) {
	if payload, ok := workUnit["payload"].(string); ok {
		// Decode the base64 payload
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			fmt.Printf("Error decoding payload: %v\n", err)
			return
		}
		
		fmt.Println("Content:", string(decoded))
	} else {
		fmt.Println("No payload found")
	}
}

func main() {
	// Configuration
	baseURL := "http://localhost:8080"
	agentID := "example-agent"
	
	// Override from environment variables if available
	if envURL := os.Getenv("ENGRAM_URL"); envURL != "" {
		baseURL = envURL
	}
	if envAgent := os.Getenv("ENGRAM_AGENT_ID"); envAgent != "" {
		agentID = envAgent
	}
	
	// Create a new client
	client := NewVectorSearchClient(baseURL, agentID)
	
	// Create a new context
	contextID, err := client.CreateContext("Vector Search Example")
	if err != nil {
		fmt.Printf("Error creating context: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Created context: %s\n", contextID)
	
	// Sample documents about various topics
	documents := []string{
		"Distributed tracing is a method used to profile and monitor applications, especially those built using a microservices architecture. It helps pinpoint where failures occur and what causes poor performance.",
		"BadgerDB is an embeddable, persistent, and fast key-value database written in pure Go. It's designed to be highly performant for SSDs.",
		"Vector search, also known as semantic search, uses embeddings to find similar content based on meaning rather than exact keyword matches.",
		"OpenTelemetry provides a single set of APIs, libraries, agents, and collector services to capture distributed traces and metrics from your application.",
		"Weaviate is an open-source vector search engine that enables the search through data based on vector representations of data.",
		"Go concurrency is built around the concept of goroutines and channels, which provide a simple and efficient way to structure concurrent programs.",
		"Prometheus is an open-source systems monitoring and alerting toolkit originally built at SoundCloud.",
		"Kubernetes is an open-source container orchestration system for automating software deployment, scaling, and management.",
		"JSON Web Tokens (JWT) are an open, industry standard method for representing claims securely between two parties.",
	}
	
	// Add documents to Engram
	workUnitIDs, err := client.AddDocuments(documents)
	if err != nil {
		fmt.Printf("Error adding documents: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Added %d documents\n", len(workUnitIDs))
	
	// Wait for a moment to allow for indexing
	fmt.Println("Waiting for documents to be indexed...")
	time.Sleep(3 * time.Second)
	
	// Perform vector search
	query := "What is the purpose of distributed tracing in microservices?"
	fmt.Printf("Performing vector search for query: %s\n", query)
	
	results, err := client.PerformVectorSearch(query, 3, 0.7)
	if err != nil {
		fmt.Printf("Error performing vector search: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Found %d results\n", len(results))
	
	// Print the results
	for i, result := range results {
		fmt.Printf("\nResult %d (Similarity: %.4f):\n", i+1, result.Similarity)
		PrintWorkUnitContent(result.WorkUnit)
	}
	
	// Try another query
	anotherQuery := "How does vector search find semantically similar content?"
	fmt.Printf("\nPerforming another vector search for query: %s\n", anotherQuery)
	
	results, err = client.PerformVectorSearch(anotherQuery, 3, 0.7)
	if err != nil {
		fmt.Printf("Error performing vector search: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Found %d results\n", len(results))
	
	// Print the results
	for i, result := range results {
		fmt.Printf("\nResult %d (Similarity: %.4f):\n", i+1, result.Similarity)
		PrintWorkUnitContent(result.WorkUnit)
	}
}