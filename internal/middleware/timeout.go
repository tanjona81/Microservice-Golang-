package middleware

import (
	"context"
	"net/http"
	"time"
)

func Timeout(timer time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a context that automatically cancels after 30 seconds
			ctx, cancel := context.WithTimeout(r.Context(), timer)
			defer cancel() // frees up resources

			// Pass the request with the "Time Bomb" context down the chain
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
