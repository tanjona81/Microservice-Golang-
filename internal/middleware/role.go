package middleware

import (
	"example/hello/internal/contextkeys"
	"example/hello/internal/utils"
	"net/http"
)

// EnsureRole checks if the user has a specific role slug in their JWT
func EnsureRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Get claims from context (placed there by Auth middleware)
			claims, ok := r.Context().Value(contextkeys.ClaimedIDKey).(*utils.CustomClaims)
			if !ok {
				utils.SendErrors(w, r, http.StatusUnauthorized, "Unauthorized: No claims found", nil)
				return
			}

			// 2. Check if the required role exists in the user's roles slice
			hasRole := false
			for _, role := range claims.Roles {
				if role == requiredRole {
					hasRole = true
					break
				}
			}

			// 3. If they don't have the role, return 403 Forbidden
			if !hasRole {
				utils.SendErrors(w, r, http.StatusForbidden, "Forbidden: Insufficient permissions", nil)
				return
			}

			// 4. Authorized! Proceed to the next handler
			next.ServeHTTP(w, r)
		})
	}
}
