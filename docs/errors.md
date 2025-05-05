# Error Handling in Engram v3

This document describes the standardized error handling approach in Engram v3, including error types, error response formats, and best practices.

## Error Philosophy

Engram v3 follows these error handling principles:

1. **Consistency**: All errors follow the same structure and pattern
2. **Locality**: Errors are handled as close to their source as possible
3. **Context**: Errors include relevant context to aid debugging
4. **Traceability**: Errors are linked to OpenTelemetry traces
5. **Security**: Error messages don't leak sensitive information
6. **Clarity**: Error messages are clear and actionable

## Error Types

Engram v3 defines a set of standard error types:

```go
package errors

import (
	"fmt"
	"net/http"
)

// Standard error codes
const (
	CodeUnknown           = "UNKNOWN"
	CodeInvalidRequest    = "INVALID_REQUEST"
	CodeNotFound          = "NOT_FOUND"
	CodeAlreadyExists     = "ALREADY_EXISTS"
	CodePermissionDenied  = "PERMISSION_DENIED"
	CodeUnauthenticated   = "UNAUTHENTICATED"
	CodeResourceExhausted = "RESOURCE_EXHAUSTED"
	CodeFailedPrecond     = "FAILED_PRECONDITION"
	CodeAborted           = "ABORTED"
	CodeUnimplemented     = "UNIMPLEMENTED"
	CodeInternal          = "INTERNAL"
	CodeUnavailable       = "UNAVAILABLE"
)

// APIError represents a standard API error
type APIError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	HTTPCode  int                    `json:"-"`
	Cause     error                  `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *APIError) Unwrap() error {
	return e.Cause
}
```

## Error Mapping

Each error code maps to a specific HTTP status code:

| Error Code | HTTP Status Code | Description |
|------------|------------------|-------------|
| `INVALID_REQUEST` | 400 | The request was invalid or malformed |
| `UNAUTHENTICATED` | 401 | The request lacked valid authentication |
| `PERMISSION_DENIED` | 403 | The authenticated user lacks permission |
| `NOT_FOUND` | 404 | The requested resource was not found |
| `ALREADY_EXISTS` | 409 | The resource already exists |
| `FAILED_PRECONDITION` | 412 | Precondition for the operation was not met |
| `RESOURCE_EXHAUSTED` | 429 | Rate limit exceeded or resource exhausted |
| `UNIMPLEMENTED` | 501 | The requested operation is not implemented |
| `UNAVAILABLE` | 503 | The service is temporarily unavailable |
| `INTERNAL` | 500 | Internal server error |
| `UNKNOWN` | 500 | Unknown error |

## Error Factory Functions

To maintain consistency, error creation uses factory functions:

```go
// NewInvalidRequestError creates a new invalid request error
func NewInvalidRequestError(message string, details map[string]interface{}, cause error) *APIError {
	return &APIError{
		Code:     CodeInvalidRequest,
		Message:  message,
		Details:  details,
		HTTPCode: http.StatusBadRequest,
		Cause:    cause,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string, details map[string]interface{}, cause error) *APIError {
	return &APIError{
		Code:     CodeNotFound,
		Message:  message,
		Details:  details,
		HTTPCode: http.StatusNotFound,
		Cause:    cause,
	}
}

// Similar factory functions for other error types...
```

## Error Response Format

All API error responses follow a consistent JSON format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "param": "id",
      "value": "123",
      "reason": "Resource with ID 123 does not exist"
    },
    "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736"
  }
}
```

## Component-Specific Errors

### Storage Errors

```go
// Storage-specific error codes
const (
	CodeStorageKeyNotFound    = "STORAGE_KEY_NOT_FOUND"
	CodeStorageKeyExists      = "STORAGE_KEY_EXISTS"
	CodeStorageCorruption     = "STORAGE_CORRUPTION"
	CodeStorageIOError        = "STORAGE_IO_ERROR"
	CodeStorageBatchError     = "STORAGE_BATCH_ERROR"
	CodeStorageTransactionErr = "STORAGE_TRANSACTION_ERROR"
)
```

### Lock Manager Errors

```go
// Lock manager error codes
const (
	CodeLockNotFound   = "LOCK_NOT_FOUND"
	CodeLockHeld       = "LOCK_HELD"
	CodeLockExpired    = "LOCK_EXPIRED"
	CodeNotLockOwner   = "NOT_LOCK_OWNER"
	CodeLockAcquireErr = "LOCK_ACQUIRE_ERROR"
)
```

### Search Errors

```go
// Search error codes
const (
	CodeSearchIndexErr        = "SEARCH_INDEX_ERROR"
	CodeSearchQueryErr        = "SEARCH_QUERY_ERROR"
	CodeSearchVectorErr       = "SEARCH_VECTOR_ERROR"
	CodeSearchEmbeddingErr    = "SEARCH_EMBEDDING_ERROR"
	CodeSearchWeaviateConnErr = "SEARCH_WEAVIATE_CONNECTION_ERROR"
)
```

## Error Handling Patterns

### API Error Handling

```go
func (a *API) handleGetWorkUnit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract ID from URL params
	id := chi.URLParam(r, "id")
	if id == "" {
		a.writeError(w, r, errors.NewInvalidRequestError("Missing work unit ID", nil, nil))
		return
	}
	
	// Get work unit from storage
	workUnit, err := a.storage.GetWorkUnit(ctx, id)
	if err != nil {
		// Convert storage error to API error
		if errors.Is(err, storage.ErrNotFound) {
			a.writeError(w, r, errors.NewNotFoundError(fmt.Sprintf("Work unit %s not found", id), nil, err))
			return
		}
		
		// Log internal errors with full context
		a.logger.Error().Err(err).Str("id", id).Msg("Failed to get work unit")
		a.writeError(w, r, errors.NewInternalError("Failed to get work unit", nil, err))
		return
	}
	
	// Write successful response
	a.writeJSON(w, http.StatusOK, map[string]interface{}{
		"work_unit": workUnit,
	})
}

// writeError writes a standardized error response
func (a *API) writeError(w http.ResponseWriter, r *http.Request, err *errors.APIError) {
	// Extract trace ID from context
	traceID := trace.SpanContextFromContext(r.Context()).TraceID().String()
	err.TraceID = traceID
	
	// Set response status code
	w.WriteHeader(err.HTTPCode)
	
	// Write JSON response
	a.writeJSON(w, err.HTTPCode, map[string]interface{}{
		"error": err,
	})
	
	// Log the error if it's a server-side error
	if err.HTTPCode >= 500 {
		a.logger.Error().
			Str("code", err.Code).
			Str("message", err.Message).
			Str("trace_id", traceID).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Err(err.Cause).
			Msg("Server error")
	}
}
```

### Storage Error Handling

```go
func (s *Storage) GetWorkUnit(ctx context.Context, id string) (*proto.WorkUnit, error) {
	// Start a span for OpenTelemetry
	ctx, span := tracer.Start(ctx, "Storage.GetWorkUnit")
	defer span.End()
	
	// Add ID attribute to the span
	span.SetAttributes(attribute.String("work_unit.id", id))
	
	// Create the key
	key := workUnitKey(id)
	
	// Perform the get operation
	txn := s.db.NewTransaction(false)
	defer txn.Discard()
	
	item, err := txn.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			// Record the not found error in the span
			span.SetStatus(codes.Error, "Work unit not found")
			span.RecordError(err)
			
			// Return a domain-specific error
			return nil, ErrNotFound
		}
		
		// For other errors, record the error and details
		span.SetStatus(codes.Error, "Failed to get work unit")
		span.RecordError(err)
		
		// Return a wrapped error with context
		return nil, fmt.Errorf("failed to get work unit %s: %w", id, err)
	}
	
	// Read and unmarshal the value
	var workUnit proto.WorkUnit
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &workUnit)
	})
	
	if err != nil {
		span.SetStatus(codes.Error, "Failed to unmarshal work unit")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal work unit %s: %w", id, err)
	}
	
	return &workUnit, nil
}
```

### Business Logic Error Handling

```go
func (e *Engine) CreateWorkUnit(ctx context.Context, request *proto.CreateWorkUnitRequest) (*proto.WorkUnit, error) {
	// Start a span for OpenTelemetry
	ctx, span := tracer.Start(ctx, "Engine.CreateWorkUnit")
	defer span.End()
	
	// Validate the request
	if request.ContextId == "" {
		return nil, errors.NewInvalidRequestError("Context ID is required", nil, nil)
	}
	
	if request.AgentId == "" {
		return nil, errors.NewInvalidRequestError("Agent ID is required", nil, nil)
	}
	
	// Check if the context exists
	exists, err := e.storage.ContextExists(ctx, request.ContextId)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to check context existence")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to check context existence: %w", err)
	}
	
	if !exists {
		return nil, errors.NewNotFoundError(fmt.Sprintf("Context %s not found", request.ContextId), nil, nil)
	}
	
	// Create the work unit
	workUnit := &proto.WorkUnit{
		// ... populate the work unit
	}
	
	// Save the work unit
	err = e.storage.CreateWorkUnit(ctx, workUnit)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to create work unit")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create work unit: %w", err)
	}
	
	// Return the created work unit
	return workUnit, nil
}
```

## Error Logging

Engram v3 uses structured logging with zerolog for error reporting:

```go
logger.Error().
	Err(err).
	Str("component", "storage").
	Str("op", "create_work_unit").
	Str("work_unit_id", workUnit.Id).
	Str("context_id", workUnit.ContextId).
	Str("agent_id", workUnit.AgentId).
	Str("trace_id", trace.SpanContextFromContext(ctx).TraceID().String()).
	Msg("Failed to create work unit")
```

Log fields include:
- Error message and stack trace
- Component name
- Operation being performed
- Relevant identifiers (IDs, paths, etc.)
- Trace ID for correlation with OpenTelemetry
- Additional context-specific details

## OpenTelemetry Integration

Errors are integrated with OpenTelemetry for distributed tracing:

```go
// Record error in span
span.RecordError(err)
span.SetStatus(codes.Error, "Failed to create work unit")
span.SetAttributes(
	attribute.String("error.code", apiErr.Code),
	attribute.String("error.message", apiErr.Message),
)
```

This integration enables:
- Viewing errors in the trace timeline
- Error correlations across components
- Detailed error context in trace viewers like Jaeger

## Error Recovery

For critical operations, Engram v3 implements recovery mechanisms:

```go
func (r *Router) processEvents() {
	defer func() {
		if err := recover(); err != nil {
			// Log the panic
			r.logger.Error().
				Interface("error", err).
				Str("stack", string(debug.Stack())).
				Msg("Recovered from panic in event processing")
			
			// Restart the processing after a delay
			time.Sleep(1 * time.Second)
			go r.processEvents()
		}
	}()
	
	// Process events...
}
```

## Client Error Handling

The Engram client library provides helper methods for error handling:

```go
// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == CodeNotFound
	}
	return false
}

// IsPermissionDeniedError checks if the error is a permission denied error
func IsPermissionDeniedError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == CodePermissionDenied
	}
	return false
}
```

Usage example:
```go
workUnit, err := client.GetWorkUnit(ctx, id)
if err != nil {
	if client.IsNotFoundError(err) {
		// Handle not found case
		return nil, fmt.Errorf("work unit %s not found", id)
	}
	
	if client.IsPermissionDeniedError(err) {
		// Handle permission denied case
		return nil, fmt.Errorf("permission denied to access work unit %s", id)
	}
	
	// Handle other errors
	return nil, fmt.Errorf("failed to get work unit: %w", err)
}
```

## WebSocket and SSE Error Handling

For real-time communications, errors are sent as structured messages:

```json
{
  "type": "error",
  "code": "PERMISSION_DENIED",
  "message": "Not authorized to access context ctx-123",
  "time": "2023-09-01T12:34:56Z",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

## Rate Limiting Errors

Rate limiting returns detailed information:

```json
{
  "error": {
    "code": "RESOURCE_EXHAUSTED",
    "message": "Rate limit exceeded",
    "details": {
      "limit": 100,
      "current": 120,
      "reset_at": "2023-09-01T12:35:00Z"
    },
    "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736"
  }
}
```

These responses include the HTTP headers:
- `X-RateLimit-Limit`: Requests allowed per window
- `X-RateLimit-Remaining`: Requests remaining in current window
- `X-RateLimit-Reset`: Time (in seconds) when the rate limit resets

## Error Status Pages

For human-readable error pages, Engram v3 provides customizable templates:

```go
func (a *API) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	
	if strings.Contains(accept, "text/html") {
		// Render HTML error page for browsers
		a.templates.ExecuteTemplate(w, "error.html", map[string]interface{}{
			"StatusCode": 404,
			"Title":      "Not Found",
			"Message":    "The requested resource could not be found.",
		})
		return
	}
	
	// Return JSON for API clients
	a.writeError(w, r, errors.NewNotFoundError("Resource not found", nil, nil))
}
```

## Best Practices

### Error Creation

- Use the factory functions for creating errors
- Include relevant context in the error details
- Don't expose sensitive information in error messages
- Use different error codes for different error conditions

### Error Handling

- Handle errors at the appropriate level
- Convert low-level errors to domain-specific errors
- Log errors with appropriate severity
- Include trace IDs in error responses

### Error Recovery

- Implement recovery mechanisms for critical components
- Use circuit breakers for external dependencies
- Implement graceful degradation when possible
- Ensure proper cleanup after errors

## Conclusion

Engram v3's standardized error handling provides consistent, informative, and traceable errors throughout the system. This approach improves developer experience, simplifies debugging, and enhances system reliability.