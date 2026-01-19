package rate_limiter

type TokenBucketHandler interface {
	CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiration int) (bool, error)
}

type TokenBucket struct {
	Capacity         int     // max tokens allowed in the bucket
	RefillRate       float64 // number of tokens refilled per second
	Expiration       int     // refill the bucket with max token when elapsed time since last refill is greater or equal to expiration (in seconds)
	rateLimitHandler TokenBucketHandler
}

func NewTokenBucket(handler TokenBucketHandler, options *TokenBucket) RateLimiter {
	options.rateLimitHandler = handler
	return options
}

func (tb *TokenBucket) Allow(key string) (bool, error) {
	return tb.rateLimitHandler.CheckAndUpdateTokenBucket(key, tb.Capacity, tb.RefillRate, tb.Expiration)
}
