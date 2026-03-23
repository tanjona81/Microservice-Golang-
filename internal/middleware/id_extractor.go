package middleware

import (
	"context"
	"example/hello/internal/contextkeys"
	"example/hello/internal/domain"
	"example/hello/internal/utils"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// This middlleware's purpose is to avoid DRY on parsing and handling the ID from the param's path
// Here the context is attached to the request
// Because in Go after the request is finished, it is deleted by the Garbage Collector. That means there is no memory problem.
func UserIDCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get the ID from the Url Path parameter
		// Check if the parameter id has a valid value
		id, errConversion := strconv.Atoi(chi.URLParam(r, "id"))

		if errConversion != nil {
			utils.SendErrors(w, r, http.StatusBadRequest,
				"The provided ID must be a number",
				fmt.Errorf("Middleware id_extractor error: %w: %s", domain.ErrInvalidInput, chi.URLParam(r, "id")))
			return
		}

		// Put the ID into the Context
		ctx := context.WithValue(r.Context(), contextkeys.UserIDKey, id)

		// Pass the NEW request (with the new context) to the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
