package middleware

import (
	"example/hello/internal/contextkeys"
	"example/hello/internal/metrics"
	"example/hello/internal/utils"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// we use embedding to mimic the inheritance from java here
// now this struct have all the methods that ResponseWriter have and is considered a ResponseWriter
// responseWriter is a "Spy" that intercepts the status code because of the forgetful state of the ResponseWriter
// the boolean here is used to prevent double-writing errors
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// when the Handler calls w.WriteHeader, it calls this version first
// in this function, we save the status code before sending it
func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Logger is our Architect-grade middleware
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := utils.GetClientIP(r)

		// Wrap the original writer in our spy
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Pass control to the next handler in the chain
		next.ServeHTTP(wrapped, r)

		// Extract the Request-ID we generated in the previous middleware
		reqID, _ := r.Context().Value(contextkeys.RequestIDKey).(string)

		// Calculate level based on status code
		level := slog.LevelInfo
		if wrapped.status >= 500 {
			level = slog.LevelError
		} else if wrapped.status >= 400 {
			level = slog.LevelWarn
		}

		duration := time.Since(start)

		// Telemetry
		// Record Latency (Histogram)
		metrics.HTTPDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())
		// Record Total Requests (Counter)
		status := strconv.Itoa(wrapped.status)
		metrics.HTTPRequests.WithLabelValues(r.Method, r.URL.Path, status).Inc()

		// Use slog.Info for structured output
		// Every key-value pair here becomes a searchable field in your logs
		slog.Log(r.Context(), level, "HTTP request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(), // Easier to graph than "1.2ms"
			"request_id", reqID,
			"ip", ip,
		)
	})
}
