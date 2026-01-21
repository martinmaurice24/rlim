package rate_limiter

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func newTestMemoryStorage() MemoryStorage {
	return MemoryStorage{
		mu:           &sync.Mutex{},
		db:           make(map[string]any),
		expirationDb: make(map[int64][]string),
		requestCost:  1.0,
	}
}

func TestMemoryStorage_CheckAndUpdateLeakyBucket(t *testing.T) {
	var (
		maxTokens = 2
		leakRate  = 1.0
		expiresIn = time.Second
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
				"leaky:john": leakyBucket{
					lastLeakUnixNano: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					bucketSize:       1,
				},
			},
			want: true,
		},
		{
			id:  "Disallow request because the bucket is full",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": leakyBucket{
					lastLeakUnixNano: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					bucketSize:       2,
				},
			},
			want: false,
		},
		{
			id:  "Allow request because the bucket bucketSize have been consumed enough due to elapsed time rule",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": leakyBucket{
					lastLeakUnixNano: time.Now().Add(-1 * time.Second).UnixNano(),
					bucketSize:       2,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		storage := newTestMemoryStorage()
		storage.db = tt.db

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateLeakyBucket(
				tt.key,
				maxTokens,
				leakRate,
				expiresIn,
			)
			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want, "want", tt.want, "got", ok)
		})
	}
}

func TestMemoryStorage_CheckAndUpdateTokenBucket(t *testing.T) {
	var (
		capacity   = 2
		refillRate = 1.0
		expiresIn  = time.Millisecond * 200
	)

	tests := []struct {
		id   string
		key  string
		db   map[string]any
		want bool
	}{
		{
			id:   "Allow request because the bucket does not exist and so will be created an filled with max tokens",
			key:  "token_bucket:john",
			db:   make(map[string]any),
			want: true,
		},
		{
			id:  "Allow request because the bucket is not empty yet",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefillUnixNano: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					bucketSize:         1,
				},
			},
			want: true,
		},
		{
			id:  "Disallow request because the bucket is empty",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefillUnixNano: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					bucketSize:         0,
				},
			},
			want: false,
		},
		{
			id:  "Allow request because the bucket have been refilled with enough bucketSize due to elapsed time rule",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefillUnixNano: time.Now().Add(-1 * time.Second).UnixNano(),
					bucketSize:         0,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		storage := newTestMemoryStorage()
		storage.db = tt.db

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateTokenBucket(
				tt.key,
				capacity,
				refillRate,
				expiresIn,
			)
			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want, "want", tt.want, "got", ok)
		})
	}
}

func TestMemoryStorage_TokenBucket_EdgeCases(t *testing.T) {
	t.Run("Exact boundary - exactly enough bucketSize for request", func(t *testing.T) {
		storage := newTestMemoryStorage()

		// Set up bucket with exactly 1.0 bucketSize
		key := "token:boundary"
		storage.db[key] = tokenBucket{
			lastRefillUnixNano: time.Now().UnixNano(),
			bucketSize:         1.0,
		}

		// Should allow request when bucketSize exactly equals requestCost
		ok, err := storage.CheckAndUpdateTokenBucket(key, 10, 1.0, 0)
		require.NoError(t, err)
		assert.True(t, ok, "Should allow request when bucketSize exactly equals requestCost")

		// Verify bucketSize were consumed
		bucket := storage.db[key].(tokenBucket)
		assert.Equal(t, 0.0, bucket.bucketSize, "bucketSize should be 0 after consuming exactly 1.0")
	})

	t.Run("Sequential requests - rapid fire", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "token:sequential"
		capacity := 5
		refillRate := 10.0 // 10 bucketSize per second

		// First request - should create bucket with capacity-1 bucketSize
		ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
		require.NoError(t, err)
		assert.True(t, ok, "First request should succeed")

		// Rapid sequential requests without time gap
		successCount := 1
		for i := 0; i < 10; i++ {
			ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
			require.NoError(t, err)
			if ok {
				successCount++
			}
		}

		// Should allow up to capacity requests (5 total) before denying
		assert.LessOrEqual(t, successCount, capacity, "Should not exceed capacity")
		assert.GreaterOrEqual(t, successCount, capacity-1, "Should allow at least capacity-1 requests")
	})

	t.Run("Refill does not exceed capacity", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "token:overflow"
		capacity := 10
		refillRate := 100.0 // Very high refill rate

		// Set up bucket with some bucketSize, long time ago
		storage.db[key] = tokenBucket{
			lastRefillUnixNano: time.Now().Add(-10 * time.Second).UnixNano(),
			bucketSize:         5.0,
		}

		// Request should succeed
		ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
		require.NoError(t, err)
		assert.True(t, ok)

		// bucketSize should be capped at capacity-1 (after consuming 1)
		bucket := storage.db[key].(tokenBucket)
		assert.LessOrEqual(t, bucket.bucketSize, float64(capacity), "bucketSize should not exceed capacity")
	})

	t.Run("Very small request cost", func(t *testing.T) {
		storage := newTestMemoryStorage()
		storage.requestCost = 0.001

		key := "token:small-cost"
		capacity := 1

		// Should allow multiple requests due to small cost
		for i := 0; i < 5; i++ {
			ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, 1.0, 0)
			require.NoError(t, err)
			assert.True(t, ok, "Request %d should succeed with small cost", i+1)
		}
	})

	t.Run("Zero refill rate - bucketSize never refill", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "token:zero-refill"
		capacity := 2
		refillRate := 0.0

		// First request
		ok, _ := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
		assert.True(t, ok)

		// Second request
		ok, _ = storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
		assert.True(t, ok)

		// Wait some time
		time.Sleep(100 * time.Millisecond)

		// Third request should fail - no refill happened
		ok, _ = storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
		assert.False(t, ok, "Should deny request when refill rate is 0")
	})

	t.Run("Concurrent requests - race condition test", func(t *testing.T) {
		storage := newTestMemoryStorage()
		key := "token:concurrent"
		capacity := 10
		refillRate := 1.0

		var wg sync.WaitGroup
		successCount := 0
		var countMutex sync.Mutex

		// Launch 20 concurrent requests
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, 0)
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
		storage := newTestMemoryStorage()

		stop := make(chan bool)
		defer func() {
			stop <- true
		}()
		storage.removeExpiredBucket(time.Millisecond*100, stop)

		key := "token:expired"
		capacity := 1
		refillRate := 0.0
		expiresIn := time.Millisecond * 50

		ok, err := storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, expiresIn)
		require.NoError(t, err)
		assert.True(t, ok, "Request should success")

		bucket, _ := storage.db[key].(tokenBucket)
		assert.Equal(t, 0.0, bucket.bucketSize, "Bucket size should be incremented")

		time.Sleep(time.Millisecond * 200)

		_, ok = storage.db[key]
		assert.False(t, ok, "Bucket was removed because it expired")

		ok, err = storage.CheckAndUpdateTokenBucket(key, capacity, refillRate, expiresIn)
		require.NoError(t, err)
		assert.True(t, ok, "Request should success because previous bucket was removed")

	})
}

func TestMemoryStorage_LeakyBucket_EdgeCases(t *testing.T) {
	t.Run("Exact boundary - exactly at capacity", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:boundary"
		maxTokens := 2

		// Set up bucket with maxTokens-1 tokens
		storage.db[key] = leakyBucket{
			lastLeakUnixNano: time.Now().UnixNano(),
			bucketSize:       1.0,
		}

		// Should allow request that brings us exactly to capacity
		ok, err := storage.CheckAndUpdateLeakyBucket(key, maxTokens, 1.0, 0)
		require.NoError(t, err)
		assert.True(t, ok, "Should allow request when result equals capacity")

		// Verify bucket is now at capacity
		bucket := storage.db[key].(leakyBucket)
		assert.Equal(t, 2.0, bucket.bucketSize, "Bucket should be at capacity (2.0)")
	})

	t.Run("Sequential requests track bucket size correctly", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:sequential"
		maxTokens := 5
		leakRate := 0.5 // 0.5 tokens per second

		// First request - creates bucket with 1 token
		ok, err := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		require.NoError(t, err)
		assert.True(t, ok)

		bucket := storage.db[key].(leakyBucket)
		assert.Equal(t, 1.0, bucket.bucketSize, "First request should add 1 token")

		// Second request immediately
		ok, err = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		require.NoError(t, err)
		assert.True(t, ok)

		bucket = storage.db[key].(leakyBucket)
		assert.Equal(t, 2.0, bucket.bucketSize, "Second request should increase to 2 bucketSize")

		// Third request
		ok, err = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		require.NoError(t, err)
		assert.True(t, ok)

		bucket = storage.db[key].(leakyBucket)
		assert.Equal(t, 3.0, bucket.bucketSize, "Third request should increase to 3 bucketSize")
	})

	t.Run("Leak rate correctly drains bucket over time", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:drain"
		maxTokens := 5
		leakRate := 2.0 // 2 bucketSize per second

		// Fill bucket to capacity
		storage.db[key] = leakyBucket{
			lastLeakUnixNano: time.Now().Add(-1 * time.Second).UnixNano(),
			bucketSize:       5.0,
		}

		// After 1 second, 2 bucketSize should have leaked
		// So bucket should have 3 bucketSize, allowing a request
		ok, err := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		require.NoError(t, err)
		assert.True(t, ok, "Request should succeed after leak drains bucket")

		bucket := storage.db[key].(leakyBucket)
		// After leak (5 - 2 = 3) and adding request (3 + 1 = 4)
		assert.InDelta(t, 4.0, bucket.bucketSize, 0.1, "Bucket should have ~4 bucketSize after leak and request")
	})

	t.Run("Bucket does not go negative", func(t *testing.T) {
		storage := newTestMemoryStorage()
		key := "leaky:negative"
		maxTokens := 3
		leakRate := 10.0 // Very high leak rate

		// Set up bucket with 1 token, long time ago
		storage.db[key] = leakyBucket{
			lastLeakUnixNano: time.Now().Add(-10 * time.Second).UnixNano(),
			bucketSize:       1.0,
		}

		// After 10 seconds with leak rate 10, all bucketSize should have leaked
		ok, err := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		require.NoError(t, err)
		assert.True(t, ok)

		bucket := storage.db[key].(leakyBucket)
		assert.GreaterOrEqual(t, bucket.bucketSize, 0.0, "bucketSize should never be negative")
	})

	t.Run("Zero leak rate - bucket never drains", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:no-leak"
		maxTokens := 2
		leakRate := 0.0

		// First request
		ok, _ := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.True(t, ok)

		// Second request
		ok, _ = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.True(t, ok)

		// Wait some time
		time.Sleep(100 * time.Millisecond)

		// Third request should fail - bucket full and no leak
		ok, _ = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.False(t, ok, "Should deny request when bucket is full and leak rate is 0")
	})

	t.Run("Concurrent requests - race condition test", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:concurrent"
		maxTokens := 10
		leakRate := 1.0

		var wg sync.WaitGroup
		successCount := 0
		var countMutex sync.Mutex

		// Launch 20 concurrent requests
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				ok, err := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
				require.NoError(t, err)
				if ok {
					countMutex.Lock()
					successCount++
					countMutex.Unlock()
				}
			})
		}

		wg.Wait()

		// Should allow at most maxTokens requests
		assert.LessOrEqual(t, successCount, maxTokens, "Concurrent requests should not exceed capacity")
	})

	t.Run("Very small leak rate", func(t *testing.T) {
		storage := newTestMemoryStorage()

		key := "leaky:slow-leak"
		maxTokens := 2
		leakRate := 0.001 // Very slow leak

		// Fill bucket
		ok, _ := storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.True(t, ok)

		ok, _ = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.True(t, ok)

		// Bucket should be full now
		ok, _ = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.False(t, ok, "Bucket should be full")

		// Even after short wait, minimal leak occurred
		time.Sleep(10 * time.Millisecond)
		ok, _ = storage.CheckAndUpdateLeakyBucket(key, maxTokens, leakRate, 0)
		assert.False(t, ok, "Bucket should still be full with slow leak rate")
	})
}

func TestMemoryStorage_AlgorithmBehaviorComparison(t *testing.T) {
	t.Run("Token bucket allows burst, leaky bucket smooths traffic", func(t *testing.T) {
		tokenStorage := newTestMemoryStorage()
		leakyStorage := newTestMemoryStorage()

		capacity := 5
		rate := 1.0

		tokenSuccessCount := 0
		leakySuccessCount := 0

		// Burst of requests
		for i := 0; i < 10; i++ {
			okToken, _ := tokenStorage.CheckAndUpdateTokenBucket("token:burst", capacity, rate, 0)
			okLeaky, _ := leakyStorage.CheckAndUpdateLeakyBucket("leaky:burst", capacity, rate, 0)

			if okToken {
				tokenSuccessCount++
			}
			if okLeaky {
				leakySuccessCount++
			}
		}

		// Token bucket should allow more initial burst requests
		assert.Equal(t, capacity, tokenSuccessCount, "Token bucket should allow burst up to capacity")
		assert.Equal(t, capacity, leakySuccessCount, "Leaky bucket should also respect capacity")

		// After capacity is exhausted/filled, both should deny
		okToken, _ := tokenStorage.CheckAndUpdateTokenBucket("token:burst", capacity, rate, 0)
		okLeaky, _ := leakyStorage.CheckAndUpdateLeakyBucket("leaky:burst", capacity, rate, 0)

		assert.False(t, okToken, "Token bucket should deny after exhausting bucketSize")
		assert.False(t, okLeaky, "Leaky bucket should deny when full")
	})
}
