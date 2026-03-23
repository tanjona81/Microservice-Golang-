package utils

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func CreatePipeline(rdb *redis.Client, ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := rdb.Pipeline()
	count := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return count.Val(), nil
}
