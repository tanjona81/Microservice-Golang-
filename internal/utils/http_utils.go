package utils

import (
	"context"
	"encoding/json"
	"errors"
	"example/hello/internal/contextkeys"
	"example/hello/internal/domain"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

var retryDuration = "30"

// SetupErrors is called ONCE in main.go
func SetupErrors(timeout string) {
	retryDuration = timeout
}

type ServerResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
	Message   string      `json:"message,omitempty"`
	Code      int         `json:"code"`
	RequestId string      `json:"request_id,omitempty"`
}

// SendSuccess is a helper to keep success responses consistent
func SendSuccess(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := ServerResponse{
		Status: "success",
		Data:   data,
		Code:   code,
	}

	// Sends the data body
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If encoding fails, we've already sent the header
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SendSuccessWithMetadata is a helper to keep success responses consistent with metadata
func SendSuccessWithMetadata(w http.ResponseWriter, code int, data interface{}, metadata interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := ServerResponse{
		Status:   "success",
		Metadata: metadata,
		Data:     data,
		Code:     code,
	}

	json.NewEncoder(w).Encode(response)
}

// SendErrors is a helper to keep error responses consistent
func SendErrors(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, internalErr error) {
	// Extract RequestID
	reqID, _ := r.Context().Value(contextkeys.RequestIDKey).(string)

	// Structured Logging with slog
	if internalErr != nil {
		slog.Debug("sendErrors utility", "statusCode", statusCode)
		if statusCode >= 500 {
			slog.Error("API Server Error",
				"status", statusCode,
				"user_msg", userMsg,
				"error", internalErr.Error(), // Log the raw technical error
				"request_id", reqID,
				"path", r.URL.Path,
				"method", r.Method,
			)
		} else if statusCode >= 400 {
			// 4xx errors are usually the client's fault, so we log as Warn
			slog.Warn("API Client Error",
				"status", statusCode,
				"user_msg", userMsg,
				"error", internalErr.Error(), // Log the raw technical error
				"request_id", reqID,
				"path", r.URL.Path,
				"method", r.Method,
			)
		}
	}

	response := ServerResponse{
		Status:    "error",
		Code:      statusCode,
		Message:   userMsg,
		RequestId: reqID,
	}

	// Add the Retry-After header for 503s
	// This tells Load Balancers and Mobile Apps when to try again.
	if statusCode == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", retryDuration) // 30 seconds
	}

	// Send the clean message to the user
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// In a central place
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	// Extract RequestID
	reqID, _ := r.Context().Value(contextkeys.RequestIDKey).(string)

	// Check for Context Cancellation FIRST
	if errors.Is(err, context.Canceled) {
		slog.Info("Request cancelled by the client",
			"request_id", reqID,
			"path", r.URL.Path,
			"method", r.Method,
		)

		// Return immediately. Do not write a header or body.
		return
	}

	// Timeout (Gateway Timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Request timed out",
			"request_id", reqID,
			"path", r.URL.Path,
		)
		SendErrors(w, r, http.StatusGatewayTimeout, "The request took too long to process", err)
		return
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		slog.Debug("HandleError utility", "Error", appErr.Err)
		SendErrors(w, r, appErr.Code, appErr.Message, appErr.Err)
		return
	}
	// Default fallback
	SendErrors(w, r, http.StatusInternalServerError, "Internal Server Error", err)
}

// JSONDecoder strictly enforces the type T and returns it initialized.
func JSONDecoder[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var payload T

	// We limit the size of the request body to prevent "Big JSON" attacks
	// (1MB limit)
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		HandleError(w, r, &domain.AppError{
			Err:     err,
			Message: "Malformed JSON request body",
			Code:    http.StatusBadRequest,
		})
		return payload, err
	}

	return payload, nil
}

// We identify by IP. In a real production setup (behind Nginx),
// we would look at the "X-Forwarded-For" header.
// Use net.SplitHostPort to get only the IP
func GetClientIP(r *http.Request) string {
	// 1. Check for X-Forwarded-For (standard for proxies)
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// The header can be a comma-separated list: "client, proxy1, proxy2"
		// We only want the first one (the actual client)
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// 2. Check for X-Real-IP (used by Nginx sometimes)
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// 3. Fallback to RemoteAddr (and strip the port!)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return host // If no port exists, return as is
	}
	return host
}
