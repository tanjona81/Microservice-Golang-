package middleware

import (
	"example/hello/internal/config"
	"example/hello/internal/utils"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func ShadowLimiter(rdb *redis.Client, cfg *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Get IP Address (Simple version)
			ip := utils.GetClientIP(r)
			key := fmt.Sprintf("limit:register:%s", ip)

			// ctx := r.Context()

			// 2. Increment in Redis
			count, err := utils.CreatePipeline(rdb, r.Context(), key, cfg.ShadowLimiter.Window)

			if err != nil {
				// If Redis fails, don't block the user.
				// Log the error and move on (Fail-Open).
				slog.Error("ShadowLimiter: Redis Limiter Error", "err", err)
				next.ServeHTTP(w, r)
				return
			}

			// 3. Check Limit
			if count > int64(cfg.ShadowLimiter.MaxAttempts) {
				utils.SendErrors(w, r, http.StatusTooManyRequests,
					"Too many registration attempts. Please try again in a minute.",
					fmt.Errorf("rate limit exceeded for IP: %s", ip))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
