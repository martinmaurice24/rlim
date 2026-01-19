package rate_limiter

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestMemoryStorage_IsSingletonAndConcurrentSafe(t *testing.T) {
	goroutine := 100
	wg := sync.WaitGroup{}
	wg.Add(goroutine)

	instances := make(chan Storer, goroutine)
	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			instance := NewMemoryStorage()
			instances <- instance
		}()
	}
	wg.Wait()
	close(instances)

	var first Storer
	for instance := range instances {
		if first == nil {
			first = instance
			continue
		}
		assert.Same(t, first, instance, "should be the same instance")
	}
}

func TestMemoryStorage_CheckAndUpdateLeakyBucket(t *testing.T) {
	var (
		maxTokens   = 2
		consumeRate = 1.0
		expiration  = 0
	)

	tests := []struct {
		id   string
		key  string
		db   map[string]any
		want bool
	}{
		{
			id:   "Allow request because the bucket is empty",
			key:  "leaky:john",
			db:   make(map[string]any),
			want: true,
		},
		{
			id:  "Allow request because the bucket is not full yet",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": leakyBucket{
					lastConsume: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					tokens:      1,
				},
			},
			want: true,
		},
		{
			id:  "Disallow request because the bucket is full",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": leakyBucket{
					lastConsume: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					tokens:      2,
				},
			},
			want: false,
		},
		{
			id:  "Allow request because the bucket tokens have been consumed enough due to elapsed time rule",
			key: "leaky:john",
			db: map[string]any{
				"leaky:john": leakyBucket{
					lastConsume: time.Now().Add(-1 * time.Second).UnixNano(),
					tokens:      2,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		storage := MemoryStorage{
			mu:          &sync.Mutex{},
			db:          tt.db,
			requestCost: 1.0,
		}

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateLeakyBucket(
				tt.key,
				maxTokens,
				consumeRate,
				expiration,
			)
			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want, "want", tt.want, "got", ok)
		})
	}
}

func TestMemoryStorage_CheckAndUpdateTokenBucket(t *testing.T) {
	var (
		maxTokens  = 2
		refillRate = 1.0
		expiration = 0
	)

	tests := []struct {
		id   string
		key  string
		db   map[string]any
		want bool
	}{
		{
			id:   "Allow request because the bucket is full",
			key:  "token_bucket:john",
			db:   make(map[string]any),
			want: true,
		},
		{
			id:  "Allow request because the bucket is not empty yet",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefill: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					tokens:     1,
				},
			},
			want: true,
		},
		{
			id:  "Disallow request because the bucket is empty",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefill: time.Now().Add(-10 * time.Microsecond).UnixNano(),
					tokens:     0,
				},
			},
			want: false,
		},
		{
			id:  "Allow request because the bucket have been refilled with enough tokens due to elapsed time rule",
			key: "token_bucket:john",
			db: map[string]any{
				"token_bucket:john": tokenBucket{
					lastRefill: time.Now().Add(-1 * time.Second).UnixNano(),
					tokens:     0,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		storage := MemoryStorage{
			mu:          &sync.Mutex{},
			db:          tt.db,
			requestCost: 1.0,
		}

		t.Run(tt.id, func(t *testing.T) {
			ok, err := storage.CheckAndUpdateTokenBucket(
				tt.key,
				maxTokens,
				refillRate,
				expiration,
			)
			require.ErrorIs(t, err, nil)
			require.Equal(t, ok, tt.want, "want", tt.want, "got", ok)
		})
	}
}
