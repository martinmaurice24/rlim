package rate_limiter

type RateLimiter interface {
	Allow(key string) (bool, error)
}

type Storer interface {
	CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiration int) (bool, error)
	CheckAndUpdateLeakyBucket(key string, maxTokens int, consumeRate float64, expiration int) (bool, error)
	GetRateByKey(string) (Rate, error)
	DeleteRateByKey(string) error
}
