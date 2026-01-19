package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github/martinmaurice/rlim/pkg/enum"
	"sync"
	"testing"
)

func setRequiredEnvVars(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_VERSION", "1")
	t.Setenv("APP_REDIS_ADDR", "localhost")
	t.Setenv("APP_CONFIG_FILE", "./test/config.yaml")
}

func TestGetConfig_IsSingletonAndConcurrentSafe(t *testing.T) {
	setRequiredEnvVars(t)
	goroutine := 100

	wg := sync.WaitGroup{}
	wg.Add(goroutine)

	instances := make(chan *Config, goroutine)

	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			instances <- GetConfig()
		}()
	}
	wg.Wait()
	close(instances)

	var first *Config
	for instance := range instances {
		if first == nil {
			first = instance
			continue
		}
		require.Same(t, first, instance)
	}
}

func TestParseAlgorithm(t *testing.T) {
	tests := []struct {
		input    string
		expected enum.Algorithm
	}{
		{input: "token_bucket", expected: enum.TokenBucket},
		{input: "leaky_bucket", expected: enum.LeakyBucket},
		{input: "unknown", expected: enum.TokenBucket},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, parseAlgorithmConfig(tt.input))
		})
	}
}

func TestGetConfig_IsParsingValid(t *testing.T) {
	setRequiredEnvVars(t)
	cfg := GetConfig()

	expectedRateLimiter := map[string][]RateLimiterConfig{
		"default": []RateLimiterConfig{
			{
				ID:         "default",
				Algorithm:  enum.TokenBucket,
				Capacity:   100,
				RefillRate: 10,
				Expiration: 3600,
			},
		},
		"leaky_bucket": []RateLimiterConfig{
			{
				ID:          "rpm",
				Algorithm:   enum.LeakyBucket,
				Capacity:    10,
				ConsumeRate: 1,
				Expiration:  3600,
			},
			{
				ID:          "rph",
				Algorithm:   enum.LeakyBucket,
				Capacity:    10,
				ConsumeRate: 0.2777777777777778,
				Expiration:  3600,
			},
		},
		"token_bucket": []RateLimiterConfig{
			{
				ID:         "rpm",
				Algorithm:  enum.TokenBucket,
				Capacity:   10,
				RefillRate: 1,
				Expiration: 3600,
			},
			{
				ID:         "rph",
				Algorithm:  enum.TokenBucket,
				Capacity:   10,
				RefillRate: 0.2777777777777778,
				Expiration: 3600,
			},
		},
	}

	assert.True(t, cfg.Metrics.Enabled, "metric should be enabled")
	assert.Equal(t, "/metrics", cfg.Metrics.Path, "metric path is not expected")
	assert.Equal(t, expectedRateLimiter, cfg.RateLimiters, "rate limiters parsing does not meet expectations")
}
