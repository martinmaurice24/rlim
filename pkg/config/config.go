package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"github/martinmaurice/rlim/pkg/enum"
	"github/martinmaurice/rlim/pkg/env"
	"log"
	"log/slog"
	"sync"
)

const (
	hourInSeconds   = 3600.0
	minuteInSeconds = 60.0

	requestPerMinRateLimiterKey  = "rpm"
	requestPerHourRateLimiterKey = "rph"
	defaultRateLimiterKey        = "default"
)

type rateLimiterRawConfig struct {
	Algorithm         string
	RequestsPerMinute *int `mapstructure:"requests_per_minute"`
	RequestsPerHour   *int `mapstructure:"requests_per_hour"`
	Capacity          int  `mapstructure:"capacity"`
	Expiration        int  `mapstructure:"expiration"`
}

type rawConfig struct {
	RateLimits struct {
		Default struct {
			Algorithm   string
			Capacity    int
			RefillRate  *float64 `mapstructure:"refill_rate"`
			ConsumeRate *float64 `mapstructure:"consume_rate"`
			Expiration  int
		}
		Items *map[string]rateLimiterRawConfig
	} `mapstructure:"rate_limits"`
	Metrics *struct {
		Enabled *bool
		Path    string
	}
}

func loadRawConfig() (*rawConfig, error) {
	var rc rawConfig
	if err := viper.Unmarshal(&rc); err != nil {
		return nil, err
	}
	return &rc, nil
}

type RateLimiterConfig struct {
	ID          string
	Algorithm   enum.Algorithm
	Capacity    int
	RefillRate  float64 // Token Bucket Specific
	ConsumeRate float64 // Leaky Bucket Specific
	Expiration  int
}

func (r RateLimiterConfig) validate() error {
	if r.ID == "" {
		return errors.New("rate limiter id is required")
	}

	if r.Expiration <= 0 {
		return errors.New("rate limiter expiration must not be less than or equal to zero")
	}

	if r.Capacity <= 0 {
		return errors.New("rate limiter capacity must not be less than or equal to zero")
	}

	if r.Algorithm == enum.LeakyBucket {
		if r.ConsumeRate <= 0 {
			return errors.New("rate limiter consume_rate must not be less than or equal to zero")
		}
	}

	if r.Algorithm == enum.TokenBucket {
		if r.RefillRate <= 0 {
			return errors.New("rate limiter refill_rate must not be less than or equal to zero")
		}
	}

	return nil
}

type metricConfig struct {
	Enabled bool
	Path    string
}

type Config struct {
	RateLimiters map[string][]RateLimiterConfig
	Metrics      metricConfig
}

func parseAlgorithmConfig(algorithm string) enum.Algorithm {
	switch algorithm {
	case "token_bucket":
		return enum.TokenBucket
	case "leaky_bucket":
		return enum.LeakyBucket
	default:
		return enum.TokenBucket
	}
}

func parseRateLimiterConfig(rlCfg rateLimiterRawConfig) []RateLimiterConfig {
	var (
		rateLimiters []RateLimiterConfig
		algorithm    = parseAlgorithmConfig(rlCfg.Algorithm)
		capacity     = rlCfg.Capacity
		expiration   = rlCfg.Expiration
	)

	createNewRateLimiter := func(id string, refillOrConsumeRate float64) RateLimiterConfig {
		rateLimitConfig := RateLimiterConfig{
			ID:         id,
			Algorithm:  algorithm,
			Capacity:   capacity,
			Expiration: expiration,
		}

		if algorithm == enum.TokenBucket {
			rateLimitConfig.RefillRate = refillOrConsumeRate
		} else if algorithm == enum.LeakyBucket {
			rateLimitConfig.ConsumeRate = refillOrConsumeRate
		}

		return rateLimitConfig
	}

	if rlCfg.RequestsPerMinute != nil {
		refillOrConsumeRate := float64(*rlCfg.RequestsPerMinute) / minuteInSeconds
		rateLimiters = append(rateLimiters, createNewRateLimiter(requestPerMinRateLimiterKey, refillOrConsumeRate))
	}

	if rlCfg.RequestsPerHour != nil {
		refillOrConsumeRate := float64(*rlCfg.RequestsPerHour) / hourInSeconds
		rateLimiters = append(rateLimiters, createNewRateLimiter(requestPerHourRateLimiterKey, refillOrConsumeRate))
	}

	return rateLimiters
}

func parseMetricConfig(rc *rawConfig) (*metricConfig, error) {
	metrics := metricConfig{
		Enabled: true,
		Path:    "/metrics",
	}

	if rc.Metrics != nil {
		if rc.Metrics.Path == "" {
			return nil, errors.New("metrics path could not be empty")
		}

		metrics.Path = rc.Metrics.Path

		if rc.Metrics.Enabled != nil {
			metrics.Enabled = *rc.Metrics.Enabled
		}
	}

	return &metrics, nil
}

func parseDefaultRateLimiterConfig(rc *rawConfig) (*RateLimiterConfig, error) {
	defaultAlgorithm := parseAlgorithmConfig(rc.RateLimits.Default.Algorithm)
	defaultRateLimiter := RateLimiterConfig{
		ID:         defaultRateLimiterKey,
		Algorithm:  defaultAlgorithm,
		Capacity:   rc.RateLimits.Default.Capacity,
		Expiration: rc.RateLimits.Default.Expiration,
	}

	if defaultAlgorithm == enum.TokenBucket {
		if rc.RateLimits.Default.RefillRate == nil {
			return nil, errors.New("you must specify the refill_rate for the default rate limiter")
		}
		defaultRateLimiter.RefillRate = *rc.RateLimits.Default.RefillRate

	} else if defaultAlgorithm == enum.LeakyBucket {
		if rc.RateLimits.Default.ConsumeRate == nil {
			return nil, errors.New("you must specify the consume_rate for the default rate limiter")
		}
		defaultRateLimiter.ConsumeRate = *rc.RateLimits.Default.ConsumeRate
	}

	return &defaultRateLimiter, nil
}

func parseRawConfig(rc *rawConfig) (*Config, error) {
	defaultRateLimiter, err := parseDefaultRateLimiterConfig(rc)
	if err != nil {
		return nil, err
	}

	rateLimitersMap := map[string][]RateLimiterConfig{
		defaultRateLimiterKey: {*defaultRateLimiter},
	}

	for k, rateLimiterCfg := range *rc.RateLimits.Items {
		var (
			rateLimiters []RateLimiterConfig
		)

		rateLimiters = parseRateLimiterConfig(rateLimiterCfg)
		if len(rateLimiters) > 0 {
			rateLimitersMap[k] = rateLimiters
		}
	}

	metric, err := parseMetricConfig(rc)
	if err != nil {
		return nil, err
	}

	return &Config{
		RateLimiters: rateLimitersMap,
		Metrics:      *metric,
	}, nil
}

var (
	once           sync.Once
	configInstance *Config
)

func newConfig() (*Config, error) {
	slog.Info("loading config")
	viper.SetConfigFile(env.GetEnv().ConfigFile)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	rawConfig, err := loadRawConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to read raw config, %v", err)
	}

	return parseRawConfig(rawConfig)
}

func GetConfig() *Config {
	once.Do(func() {
		var err error
		configInstance, err = newConfig()
		if err != nil {
			log.Fatalf("Could not create new config err: %v", err)
		}
	})
	return configInstance
}
