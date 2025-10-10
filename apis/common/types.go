package common

// ErrorResponse represents a standardized error response structure.
// It provides consistent error formatting across all API endpoints.
type ErrorResponse struct {
	// Error indicates whether this is an error response
	Error bool `json:"error"`

	// Message contains the error message description
	Message string `json:"message"`
}
