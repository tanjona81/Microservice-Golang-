package middleware

import (
	"example/hello/internal/config"
	"example/hello/internal/metrics"
	"example/hello/internal/utils"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func BruteForceMiddleware(rdb *redis.Client, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := utils.GetClientIP(r)
			key := fmt.Sprintf("ratelimit:%s", ip)

			// READ ONLY: Just check the current count
			count, err := rdb.Get(r.Context(), key).Int64()
			if err != nil && err != redis.Nil {
				slog.Error("Redis connection error in middleware", "err", err)
			}

			// Check if blocked
			if count >= int64(cfg.Bruteforce.MaxAttempts) {
				// Telemetry
				metrics.Security.LoginAttempts.WithLabelValues("blocked").Inc()

				// Get the actual remaining time from Redis
				remaining, _ := rdb.TTL(r.Context(), key).Result()

				// Format minutes/seconds for the user
				minutes := int(remaining.Minutes())
				if minutes == 0 {
					minutes = 1
				} // Show at least 1 min

				utils.SendErrors(w, r, http.StatusTooManyRequests, fmt.Sprintf("Too many attempts. Try again in %d minutes.", minutes),
					fmt.Errorf("rate limit exceeded for IP: %s", ip))
				return
			}

			// Record the success
			metrics.Security.LoginAttempts.WithLabelValues("allowed").Inc()
			next.ServeHTTP(w, r)
		})
	}
}
