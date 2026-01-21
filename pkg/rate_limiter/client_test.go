package rate_limiter

import (
	"github.com/stretchr/testify/assert"
	"github/martinmaurice/rlim/pkg/config"
	"github/martinmaurice/rlim/pkg/enum"
	"testing"
)

func TestClient_CheckRateLimit(t *testing.T) {

	tests := []struct {
		name           string
		key            string
		rateLimitersId string
		allow          bool
		expectedKey    string
	}{
		{
			name:           "Allow because the bucket has enough token",
			key:            "k1",
			rateLimitersId: "test1",
			allow:          true,
			expectedKey:    "k1:test1",
		},
		{
			name:           "Disallow because the bucket is empty",
			key:            "k1",
			rateLimitersId: "test1",
			expectedKey:    "k1:test1",
			allow:          false,
		},
		{
			name:           "Allow because the buckets are empty",
			key:            "k1",
			rateLimitersId: "test2",
			expectedKey:    "k1:test2",
			allow:          true,
		},
		{
			name:           "Disallow because one of the buckets is full",
			key:            "k1",
			rateLimitersId: "test2",
			expectedKey:    "k1:test2",
			allow:          false,
		},
		{
			name:           "Allow because the is not known",
			key:            "",
			rateLimitersId: "test2",
			expectedKey:    "",
			allow:          true,
		},
		{
			name:           "Allow because the rateLimiters key does not exist",
			key:            "k1",
			rateLimitersId: "",
			expectedKey:    "",
			allow:          true,
		},
	}

	c := &Client{
		rateStorage: NewMemoryStorage(),
	}

	c.rateLimiters = map[string][]rateLimiterWithID{
		"test1": {
			{
				id: "rpm",
				rl: c.newRateLimiter(config.RateLimiterConfig{
					ID:         "rpm",
					Algorithm:  enum.TokenBucket,
					Capacity:   1,
					RefillRate: .1,
				}),
			},
		},
		"test2": {
			{
				id: "rpm",
				rl: c.newRateLimiter(config.RateLimiterConfig{
					ID:         "rpm",
					Algorithm:  enum.LeakyBucket,
					Capacity:   1,
					RefillRate: .1,
				}),
			},
			{
				id: "rph",
				rl: c.newRateLimiter(config.RateLimiterConfig{
					ID:         "rph",
					Algorithm:  enum.LeakyBucket,
					Capacity:   4,
					RefillRate: 1,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, allowed := c.CheckRateLimit(tt.key, tt.rateLimitersId)
			assert.Equal(t, tt.expectedKey, key)
			assert.Equalf(
				t,
				tt.allow,
				allowed,
				"CheckRateLimit(%v, %v)",
				tt.key,
				tt.rateLimitersId,
			)
		})
	}
}
