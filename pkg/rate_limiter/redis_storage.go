package rate_limiter

import (
	"context"
	_ "embed"
	"github.com/redis/go-redis/v9"
	"github/martinmaurice/rlim/pkg/env"
	"log/slog"
	"time"
)

var (
	//go:embed redis_lua/redis_token_bucket.lua
	redisTokenBucketLua string

	//go:embed redis_lua/redis_leaky_bucket.lua
	redisLeakyBucketLua string
)

type redisTokenBucket struct {
	lastRefillUnix int64
	bucketSize     float64
}

type redisLeakyBucket struct {
	lastLeakUnix int64
	bucketSize   float64
}

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

func (r *RedisStorage) CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiresIn time.Duration) (bool, error) {
	script := redis.NewScript(redisTokenBucketLua)
	keys := []string{key}

	result, err := script.Run(
		context.Background(),
		r.dB,
		keys,
		capacity,
		refillRate,
		expiresIn,
		time.Now().Unix(),
	).Int64Slice()
	if err != nil {
		return false, err
	}

	ok := result[0]
	bucketSize := result[1]

	slog.Debug("token bucket", "ok", ok, "bucket_size", bucketSize)
	return ok > 0, nil
}

func (r *RedisStorage) CheckAndUpdateLeakyBucket(key string, capacity int, leakRate float64, expiresIn time.Duration) (bool, error) {
	script := redis.NewScript(redisLeakyBucketLua)
	keys := []string{key}

	result, err := script.Run(
		context.Background(),
		r.dB,
		keys,
		capacity,
		leakRate,
		expiresIn,
		time.Now().Unix(),
	).Int64Slice()
	if err != nil {
		return false, err
	}

	ok := result[0]
	bucketSize := result[1]

	slog.Debug("token bucket", "ok", ok, "bucket_size", bucketSize)
	return ok > 0, nil
}
