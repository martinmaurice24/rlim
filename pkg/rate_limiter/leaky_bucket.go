package rate_limiter

import "time"

type LeakyBucketHandler interface {
	CheckAndUpdateLeakyBucket(key string, capacity int, leakRate float64, expiresIn time.Duration) (bool, error)
}

type LeakyBucket struct {
	Capacity         int           // max tokens allowed in the bucket
	LeakRate         float64       // number of tokens refilled per second
	ExpiresIn        time.Duration // refill the bucket with max token when elapsed time since last refill is greater or equal to expiration (in seconds)
	rateLimitHandler LeakyBucketHandler
}

func NewLeakyBucket(handler LeakyBucketHandler, options *LeakyBucket) RateLimiter {
	options.rateLimitHandler = handler
	return options
}

func (lb *LeakyBucket) Allow(key string) (bool, error) {
	return lb.rateLimitHandler.CheckAndUpdateLeakyBucket(key, lb.Capacity, lb.LeakRate, lb.ExpiresIn)
}
