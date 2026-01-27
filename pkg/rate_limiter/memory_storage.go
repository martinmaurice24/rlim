package rate_limiter

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"
)

type MemoryStorage struct {
	mu           *sync.Mutex
	db           map[string]any
	expirationDb map[int64][]string
	requestCost  float64
}

type memoryTokenBucket struct {
	lastRefillUnixNano  int64
	bucketSize          float64
	expiredAtInUnixNano int64
}

type memoryLeakyBucket struct {
	lastLeakUnixNano    int64
	bucketSize          float64
	expiredAtInUnixNano int64
}

func NewMemoryStorage() Storer {
	return &MemoryStorage{
		db:           make(map[string]any),
		expirationDb: make(map[int64][]string),
		mu:           &sync.Mutex{},
		requestCost:  1.0,
	}
}
func (m *MemoryStorage) removeExpiredBucket(tickerDuration time.Duration, stop <-chan bool) {
	go func() {
		ticker := time.NewTicker(tickerDuration)
		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				for expiredAt, expiredKeys := range m.expirationDb {
					if time.Now().UnixNano() < expiredAt {
						continue
					}
					for _, key := range expiredKeys {
						delete(m.db, key)
					}
				}
				m.mu.Unlock()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *MemoryStorage) CheckAndUpdateTokenBucket(
	ctx context.Context,
	key string,
	capacity int,
	refillRate float64,
	expiresIn time.Duration,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var (
		now               = time.Now().UnixNano()
		updateTokenBucket = func(bucketSize float64, setExpiration bool) {
			bucket := memoryTokenBucket{
				lastRefillUnixNano: now,
				bucketSize:         bucketSize,
			}
			if setExpiration {
				bucket.expiredAtInUnixNano = time.Now().Add(expiresIn).UnixNano()
			}
			m.db[key] = bucket
			m.expirationDb[bucket.expiredAtInUnixNano] = append(m.expirationDb[bucket.expiredAtInUnixNano], key)
		}
	)

	slog.Debug("looking for bucket with", "key", key)
	match, ok := m.db[key].(memoryTokenBucket)
	if !ok { // if no token_bucket match the key create one
		bucketSize := float64(capacity) - m.requestCost
		slog.Debug("creating new token bucket", "key", key, "bucketSize", bucketSize)
		updateTokenBucket(bucketSize, true) // remove the cost of the ongoing request
		return true, nil
	}

	elapsedSecondsSinceLastRefill := math.Round(time.Now().Sub(time.Unix(0, match.lastRefillUnixNano)).Seconds())
	tokensToRefill := elapsedSecondsSinceLastRefill * refillRate
	bucketSize := math.Min(float64(capacity), tokensToRefill+match.bucketSize)

	if bucketSize >= m.requestCost {
		slog.Debug("refilling token bucket", "key", key, "bucketSize", bucketSize)
		updateTokenBucket(bucketSize-m.requestCost, false)
		return true, nil
	}

	return false, nil
}

func (m *MemoryStorage) CheckAndUpdateLeakyBucket(
	ctx context.Context,
	key string,
	maxTokens int,
	leakRate float64,
	expiresIn time.Duration,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var (
		now               = time.Now().UnixNano()
		updateLeakyBucket = func(bucketSize float64, setExpiration bool) {
			bucket := memoryLeakyBucket{
				lastLeakUnixNano: now,
				bucketSize:       bucketSize,
			}

			if setExpiration {
				bucket.expiredAtInUnixNano = time.Now().Add(expiresIn).UnixNano()
			}
			m.db[key] = bucket
		}
	)

	slog.Debug("looking for bucket with", "key", key)
	match, ok := m.db[key].(memoryLeakyBucket)
	if !ok { // if no leaky_bucket match the key create one
		bucketSize := m.requestCost
		slog.Debug("creating new leaky bucket", "key", key, "bucketSize", bucketSize)
		updateLeakyBucket(bucketSize, true)
		return true, nil
	}

	elapsedSecondsSinceLastLeak := math.Round(time.Now().Sub(time.Unix(0, match.lastLeakUnixNano)).Seconds())
	nbTokensToLeak := elapsedSecondsSinceLastLeak * leakRate
	bucketSize := math.Max(0, match.bucketSize-nbTokensToLeak)

	if n := bucketSize + m.requestCost; n <= float64(maxTokens) {
		slog.Debug("leaking tokens", "key", key, "bucketSize", bucketSize)
		updateLeakyBucket(n, false)
		return true, nil
	}

	return false, nil
}
