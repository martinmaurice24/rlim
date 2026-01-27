package rate_limiter

import (
	"context"
	"time"
)

type TokenBucketHandler interface {
	CheckAndUpdateTokenBucket(ctx context.Context, key string, capacity int, refillRate float64, expiresIn time.Duration) (bool, error)
}

type TokenBucket struct {
	Capacity         int           // max tokens allowed in the bucket
	RefillRate       float64       // number of tokens refilled per second
	ExpiresIn        time.Duration // remove the bucket when it expires
	rateLimitHandler TokenBucketHandler
}

func NewTokenBucket(handler TokenBucketHandler, options *TokenBucket) RateLimiter {
	options.rateLimitHandler = handler
	return options
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) (bool, error) {
	return tb.rateLimitHandler.CheckAndUpdateTokenBucket(ctx, key, tb.Capacity, tb.RefillRate, tb.ExpiresIn)
}
