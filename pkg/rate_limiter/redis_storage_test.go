package rate_limiter

import (
	"context"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	bucketSizeRedisFieldName            = "bucket_size"
	tokenBucketLastRefillRedisFieldName = "last_refill_unix"
	leakyBucketLastRefillRedisFieldName = "last_leak_unix"
)

// newTestRedisStorage create a fake redis initialized with the given db
// and returns the fake redis instance and the RedisStorage instance
func newTestRedisStorage(t *testing.T, db map[string]any) (*miniredis.Miniredis, RedisStorage) {
	mr := miniredis.RunT(t)
	if db != nil {
		for key, value := range db {
			switch bucket := value.(type) {
			case redisLeakyBucket:
				mr.HSet(
					key,
					bucketSizeRedisFieldName,
					fmt.Sprintf("%f", bucket.bucketSize),
					leakyBucketLastRefillRedisFieldName,
					fmt.Sprintf("%d", bucket.lastLeakUnix),
				)
			case redisTokenBucket:
				mr.HSet(
					key,
					bucketSizeRedisFieldName,
					fmt.Sprintf("%f", bucket.bucketSize),
					tokenBucketLastRefillRedisFieldName,
					fmt.Sprintf("%d", bucket.lastRefillUnix),
				)
			}
		}
	}

	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		mr.Close()
		rc.Close()
	})

	return mr, RedisStorage{
		dB: rc,
	}
}

func assertBucketSize(t *testing.T, mr *miniredis.Miniredis, key string, expected float64, message string) {
	bucketSize, err := strconv.ParseFloat(mr.HGet(key, bucketSizeRedisFieldName), 10)
	require.NoError(t, err)
	assert.Equal(t, expected, bucketSize, message)
}

func getBucketSize(t *testing.T, mr *miniredis.Miniredis, key string) float64 {
	bucketSize, err := strconv.ParseFloat(mr.HGet(key, bucketSizeRedisFieldName), 10)
	require.NoError(t, err)
	return bucketSize
}

func assertNotAllowed(t *testing.T, ok bool, err error, message string) {
	require.NoError(t, err)
	assert.False(t, ok, message)
}

func assertAllowed(t *testing.T, ok bool, err error, message string) {
	require.NoError(t, err)
	assert.True(t, ok, message)
}

func TestRedisStorage_CheckAndUpdateLeakyBucket(t *testing.T) {
	var (
		capacity  = 2
		leakRate  = 1.0
		expiresIn = time.Millisecond * 100
	)

	tests := []struct {
		id   string
		key  string
		db   map[string]any
		want bool
	}{
		{
			id:   "Allow request because the bucket does not exist",
			key:  "leaky:john",
			db:   make(map[string]any),
			want: true,
		},
		{
			id:  "Allow request because the bucket is not full yet",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": redisLeakyBucket{
					lastLeakUnix: time.Now().Add(-10 * time.Microsecond).Unix(),
					bucketSize:   1,
				},
			},
			want: true,
		},
		{
			id:  "Disallow request because the bucket is full",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": redisLeakyBucket{
					lastLeakUnix: time.Now().Add(-10 * time.Microsecond).Unix(),
					bucketSize:   2,
				},
			},
			want: false,
		},
		{
			id:  "Allow request because the bucket bucketSize have been consumed enough due to elapsed time rule",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": redisLeakyBucket{
					lastLeakUnix: time.Now().Add(-1 * time.Second).Unix(),
					bucketSize:   2,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		_, storage := newTestRedisStorage(t, tt.db)

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(),
				tt.key,
				capacity,
				leakRate,
				expiresIn,
			)
			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want, "want", tt.want, "got", ok)
		})
	}
}

func TestStorageRedis_CheckAndUpdateTokenBucket(t *testing.T) {
	var (
		capacity   = 2
		refillRate = 1.0
		expiresIn  = time.Second * 10
	)

	tests := []struct {
		id                 string
		key                string
		db                 map[string]any
		expectedBucketSize string
		want               bool
	}{
		{
			id:                 "Allow request because the bucket does not exist and so will be created an filled with max tokens",
			key:                "token_bucket:john",
			db:                 make(map[string]any),
			expectedBucketSize: "1",
			want:               true,
		},
		{
			id:  "Allow request because the bucket is not empty yet",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": redisTokenBucket{
					lastRefillUnix: time.Now().Add(-10 * time.Microsecond).Unix(),
					bucketSize:     1,
				},
			},
			expectedBucketSize: "0",
			want:               true,
		},
		{
			id:  "Disallow request because the bucket is empty",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": redisTokenBucket{
					lastRefillUnix: time.Now().Add(-10 * time.Microsecond).Unix(),
					bucketSize:     0,
				},
			},
			expectedBucketSize: "0",
			want:               false,
		},
		{
			id:  "Allow request because the bucket have been refilled with enough bucketSize due to elapsed time rule",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": redisTokenBucket{
					lastRefillUnix: time.Now().Add(-1 * time.Second).Unix(),
					bucketSize:     0,
				},
			},
			expectedBucketSize: "0",
			want:               true,
		},
	}

	for _, tt := range tests {
		_, storage := newTestRedisStorage(t, tt.db)

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateTokenBucket(context.Background(),
				tt.key,
				capacity,
				refillRate,
				expiresIn,
			)

			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want)
		})
	}
}

func TestRedisStorage_TokenBucket_EdgeCases(t *testing.T) {
	t.Run("Exact boundary - exactly enough bucketSize for request", func(t *testing.T) {
		// Set up bucket with exactly 1.0 bucketSize
		key := "token:boundary"
		db := map[string]any{
			key: redisTokenBucket{
				lastRefillUnix: time.Now().Unix(),
				bucketSize:     1.0,
			},
		}

		mr, storage := newTestRedisStorage(t, db)

		// Should allow request when bucketSize exactly equals requestCost
		ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, 10, 1.0, time.Hour)
		assertAllowed(t, ok, err, "Should allow request when bucketSize exactly equals requestCost")

		// Verify bucketSize were consumed
		assertBucketSize(t, mr, key, 0.0, "bucketSize should be 0 after consuming exactly 1.0")
	})

	t.Run("Sequential requests - rapid fire", func(t *testing.T) {
		_, storage := newTestRedisStorage(t, nil)

		key := "token:sequential"
		capacity := 5
		refillRate := 10.0 // 10 bucketSize per second

		ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
		assertAllowed(t, ok, err, "First request should succeed")

		// Rapid sequential requests without time gap
		successCount := 1
		for i := 0; i < 10; i++ {
			ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
			require.NoError(t, err)
			if ok {
				successCount++
			}
		}

		assert.Equal(t, successCount, capacity, "Should allow up to capacity requests before denying")
	})

	t.Run("Refill does not exceed capacity", func(t *testing.T) {
		key := "token:overflow"
		capacity := 10
		refillRate := 100.0 // Very high refill rate

		// Set up bucket with some bucketSize, long time ago
		db := map[string]any{
			key: redisTokenBucket{
				lastRefillUnix: time.Now().Add(-10 * time.Second).Unix(),
				bucketSize:     5.0,
			},
		}

		mr, storage := newTestRedisStorage(t, db)

		ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
		assertAllowed(t, ok, err, "Request should succeed")

		bucketSize := getBucketSize(t, mr, key)
		assert.LessOrEqual(t, bucketSize, float64(capacity), "bucketSize should not exceed capacity")
	})

	t.Run("Zero refill rate - bucketSize never refill", func(t *testing.T) {
		key := "token:zero-refill"
		capacity := 10
		refillRate := 0.0

		// the bucket is initialized with an elapsed time greater than 10s
		// token to refill will be equal to elapsed * refillRate => 10 * 0 = 0
		// so every request will decrement and the bucket is never refilled
		_, storage := newTestRedisStorage(t, map[string]any{
			key: redisTokenBucket{
				lastRefillUnix: time.Now().Add(-10 * time.Second).Unix(),
				bucketSize:     2.0,
			},
		})

		ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
		assertAllowed(t, ok, err, "First request must be allowed given current config")

		ok, err = storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
		assertAllowed(t, ok, err, "Second request must be allowed given current config")

		ok, err = storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
		assertNotAllowed(t, ok, err, "Should deny request when refill rate is 0")
	})

	t.Run("Concurrent requests - race condition test", func(t *testing.T) {
		_, storage := newTestRedisStorage(t, nil)
		key := "token:concurrent"
		capacity := 10
		refillRate := 1.0

		var wg sync.WaitGroup
		successCount := 0
		var countMutex sync.Mutex

		// Launch 20 concurrent requests
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, time.Hour)
				require.NoError(t, err)
				if ok {
					countMutex.Lock()
					successCount++
					countMutex.Unlock()
				}
			})
		}

		wg.Wait()

		// Should allow at most capacity requests
		assert.LessOrEqual(t, successCount, capacity, "Concurrent requests should not exceed capacity")
	})

	t.Run("Buckets are removed when they expires", func(t *testing.T) {
		// GIVEN an empty redis db
		mr, storage := newTestRedisStorage(t, nil)
		key := "token:expired"
		capacity := 1
		refillRate := 0.0
		expiresIn := time.Millisecond * 10

		// WHEN we check request allowance with given config on top
		ok, err := storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, expiresIn)
		assertAllowed(t, ok, err, "Request should success")

		// The bucket which initially contains 1 token is left with no tokens
		assertBucketSize(t, mr, key, 0.0, "Bucket size should be decremented")

		// GIVEN a bucket with Key config to expires
		mr.SetTTL(key, expiresIn)

		// FIRST the bucket exist in the DB
		assert.True(t, mr.Exists(key))

		// WHEN the key expires, it's removed
		mr.FastForward(time.Millisecond * 20)

		// THEN the bucket with key no longer exist
		assert.False(t, mr.Exists(key))

		// AND when we check the request allowance again, it allowed for the same reason describe on top
		ok, err = storage.CheckAndUpdateTokenBucket(context.Background(), key, capacity, refillRate, expiresIn)
		assertAllowed(t, ok, err, "Request should success because previous bucket was removed")
	})
}

func TestRedisStorage_LeakyBucket_EdgeCases(t *testing.T) {
	t.Run("Exact boundary - exactly at capacity", func(t *testing.T) {
		key := "leaky:boundary"
		leakRate := 0.0
		capacity := 2

		// GIVEN redis db
		db := map[string]any{
			key: redisLeakyBucket{
				lastLeakUnix: time.Now().Unix(),
				bucketSize:   1.0,
			},
		}

		mr, storage := newTestRedisStorage(t, db)

		// WHEN we check for the request allowance
		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "Should allow request when result equals capacity")

		// THEN bucket size must have been incremented with one token
		assertBucketSize(t, mr, key, 2.0, "Should increment the bucket size")
	})

	t.Run("Sequential requests track bucket size correctly", func(t *testing.T) {
		mr, storage := newTestRedisStorage(t, nil)

		key := "leaky:sequential"
		maxTokens := 5
		leakRate := 0.5 // 0.5 tokens per second

		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, maxTokens, leakRate, time.Hour)
		assertAllowed(t, ok, err, "First request - creates bucket with 1 token")

		assertBucketSize(t, mr, key, 1.0, "First request should add 1 token")

		// Second request immediately
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, maxTokens, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		assertBucketSize(t, mr, key, 2.0, "Second request should increase to 2 bucketSize")

		// Third request
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, maxTokens, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		assertBucketSize(t, mr, key, 3.0, "Third request should increase to 3 bucketSize")
	})

	t.Run("Leak rate correctly drains bucket over time", func(t *testing.T) {
		key := "leaky:drain"
		maxTokens := 5
		leakRate := 2.0 // 2 bucketSize per second

		// Given db
		db := map[string]any{
			key: redisLeakyBucket{
				lastLeakUnix: time.Now().Add(-1 * time.Second).Unix(),
				bucketSize:   5.0,
			},
		}

		mr, storage := newTestRedisStorage(t, db)

		// After 1 second, 2 tokens should have leaked
		// So bucket should have 3 tokens, allowing a request
		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, maxTokens, leakRate, time.Hour)
		assertAllowed(t, ok, err, "Request should succeed after leak drains bucket")

		// A request being allowed the bucket is filled with one more token
		assertBucketSize(t, mr, key, 4.0, "Bucket should have ~4 bucketSize after leak and request")
	})

	t.Run("Bucket does not go negative", func(t *testing.T) {
		key := "leaky:negative"
		capacity := 3
		leakRate := 10.0 // Very high leak rate

		// Given db
		db := map[string]any{
			key: redisLeakyBucket{
				lastLeakUnix: time.Now().Add(-2 * time.Second).Unix(),
				bucketSize:   7.0,
			},
		}

		mr, storage := newTestRedisStorage(t, db)

		// After 10 seconds with leak rate 10, all bucketSize should have leaked
		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		assertBucketSize(t, mr, key, 1.0, "bucketSize should never be negative")
	})

	t.Run("Zero leak rate - bucket never drains", func(t *testing.T) {
		key := "leaky:no-leak"
		capacity := 2
		leakRate := 0.0

		// Given db
		db := map[string]any{
			key: redisLeakyBucket{
				lastLeakUnix: time.Now().Add(-10 * time.Second).Unix(),
				bucketSize:   0.0,
			},
		}

		_, storage := newTestRedisStorage(t, db)

		// First request
		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		// Second request
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		// Third request should fail - bucket full and no leak
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertNotAllowed(t, ok, err, "Should deny request when bucket is full and leak rate is 0")
	})

	t.Run("Concurrent requests - race condition test", func(t *testing.T) {
		mr, storage := newTestRedisStorage(t, nil)

		key := "leaky:concurrent"
		capacity := 10
		leakRate := 1.0

		var wg sync.WaitGroup
		successCount := 0
		var countMutex sync.Mutex

		// Launch 20 concurrent requests
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
				require.NoError(t, err)
				if ok {
					countMutex.Lock()
					successCount++
					countMutex.Unlock()
				}
			})
		}

		wg.Wait()

		// Should allow at most capacity requests
		assertBucketSize(t, mr, key, 10.0, "")
		assert.LessOrEqual(t, successCount, capacity, "Concurrent requests should not exceed capacity")
	})

	t.Run("Very small leak rate", func(t *testing.T) {
		_, storage := newTestRedisStorage(t, nil)

		key := "leaky:slow-leak"
		capacity := 2
		leakRate := 0.001 // Very slow leak

		// Fill bucket
		ok, err := storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertAllowed(t, ok, err, "")

		// Bucket should be full now
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertNotAllowed(t, ok, err, "Bucket should be full")

		// Even after short wait, minimal leak occurred
		time.Sleep(10 * time.Millisecond)
		ok, err = storage.CheckAndUpdateLeakyBucket(context.Background(), key, capacity, leakRate, time.Hour)
		assertNotAllowed(t, ok, err, "Bucket should still be full with slow leak rate")
	})
}
