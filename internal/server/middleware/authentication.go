package middleware

import (
	"github.com/gin-gonic/gin"
	"log/slog"
)

type User struct {
	ApiKey string
	Tier   string
}

// This is for demo purpose
// In production you must not store your api keys here
var users = []User{
	{"live-is-easy-and-hard", "free"},
	{"fight-for-freedom", "premium"},
	{"change-your-perspective", "enterprise"},
}

const (
	apiKeyHeader                   = "X-API-KEY"
	TierContextKey                 = "authenticatedUserTier"
	IsAuthenticatedContextValueKey = "isUserAuthenticated"
)

func AuthenticationMiddleware(c *gin.Context) {
	apiKey := c.GetHeader(apiKeyHeader)
	if apiKey == "" {
		c.Next()
		return
	}

	for _, user := range users {
		if user.ApiKey == apiKey {
			slog.Debug("User is authenticated", "tier", user.Tier)
			c.Set(TierContextKey, user.Tier)
			c.Set(IsAuthenticatedContextValueKey, true)
			break
		}
	}

	c.Next()
}
