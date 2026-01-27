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

type ClientOptions struct {
	UseMemoryStorage bool
}

func New(options *ClientOptions) *Client {
	slog.Info(
		"creating new rate limiter client instance",
		"useMemoryStorage", options.UseMemoryStorage,
	)
	var c Client
	c.cfg = config.GetConfig()

	if options.UseMemoryStorage {
		slog.Debug("creating the client with memory storage")
		c.rateStorage = NewMemoryStorage()
	} else {
		slog.Debug("creating the client with redis storage")
		c.rateStorage = NewRedis()
	}

	c.setTierRateLimiters()

	slog.Debug("rate limiter client created", "client", c)

	return &c
}

func (c *Client) newRateLimiter(rateLimiterConfig config.RateLimiterConfig) RateLimiter {
	slog.Debug("newRateLimiter", "ID", rateLimiterConfig.ID, "algorithm", rateLimiterConfig.Algorithm)
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
		log.Fatalf("Unknown rate limiter algorithm: %v", rateLimiterConfig.Algorithm)
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
	slog.Debug("checkRateLimit", "key", key, "rateLimiter", rateLimiter)
	ok, err := rateLimiter.Allow(key)
	if err != nil {
		slog.Error("unexpected error while checking request against rate limit", "error", err)
		return false
	}
	if ok == false {
		return false
	}

	return true
}

func (c *Client) CheckRateLimit(key, rateLimitersId string) (string, bool) {
	slog.Info("checking rate limit", "key", key, "rateLimitersId", rateLimitersId)

	if key == "" || rateLimitersId == "" {
		slog.Debug(
			"CheckRateLimit called with empty key or rateLimitersId",
			"key", key,
			"rateLimitersId", rateLimitersId,
		)
		return "", true
	}

	finalKeyPrefix := fmt.Sprintf("%s:%s", key, rateLimitersId)
	for _, rl := range c.rateLimiters[rateLimitersId] {
		finalKey := fmt.Sprintf("%s:%s", finalKeyPrefix, rl.id)
		slog.Debug(
			"checking against",
			"key", finalKey,
			"rateLimiterId", rl.id,
		)

		if c.checkRateLimit(finalKey, rl.rl) == false {
			slog.Debug(
				"request rejected by one of the rate limiter",
				"key", finalKey,
				"rateLimiterID", rl.id,
			)
			return finalKeyPrefix, false
		}
	}

	return finalKeyPrefix, true
}
