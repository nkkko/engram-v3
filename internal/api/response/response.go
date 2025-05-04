package response

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/nkkko/engram-v3/internal/api/errors"
)

// Response represents a standardized API response
type Response struct {
	Success   bool   `json:"success"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
	Error     any    `json:"error,omitempty"`
	Meta      any    `json:"meta,omitempty"`
}

// JSON sends a JSON response
func JSON(w http.ResponseWriter, r *http.Request, statusCode int, data any) {
	// Get request ID from context (set by middleware.RequestID)
	requestID := middleware.GetReqID(r.Context())
	
	// Create response
	resp := Response{
		Success:   statusCode >= 200 && statusCode < 300,
		RequestID: requestID,
		Data:      data,
	}
	
	sendJSON(w, statusCode, resp)
}

// Error sends an error response
func Error(w http.ResponseWriter, r *http.Request, err error) {
	// Get request ID from context (set by middleware.RequestID)
	requestID := middleware.GetReqID(r.Context())
	
	// Convert to APIError if needed
	apiErr := errors.FromError(err)
	
	// Add request ID to error
	apiErr.WithRequestID(requestID)
	
	// Create response
	resp := Response{
		Success:   false,
		RequestID: requestID,
		Error:     apiErr,
	}
	
	sendJSON(w, apiErr.HTTPCode, resp)
}

// WithMeta adds metadata to a successful response
func WithMeta(w http.ResponseWriter, r *http.Request, statusCode int, data any, meta any) {
	// Get request ID from context (set by middleware.RequestID)
	requestID := middleware.GetReqID(r.Context())
	
	// Create response
	resp := Response{
		Success:   statusCode >= 200 && statusCode < 300,
		RequestID: requestID,
		Data:      data,
		Meta:      meta,
	}
	
	sendJSON(w, statusCode, resp)
}

// sendJSON is a helper function to send a JSON response
func sendJSON(w http.ResponseWriter, statusCode int, data any) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")
	
	// Set status code
	w.WriteHeader(statusCode)
	
	// Encode JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, `{"success":false,"error":{"type":"internal","code":"json_encode_error","message":"Failed to encode JSON response"}}`, http.StatusInternalServerError)
	}
}