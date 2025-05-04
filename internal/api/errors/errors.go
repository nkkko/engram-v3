package errors

import (
	"fmt"
	"net/http"
)

// ErrorType defines the type of error
type ErrorType string

const (
	// ErrorTypeValidation represents a validation error
	ErrorTypeValidation ErrorType = "validation"
	
	// ErrorTypeNotFound represents a not found error
	ErrorTypeNotFound ErrorType = "not_found"
	
	// ErrorTypeConflict represents a conflict error
	ErrorTypeConflict ErrorType = "conflict"
	
	// ErrorTypeInternal represents an internal server error
	ErrorTypeInternal ErrorType = "internal"
	
	// ErrorTypeUnauthorized represents an unauthorized error
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	
	// ErrorTypeForbidden represents a forbidden error
	ErrorTypeForbidden ErrorType = "forbidden"
	
	// ErrorTypeTimeout represents a timeout error
	ErrorTypeTimeout ErrorType = "timeout"
)

// APIError represents a standardized API error
type APIError struct {
	Type      ErrorType `json:"type"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Details   any       `json:"details,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	HTTPCode  int       `json:"-"` // Not serialized
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Code, e.Message)
}

// WithDetails adds details to the error
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// WithRequestID adds a request ID to the error
func (e *APIError) WithRequestID(requestID string) *APIError {
	e.RequestID = requestID
	return e
}

// ValidationError creates a new validation error
func ValidationError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeValidation,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusBadRequest,
	}
}

// NotFoundError creates a new not found error
func NotFoundError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeNotFound,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusNotFound,
	}
}

// ConflictError creates a new conflict error
func ConflictError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeConflict,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusConflict,
	}
}

// InternalError creates a new internal server error
func InternalError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeInternal,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusInternalServerError,
	}
}

// UnauthorizedError creates a new unauthorized error
func UnauthorizedError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeUnauthorized,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusUnauthorized,
	}
}

// ForbiddenError creates a new forbidden error
func ForbiddenError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeForbidden,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusForbidden,
	}
}

// TimeoutError creates a new timeout error
func TimeoutError(code string, message string) *APIError {
	return &APIError{
		Type:     ErrorTypeTimeout,
		Code:     code,
		Message:  message,
		HTTPCode: http.StatusGatewayTimeout,
	}
}

// FromError creates a new API error from a Go error
func FromError(err error) *APIError {
	if err == nil {
		return nil
	}
	
	// Check if it's already an APIError
	if apiErr, ok := err.(*APIError); ok {
		return apiErr
	}
	
	// Default to an internal server error
	return InternalError("internal_error", err.Error())
}