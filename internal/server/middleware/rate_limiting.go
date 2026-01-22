package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
)

type RateLimitMiddlewareServicer interface {
	CheckRateLimit(key string, tier string) (string, bool)
}

const (
	DefaultRateLimitersId = "default"
)

func checkRateLimit(servicer RateLimitMiddlewareServicer, key, rateLimiterId string) bool {
	_, ok := servicer.CheckRateLimit(key, rateLimiterId)
	if ok {
		slog.Info("Request allowed", "key", key, "rate_limiter_id", rateLimiterId)
		return true
	}

	slog.Info("Request not allowed", "key", key, "rate_limiter_id", rateLimiterId)
	return false
}

func RateLimitAnonymousUserMiddleware(servicer RateLimitMiddlewareServicer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// if the user is authenticated go forward
		isAuth, exists := c.Get(IsAuthenticatedContextValueKey)
		if exists && isAuth.(bool) == true {
			c.Next()
			return
		}

		// forge rate limit key prefix using the ip (you could have used something different)
		key := fmt.Sprintf("anonymous:%s", c.ClientIP())
		if ok := checkRateLimit(servicer, key, DefaultRateLimitersId); ok {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusTooManyRequests)
	}
}

func RateLimitAuthenticatedUserBasedOnTierMiddleware(servicer RateLimitMiddlewareServicer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// if the user is not authenticated call next
		isAuth, exists := c.Get(IsAuthenticatedContextValueKey)
		if !exists || isAuth.(bool) == false {
			c.Next()
			return
		}

		// try to get the user tier, in this context any authenticated user must have a tier
		// if not abort the request
		tier, exists := c.Get(TierContextKey)
		if !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// forge the rate limit bucket key prefix
		// and check whether the request is allowed
		key := fmt.Sprintf("auth:%s:%s", tier, c.GetHeader(apiKeyHeader))
		if ok := checkRateLimit(servicer, key, tier.(string)); ok {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusTooManyRequests)
	}
}
