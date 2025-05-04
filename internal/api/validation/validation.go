package validation

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nkkko/engram-v3/internal/api/errors"
)

// Validator defines the interface for request validation
type Validator interface {
	Validate() error
}

// ParseAndValidate parses a JSON request body and validates it
func ParseAndValidate(r *http.Request, v Validator) error {
	// Parse JSON body
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if strings.Contains(err.Error(), "EOF") {
			return errors.ValidationError("empty_request_body", "Request body is empty")
		}
		return errors.ValidationError("invalid_json", "Invalid JSON format: "+err.Error())
	}
	
	// Validate
	if err := v.Validate(); err != nil {
		return err
	}
	
	return nil
}

// MaxLength validates that a string is not longer than the specified max length
func MaxLength(field, value string, maxLen int) error {
	if len(value) > maxLen {
		return errors.ValidationError(
			"max_length_exceeded",
			field+" must be at most "+string(rune(maxLen))+" characters",
		)
	}
	return nil
}

// MinLength validates that a string is not shorter than the specified min length
func MinLength(field, value string, minLen int) error {
	if len(value) < minLen {
		return errors.ValidationError(
			"min_length_not_met",
			field+" must be at least "+string(rune(minLen))+" characters",
		)
	}
	return nil
}

// Required validates that a string is not empty
func Required(field, value string) error {
	if value == "" {
		return errors.ValidationError(
			"required_field_missing",
			field+" is required",
		)
	}
	return nil
}

// Max validates that a number is not greater than the specified max value
func Max(field string, value, max int) error {
	if value > max {
		return errors.ValidationError(
			"max_value_exceeded",
			field+" must be at most "+string(rune(max)),
		)
	}
	return nil
}

// Min validates that a number is not less than the specified min value
func Min(field string, value, min int) error {
	if value < min {
		return errors.ValidationError(
			"min_value_not_met",
			field+" must be at least "+string(rune(min)),
		)
	}
	return nil
}