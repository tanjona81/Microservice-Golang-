package middleware

import (
	"context"
	"example/hello/internal/contextkeys"
	"example/hello/internal/utils"
	"net/http"
	"strings"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.SendErrors(w, r, http.StatusUnauthorized, "Missing authorization header", nil)
			return
		}

		// Expect "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.SendErrors(w, r, http.StatusUnauthorized, "Invalid authorization format", nil)
			return
		}

		tokenString := parts[1]

		// Validate Token (Logic usually lives in utils/jwt.go)
		claims, err := utils.ValidateJWT(tokenString)
		if err != nil {
			utils.SendErrors(w, r, http.StatusUnauthorized, "TOKEN_EXPIRED", err)
			return
		}

		// Attach UserID to Context
		// We use the same 'contextkeys' pattern to avoid circular dependencies!
		ctx := context.WithValue(r.Context(), contextkeys.ClaimedIDKey, claims)

		// Pass the request down with the identity attached
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
