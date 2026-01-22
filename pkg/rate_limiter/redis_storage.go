package rate_limiter

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github/martinmaurice/rlim/pkg/enum"
	"github/martinmaurice/rlim/pkg/env"
	"os"
	"time"
)

const (
	tokenBucketAlgorithmRedisLuaScriptPath = "scripts/redis_lua/token_bucket.lua"
	leakyBucketAlgorithmRedisLuaScriptPath = "scripts/redis_lua/leaky_bucket.lua"
)

type RedisStorage struct {
	dB *redis.Client
}

func NewRedis() Storer {
	envObj := env.GetEnv()
	return &RedisStorage{
		dB: redis.NewClient(&redis.Options{
			Addr:     envObj.RedisAddr,
			Password: envObj.RedisPassword,
			DB:       envObj.RedisDb,
			PoolSize: envObj.RedisPoolSize,
		}),
	}
}

func (r *RedisStorage) readLuaScriptSource(algorithm enum.Algorithm) (string, error) {
	var scriptPath string
	switch algorithm {
	case enum.TokenBucket:
		scriptPath = tokenBucketAlgorithmRedisLuaScriptPath
	case enum.LeakyBucket:
		scriptPath = leakyBucketAlgorithmRedisLuaScriptPath
	default:
		return "", errors.New(fmt.Sprintf("unknown algorithm %v", algorithm))
	}

	b, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (r *RedisStorage) CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiresIn time.Duration) (bool, error) {
	scriptSource, err := r.readLuaScriptSource(enum.TokenBucket)
	if err != nil {
		return false, err
	}

	script := redis.NewScript(scriptSource)
	keys := []string{key}

	result, err := script.Run(
		context.Background(),
		r.dB,
		keys,
		capacity,
		refillRate,
		expiresIn,
		time.Now().Unix(),
	).Result()
	if err != nil {
		return false, err
	}

	ok := result.([]interface{})[0].(int64)
	remainingTokens := result.([]interface{})[1].(int64)

	fmt.Printf("[TokenBucket] resetAt: %v, ok: %v - remainingTokens: %v\n", expiresIn, ok, remainingTokens)
	return ok > 0, nil
}

func (r *RedisStorage) CheckAndUpdateLeakyBucket(key string, capacity int, leakRate float64, expiresIn time.Duration) (bool, error) {
	scriptSource, err := r.readLuaScriptSource(enum.LeakyBucket)
	if err != nil {
		return false, err
	}

	script := redis.NewScript(scriptSource)
	keys := []string{key}

	result, err := script.Run(
		context.Background(),
		r.dB,
		keys,
		capacity,
		leakRate,
		expiresIn,
		time.Now().Unix(),
	).Result()
	if err != nil {
		return false, err
	}

	ok := result.([]interface{})[0].(int64)
	remainingTokens := result.([]interface{})[1].(int64)

	fmt.Printf("[Leakybucket] resetAt: %v, ok: %v - bucketSize: %v\n", expiresIn, ok, remainingTokens)
	return ok > 0, nil
}
