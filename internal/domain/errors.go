package domain

import "errors"

// Common domain errors
var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized is returned when authentication is required
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when access is denied
	ErrForbidden = errors.New("forbidden")

	// ErrConflict is returned when a resource conflict occurs
	ErrConflict = errors.New("resource conflict")

	// ErrPlatformUnavailable is returned when a platform API is unavailable
	ErrPlatformUnavailable = errors.New("platform temporarily unavailable")

	// ErrInsufficientData is returned when there is not enough data
	ErrInsufficientData = errors.New("insufficient data")
)

// UserFriendlyError wraps an error with a user-friendly message
type UserFriendlyError struct {
	Err            error
	UserMessage    string
	HTTPStatusCode int
}

// Error implements the error interface
func (e *UserFriendlyError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.UserMessage
}

// Unwrap returns the underlying error
func (e *UserFriendlyError) Unwrap() error {
	return e.Err
}

// NewUserFriendlyError creates a new user-friendly error
func NewUserFriendlyError(err error, userMessage string, statusCode int) *UserFriendlyError {
	return &UserFriendlyError{
		Err:            err,
		UserMessage:    userMessage,
		HTTPStatusCode: statusCode,
	}
}
