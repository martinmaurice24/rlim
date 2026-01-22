package config

import (
	"errors"
	"github.com/go-playground/validator/v10"
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

var (
	RawConfigStructValidationErr             = errors.New("config validation error")
	FileReadErr                              = errors.New("failed to read the config file")
	MissingRefillRateInDefaultRateLimiterErr = errors.New("you must specify the refill_rate for the default rate limiter")
	MissingLeakRateInDefaultRateLimiterErr   = errors.New("you must specify the leak_rate for the default rate limiter")
)

type rateLimiterRawConfig struct {
	Algorithm         string `validate:"required,oneof=token_bucket leaky_bucket"`
	RequestsPerMinute *int   `mapstructure:"requests_per_minute" validate:"required_without=RequestsPerHour"`
	RequestsPerHour   *int   `mapstructure:"requests_per_hour" validate:"required_without=RequestsPerMinute"`
	Capacity          int    `mapstructure:"capacity" validate:"required"`
	Expiration        int    `mapstructure:"expiration" validate:"required"`
}

type rawConfig struct {
	RateLimits struct {
		Default struct {
			Algorithm  string   `validate:"required,oneof=token_bucket leaky_bucket"`
			Capacity   int      `validate:"required"`
			RefillRate *float64 `mapstructure:"refill_rate" validate:"required_if=Algorithm token_bucket"`
			LeakRate   *float64 `mapstructure:"leak_rate" validate:"required_if=Algorithm leaky_bucket"`
			Expiration int      `validate:"required"`
		} `mapstructure:"default"`
		Items map[string]rateLimiterRawConfig `validate:"dive,required"`
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

	validate := validator.New()
	if err := validate.Struct(rc); err != nil {
		return nil, errors.Join(RawConfigStructValidationErr, err)
	}

	return &rc, nil
}

type RateLimiterConfig struct {
	ID         string
	Algorithm  enum.Algorithm
	Capacity   int
	RefillRate float64 // Token Bucket Specific
	LeakRate   float64 // Leaky Bucket Specific
	Expiration int
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

	createNewRateLimiter := func(id string, refillOrLeakRate float64) RateLimiterConfig {
		rateLimitConfig := RateLimiterConfig{
			ID:         id,
			Algorithm:  algorithm,
			Capacity:   capacity,
			Expiration: expiration,
		}

		if algorithm == enum.TokenBucket {
			rateLimitConfig.RefillRate = refillOrLeakRate
		} else if algorithm == enum.LeakyBucket {
			rateLimitConfig.LeakRate = refillOrLeakRate
		}

		return rateLimitConfig
	}

	if rlCfg.RequestsPerMinute != nil {
		refillOrLeakRate := float64(*rlCfg.RequestsPerMinute) / minuteInSeconds
		rateLimiters = append(rateLimiters, createNewRateLimiter(requestPerMinRateLimiterKey, refillOrLeakRate))
	}

	if rlCfg.RequestsPerHour != nil {
		refillOrLeakRate := float64(*rlCfg.RequestsPerHour) / hourInSeconds
		rateLimiters = append(rateLimiters, createNewRateLimiter(requestPerHourRateLimiterKey, refillOrLeakRate))
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
			return nil, MissingRefillRateInDefaultRateLimiterErr
		}
		defaultRateLimiter.RefillRate = *rc.RateLimits.Default.RefillRate

	} else if defaultAlgorithm == enum.LeakyBucket {
		if rc.RateLimits.Default.LeakRate == nil {
			return nil, MissingLeakRateInDefaultRateLimiterErr
		}
		defaultRateLimiter.LeakRate = *rc.RateLimits.Default.LeakRate
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

	if rc.RateLimits.Items != nil {
		for k, rateLimiterCfg := range rc.RateLimits.Items {
			var (
				rateLimiters []RateLimiterConfig
			)

			rateLimiters = parseRateLimiterConfig(rateLimiterCfg)
			if len(rateLimiters) > 0 {
				rateLimitersMap[k] = rateLimiters
			}
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

func newConfig(configFilePath string) (*Config, error) {
	slog.Info("loading config")
	viper.SetConfigFile(configFilePath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.Join(FileReadErr, err)
	}

	rawConfig, err := loadRawConfig()
	if err != nil {
		return nil, err
	}

	return parseRawConfig(rawConfig)
}

func GetConfig() *Config {
	once.Do(func() {
		var err error
		configInstance, err = newConfig(env.GetEnv().ConfigFile)
		if err != nil {
			log.Fatalf("Could not create new config err: %v", err)
		}
	})
	return configInstance
}

func resetConfigForTests() {
	once = sync.Once{}
	configInstance = nil
}
