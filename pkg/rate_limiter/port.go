package rate_limiter

import "time"

type RateLimiter interface {
	Allow(key string) (bool, error)
}

type Storer interface {
	CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiresIn time.Duration) (bool, error)
	CheckAndUpdateLeakyBucket(key string, capacity int, leakRate float64, expiresIn time.Duration) (bool, error)
	GetRateByKey(string) (Rate, error)
	DeleteRateByKey(string) error
}
