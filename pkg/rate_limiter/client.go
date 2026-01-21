package rate_limiter

import (
	"fmt"
	"github/martinmaurice/rlim/pkg/config"
	"github/martinmaurice/rlim/pkg/enum"
	"log"
	"log/slog"
	"time"
)

type Servicer interface {
	CheckRateLimit(key string, rateLimitersId string) (string, bool)
	GetRate(key string) (Rate, error)
}

type rateLimiterWithID struct {
	id string
	rl RateLimiter
}

type Client struct {
	rateStorage  Storer
	cfg          *config.Config
	rateLimiters map[string][]rateLimiterWithID
}

func New() *Client {
	var c Client
	c.cfg = config.GetConfig()
	c.rateStorage = NewRedis()
	c.setTierRateLimiters()
	return &c
}

func (c *Client) newRateLimiter(rateLimiterConfig config.RateLimiterConfig) RateLimiter {
	switch rateLimiterConfig.Algorithm {
	case enum.TokenBucket:
		return NewTokenBucket(c.rateStorage, &TokenBucket{
			Capacity:   rateLimiterConfig.Capacity,
			RefillRate: rateLimiterConfig.RefillRate,
			ExpiresIn:  time.Second * time.Duration(rateLimiterConfig.Expiration),
		})
	case enum.LeakyBucket:
		return NewLeakyBucket(c.rateStorage, &LeakyBucket{
			Capacity:  rateLimiterConfig.Capacity,
			LeakRate:  rateLimiterConfig.LeakRate,
			ExpiresIn: time.Second * time.Duration(rateLimiterConfig.Expiration),
		})

	default:
		log.Fatalf("Unknown rate limiter algoritm: %v", rateLimiterConfig.Algorithm)
	}

	return nil
}

func (c *Client) setTierRateLimiters() {
	c.rateLimiters = make(map[string][]rateLimiterWithID)
	for k, rateLimitersCfg := range c.cfg.RateLimiters {
		for _, rl := range rateLimitersCfg {
			c.rateLimiters[k] = append(c.rateLimiters[k], rateLimiterWithID{
				id: rl.ID,
				rl: c.newRateLimiter(rl),
			})
		}
	}
}

func (c *Client) checkRateLimit(key string, rateLimiter RateLimiter) bool {
	ok, err := rateLimiter.Allow(key)
	if err != nil {
		slog.Error(fmt.Sprintf("unexpected error happened %v", err))
		return false
	}
	if ok == false {
		return false
	}

	return true
}

func (c *Client) CheckRateLimit(key, rateLimitersId string) (string, bool) {
	if key == "" || rateLimitersId == "" {
		return "", true
	}

	finalKeyPrefix := fmt.Sprintf("%s:%s", key, rateLimitersId)
	for _, rl := range c.rateLimiters[rateLimitersId] {
		finalKey := fmt.Sprintf("%s:%s", finalKeyPrefix, rl.id)
		slog.Info("checking rate limit", "key", finalKey, "rateLimitersId", rateLimitersId)
		if c.checkRateLimit(finalKey, rl.rl) == false {
			return finalKeyPrefix, false
		}
	}

	return finalKeyPrefix, true
}

func (c *Client) GetRate(key string) (Rate, error) {
	return c.rateStorage.GetRateByKey(key)
}
