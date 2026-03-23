package middleware

import (
	"example/hello/internal/contextkeys"
	"example/hello/internal/domain"
	"example/hello/internal/utils"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// // Capture the stack trace for debugging
				// stack := make([]byte, 1024*8)
				// stack = stack[:runtime.Stack(stack, false)]

				// log.Printf("[PANIC RECOVERED]\nError: %v\nStack Trace:\n%s", err, stack)

				// DEFENSIVE: Try to get the ID, but don't panic if it's nil
				reqID, ok := r.Context().Value(contextkeys.RequestIDKey).(string)
				if !ok {
					reqID = "unknown" // Fallback if RequestID middleware failed or hasn't run
				}

				// Log the Panic with full context
				// We use slog to make it searchable in Datadog/Kibana
				slog.Error("PANIC RECOVERED",
					"request_id", reqID,
					"error", err,
					"stack", string(debug.Stack()), // This tells you exactly which LINE crashed
				)

				// Respond with a clean 500 JSON (Don't leak the stack trace to the user!)
				utils.SendErrors(w, r, http.StatusInternalServerError, "An unexpected internal error occurred", domain.ErrInternal)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
