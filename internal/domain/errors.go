package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// Define standard "Sentinel" errors
var (
	ErrNotFound            = errors.New("resource not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrInternal            = errors.New("internal server error")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTokenExpired        = errors.New("token expired")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrUnavailableServices = errors.New("system is temporarily overloaded")
	ErrConflict            = errors.New("conflict")
	ErrCompromisedSession  = errors.New("compromised session")
	ErrDuplicateEmail      = errors.New("email already registered")
	ErrTooManyAttempts     = errors.New("too many attempts")
)

// AppError is a custom struct for more context
type AppError struct {
	Err     error
	Message string
	Code    int // Optional: custom application code
}

// Implementation
// We combine the custom message and the underlying error for better logging.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap allows Go's errors package to access the underlying error.
// This is what makes errors.Is(err, domain.ErrNotFound) work
func (e *AppError) Unwrap() error {
	return e.Err
}

func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	// The Whitelist: Logic errors that should NEVER be retried
	logicErrors := []error{
		ErrNotFound,
		ErrInvalidCredentials,
		ErrInvalidInput,
		ErrTokenExpired,
		ErrUnauthorized,
		ErrConflict,
		ErrCompromisedSession,
		ErrDuplicateEmail,
	}

	for _, target := range logicErrors {
		if errors.Is(err, target) {
			return false // Stop immediately, do not retry
		}
	}

	// The Transients: System errors that SHOULD be retried
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		// Only retry if it's a 500+ error, but NOT a logic error
		return appErr.Code >= 500 && appErr.Code != 503 // 503 is breaker open
	}

	// Default: If we don't know what it is, don't risk an infinite loop
	return false
}

func WrapRepositoryError(err error, notFound bool, internalError error) error {
	slog.Debug("Not found", "value", notFound)
	if err == nil {
		return nil
	}

	// Handle Universal Infrastructure Errors
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// Return naked to avoid masking for the global handler
		return err

	case notFound, errors.Is(err, sql.ErrNoRows):
		slog.Debug("Not found true")
		return ErrNotFound

	default:
		// Everything else (DB connection lost, disk full, etc.)
		return internalError
	}
}

func WrapUniversalError(err error, notFoundMsg string) error {
	if err == nil {
		return nil
	}

	// If it's ALREADY a custom AppErrors, don't wrap it again
	var appErr *AppError
	if errors.As(err, &appErr) {
		return err
	}

	// Handle Universal Infrastructure Errors
	switch {
	case errors.Is(err, ErrNotFound):
		return NewNotFoundError(err, notFoundMsg)

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// Return naked to avoid masking for the global handler
		return err

	default:
		// Everything else (DB connection lost, disk full, etc.)
		return NewInternalError(err)
	}
}

func NewNotFoundError(err error, msg string) *AppError {
	return &AppError{
		Err:     err,
		Message: msg,
		Code:    http.StatusNotFound,
	}
}

func NewInvalidInputError(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "invalid parameter",
		Code:    http.StatusBadRequest,
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "An internal error occurred",
		Code:    http.StatusInternalServerError,
	}
}

func NewConflict(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "Resource already exists",
		Code:    http.StatusConflict,
	}
}

func NewInvalidCredential(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "Invalid credentials",
		Code:    http.StatusUnauthorized,
	}
}

func NewTokenExpired(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "The session has expired",
		Code:    http.StatusUnauthorized,
	}
}

func NewUnauthorizedError(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "Unauthorized",
		Code:    http.StatusUnauthorized,
	}
}

func NewUnavailableServices(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "System is temporarily overloaded",
		Code:    http.StatusServiceUnavailable,
	}
}

func NewCompromisedSession(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "Session has expired or is invalid. Please log in again.",
		Code:    http.StatusConflict,
	}
}

func NewDuplicateEmailError(err error) *AppError {
	return &AppError{
		Err:     err,
		Message: "This email is already in use.",
		Code:    http.StatusConflict,
	}
}
