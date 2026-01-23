package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github/martinmaurice/rlim/pkg/enum"
	"os"
	"sync"
	"testing"
)

func setRequiredEnvVars(t *testing.T) {
	t.Setenv("RLIM_ENV", "test")
	t.Setenv("RLIM_VERSION", "1")
	t.Setenv("RLIM_REDIS_ADDR", "localhost")
	t.Setenv("RLIM_CONFIG_FILE", "./test/config.yaml")
}

func TestGetConfig_IsSingletonAndConcurrentSafe(t *testing.T) {
	setRequiredEnvVars(t)
	goroutine := 100

	wg := sync.WaitGroup{}
	instances := make(chan *Config, goroutine)

	for i := 0; i < goroutine; i++ {
		wg.Go(func() {
			instances <- GetConfig()
		})
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

	resetConfigForTests()
}

func TestGetConfig(t *testing.T) {
	var (
		configOk = `
rate_limits:
  default:
    algorithm: token_bucket
    capacity: 100
    refill_rate: 10
    expiration: 3600

  items:
    free:
      algorithm: token_bucket
      requests_per_minute: 60
      requests_per_hour: 1000
      capacity: 10
      expiration: 300

    login:
      algorithm: leaky_bucket
      requests_per_minute: 60
      capacity: 10
      expiration: 400

metrics:
  enabled: true
  path: "/metrics"
`
		configWithoutRateLimitsSection = `
metrics:
  enabled: true
  path: "/metrics"
`
		configMissingRateLimitsDefault = `
rate_limits:
  items:
    free:
      algorithm: token_bucket
      requests_per_minute: 60
      requests_per_hour: 1000
      capacity: 10
      expiration: 20

metrics:
  enabled: true
  path: "/metrics"
`
		configMissingRateLimitsItems = `
rate_limits:
  default:
    algorithm: leaky_bucket
    capacity: 10
    leak_rate: 10
    expiration: 3600
metrics:
  enabled: false
  path: "/the-metrics"
`
		configMissingMetricSection = `
rate_limits:
  default:
    algorithm: token_bucket
    capacity: 10
    refill_rate: 10
    expiration: 3600
`
		configWithUnknownAlgorithm = `
rate_limits:
  default:
    algorithm: unknown_algorithm
    capacity: 10
    refill_rate: 10
    expiration: 3600
`

		configWithItemsMissingBothRequestsPerMinuteAndRequestsPerHour = `
rate_limits:
  default:
    algorithm: token_bucket
    capacity: 10
    refill_rate: 10
    expiration: 3600

  items:
    toto:
      algorithm: token_bucket
      capacity: 10
      expiration: 20
`
	)

	tests := []struct {
		name              string
		configFileContent string
		expectedConfig    *Config
		wantError         bool
		expectedError     error
	}{
		{
			name:          "Config file content is empty",
			wantError:     true,
			expectedError: RawConfigStructValidationErr,
		},
		{
			name:              "Config file is ok",
			configFileContent: configOk,
			expectedConfig: &Config{
				RateLimiters: map[string][]RateLimiterConfig{
					"default": {
						{
							ID:         "default",
							Algorithm:  enum.TokenBucket,
							Capacity:   100,
							RefillRate: 10,
							Expiration: 3600,
						},
					},
					"free": {
						{
							ID:         "rpm",
							Algorithm:  enum.TokenBucket,
							Capacity:   10,
							RefillRate: 1,
							Expiration: 300,
						},
						{
							ID:         "rph",
							Algorithm:  enum.TokenBucket,
							Capacity:   10,
							RefillRate: 0.2777777777777778,
							Expiration: 300,
						},
					},
					"login": {
						{
							ID:         "rpm",
							Algorithm:  enum.LeakyBucket,
							Capacity:   10,
							LeakRate:   1,
							Expiration: 400,
						},
					},
				},
				Metrics: metricConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
		},
		{
			name:              "missing rate_limits section",
			configFileContent: configWithoutRateLimitsSection,
			wantError:         true,
			expectedError:     RawConfigStructValidationErr,
		},
		{
			name:              "missing rate_limits.default",
			configFileContent: configMissingRateLimitsDefault,
			wantError:         true,
			expectedError:     RawConfigStructValidationErr,
		},
		{
			name:              "missing rate_limits.items section",
			configFileContent: configMissingRateLimitsItems,
			expectedConfig: &Config{
				RateLimiters: map[string][]RateLimiterConfig{
					"default": {
						{
							ID:         "default",
							Algorithm:  enum.LeakyBucket,
							Capacity:   10,
							LeakRate:   10,
							Expiration: 3600,
						},
					},
				},
				Metrics: metricConfig{
					Enabled: false,
					Path:    "/the-metrics",
				},
			},
		},
		{
			name:              "missing metric section",
			configFileContent: configMissingMetricSection,
			expectedConfig: &Config{
				RateLimiters: map[string][]RateLimiterConfig{
					"default": {
						{
							ID:         "default",
							Algorithm:  enum.TokenBucket,
							Capacity:   10,
							RefillRate: 10,
							Expiration: 3600,
						},
					},
				},
				Metrics: metricConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
		},
		{
			name:              "config using unknown algorithm",
			configFileContent: configWithUnknownAlgorithm,
			wantError:         true,
			expectedError:     RawConfigStructValidationErr,
		},
		{
			name:              "config missing both requests_per_minute and requests_per_hour",
			configFileContent: configWithItemsMissingBothRequestsPerMinuteAndRequestsPerHour,
			wantError:         true,
			expectedError:     RawConfigStructValidationErr,
		},
	}

	t.Run("config file path is wrong", func(t *testing.T) {
		_, err := newConfig("wrong_file_path.yaml")
		require.ErrorIs(t, err, FileReadErr)
	})

	setConfig := func(t *testing.T, content []byte) string {
		f, err := os.CreateTemp("", "test_config_*.yaml")
		require.NoError(t, err)
		_, err = f.Write(content)
		require.NoError(t, err)
		return f.Name()
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFileName := setConfig(t, []byte(tt.configFileContent))
			defer os.Remove(configFileName)

			cfg, err := newConfig(configFileName)
			t.Log(err)
			if tt.wantError {
				require.Errorf(t, err, "should raise an error")
			}
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
			}

			if tt.expectedConfig != nil {
				assert.Equal(t, tt.expectedConfig, cfg)
			}
		})
	}
}
