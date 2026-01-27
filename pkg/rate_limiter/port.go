package rate_limiter

import (
	"context"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

type Storer interface {
	CheckAndUpdateTokenBucket(ctx context.Context, key string, capacity int, refillRate float64, expiresIn time.Duration) (bool, error)
	CheckAndUpdateLeakyBucket(ctx context.Context, key string, capacity int, leakRate float64, expiresIn time.Duration) (bool, error)
}
