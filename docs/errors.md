# Error Handling in Engram v3

This document describes the standardized error handling system in Engram v3.

## Error Structure

API errors follow a standardized format:

```json
{
  "success": false,
  "request_id": "req_123456789",
  "error": {
    "type": "validation",
    "code": "missing_required_field",
    "message": "Field 'name' is required",
    "details": {
      "field": "name",
      "constraint": "required"
    }
  }
}
```

### Fields

- `success`: Boolean indicating whether the request was successful (always `false` for errors)
- `request_id`: Unique identifier for the request, useful for tracing and debugging
- `error`: Object containing error details
  - `type`: Type of error (see Error Types)
  - `code`: Machine-readable error code (see Error Codes)
  - `message`: Human-readable error message
  - `details`: Optional object with additional error details

## Error Types

The following error types are used:

- `validation`: The request payload failed validation
- `not_found`: The requested resource was not found
- `conflict`: The request conflicts with the current state of the resource
- `internal`: An internal server error occurred
- `unauthorized`: Authentication is required but was not provided or is invalid
- `forbidden`: The authenticated user does not have permission to access the resource
- `timeout`: The operation timed out

## Common Error Codes

### Validation Errors

- `missing_required_field`: A required field is missing
- `invalid_format`: A field has an invalid format
- `max_length_exceeded`: A field exceeds the maximum allowed length
- `min_length_not_met`: A field does not meet the minimum required length
- `max_value_exceeded`: A numeric field exceeds the maximum allowed value
- `min_value_not_met`: A numeric field does not meet the minimum required value
- `invalid_enum_value`: An enum field has an invalid value
- `invalid_json`: The request body contains invalid JSON

### Not Found Errors

- `resource_not_found`: The requested resource was not found
- `work_unit_not_found`: The requested work unit was not found
- `context_not_found`: The requested context was not found
- `lock_not_found`: The requested lock was not found

### Conflict Errors

- `resource_already_exists`: The resource already exists
- `resource_already_locked`: The resource is already locked by another agent
- `lock_id_mismatch`: The lock ID provided does not match the lock ID on the server
- `optimistic_lock_failure`: The resource was modified since it was last read

### Internal Errors

- `internal_server_error`: An unexpected error occurred on the server
- `database_error`: An error occurred while interacting with the database
- `serialization_error`: An error occurred while serializing or deserializing data

## Implementation

Error handling is implemented in the following packages:

- `internal/api/errors`: Core error types and error creation functions
- `internal/api/response`: Response structure and helper functions
- `internal/api/validation`: Request validation utilities

### Example Usage

```go
// Creating and returning errors
if req.ID == "" {
    return errors.ValidationError("missing_id", "ID is required")
}

// Handling errors in API endpoints
func (a *API) handleGetWorkUnit(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        response.Error(w, r, errors.ValidationError("missing_id", "Work unit ID is required"))
        return
    }
    
    workUnit, err := a.storage.GetWorkUnit(r.Context(), id)
    if err != nil {
        response.Error(w, r, errors.NotFoundError("work_unit_not_found", "Work unit not found"))
        return
    }
    
    // Convert and return successful response
    response.JSON(w, r, http.StatusOK, workUnit)
}
```

## Client Integration

Clients should handle errors by checking the `success` field in the response. If `success` is `false`, the response contains an error. The `type` and `code` fields can be used to determine the appropriate client-side behavior.