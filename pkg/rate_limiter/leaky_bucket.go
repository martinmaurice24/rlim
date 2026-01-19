package rate_limiter

type LeakyBucketHandler interface {
	CheckAndUpdateLeakyBucket(key string, maxTokens int, consumeRate float64, expiration int) (bool, error)
}

type LeakyBucket struct {
	MaxTokens        int     // max tokens allowed in the bucket
	ConsumeRate      float64 // number of tokens refilled per second
	Expiration       int     // refill the bucket with max token when elapsed time since last refill is greater or equal to expiration (in seconds)
	rateLimitHandler LeakyBucketHandler
}

func NewLeakyBucket(handler LeakyBucketHandler, options *LeakyBucket) RateLimiter {
	options.rateLimitHandler = handler
	return options
}

func (lb *LeakyBucket) Allow(key string) (bool, error) {
	return lb.rateLimitHandler.CheckAndUpdateLeakyBucket(key, lb.MaxTokens, lb.ConsumeRate, lb.Expiration)
}
