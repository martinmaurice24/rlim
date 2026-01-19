package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
)

type RateLimitMiddlewareServicer interface {
	CheckRateLimit(key string, tier string) bool
}

const (
	FreeTierRateLimitersId = "free"
	LoginRateLimitersId    = "login"
	DefaultRateLimitersId  = "default"
)

func checkRateLimit(servicer RateLimitMiddlewareServicer, key, rateLimiterId string) bool {
	ok := servicer.CheckRateLimit(key, rateLimiterId)
	if ok {
		slog.Info("Request allowed", "key", key, "rate_limiter_id", rateLimiterId)
		return true
	}

	slog.Info("Request not allowed", "key", key, "rate_limiter_id", rateLimiterId)
	return false
}

func RateLimitAnonymousUserMiddleware(servicer RateLimitMiddlewareServicer) gin.HandlerFunc {
	return func(c *gin.Context) {
		isAuth, exists := c.Get(IsAuthenticatedContextValueKey)
		if exists && isAuth.(bool) == true {
			c.Next()
			return
		}

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
		isAuth, exists := c.Get(IsAuthenticatedContextValueKey)
		if !exists || isAuth.(bool) == false {
			c.Next()
			return
		}

		tier, exists := c.Get(TierContextKey)
		if !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		key := fmt.Sprintf("auth:%s:%s", tier, c.GetHeader(apiKeyHeader))
		if ok := checkRateLimit(servicer, key, tier.(string)); ok {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusTooManyRequests)
	}
}
