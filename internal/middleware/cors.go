package middleware

import "net/http"

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set allowed origins
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Allowed Methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		// Allowed Headers (What the frontend can SEND to us)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		// EXPOSED Headers (What the frontend can READ from us)
		// Without this, the browser hides X-Request-ID from React/Vue app
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		// 5. Handle the "Preflight" request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
