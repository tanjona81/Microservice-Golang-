package middleware

import (
	"context"
	"example/hello/internal/contextkeys"
	"example/hello/internal/utils"
	"net/http"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use of ULID for sortable tracing
		id := utils.GenerateULID()

		// Create a NEW context with the ID inside it
		// We use a custom 'key' type to avoid collisions (Senior Standard)
		ctx := context.WithValue(r.Context(), contextkeys.RequestIDKey, id)

		// Send the ID back in the Header
		w.Header().Set("X-Request-ID", id)

		// Attach the NEW context to the request and pass it down the chain
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
