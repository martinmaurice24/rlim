package rate_limiter

import (
	"log/slog"
	"math"
	"sync"
	"time"
)

var (
	onceMemoryStorage     = &sync.Once{}
	memoryStorageInstance Storer
)

type MemoryStorage struct {
	mu          *sync.Mutex
	db          map[string]any
	requestCost float64
}

type tokenBucket struct {
	lastRefill int64
	tokens     float64
}

type leakyBucket struct {
	lastConsume int64
	tokens      float64
}

func NewMemoryStorage() Storer {
	onceMemoryStorage.Do(func() {
		memoryStorageInstance = &MemoryStorage{
			db:          make(map[string]any),
			mu:          &sync.Mutex{},
			requestCost: 1.0,
		}
	})
	return memoryStorageInstance
}

func (m *MemoryStorage) CheckAndUpdateTokenBucket(key string, capacity int, refillRate float64, expiration int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var (
		now               = time.Now().UnixNano()
		updateTokenBucket = func(newTokens float64) {
			m.db[key] = tokenBucket{
				lastRefill: now,
				tokens:     newTokens,
			}
		}
	)
	slog.Debug("get bucket", "key", key)
	match, ok := m.db[key].(tokenBucket)
	if !ok { // if no token_bucket match the key create one
		newTokens := float64(capacity) - m.requestCost
		slog.Debug("Creating new token bucket", "key", key, "tokens", newTokens)
		updateTokenBucket(newTokens) // remove the cost of the ongoing request
		return true, nil
	}

	timeElapsedSinceLastRefill := time.Now().Sub(time.Unix(0, match.lastRefill)).Seconds()
	tokensToRefill := timeElapsedSinceLastRefill * refillRate
	newTokens := math.Min(float64(capacity), tokensToRefill+match.tokens)

	if newTokens > m.requestCost {
		slog.Debug("Refilling token bucket", "key", key, "tokens", newTokens)
		updateTokenBucket(newTokens - m.requestCost)
		return true, nil
	}

	return false, nil
}

func (m *MemoryStorage) CheckAndUpdateLeakyBucket(key string, maxTokens int, consumeRate float64, expiration int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var (
		now               = time.Now().UnixNano()
		updateLeakyBucket = func(newTokens float64) {
			m.db[key] = leakyBucket{
				lastConsume: now,
				tokens:      newTokens,
			}
		}
	)

	match, ok := m.db[key].(leakyBucket)
	if !ok { // if no leaky_bucket match the key create one
		newTokens := m.requestCost
		slog.Debug("Creating new leaky bucket", "key", key, "tokens", newTokens)
		updateLeakyBucket(newTokens) // add the cost of the ongoing request
		return true, nil
	}

	timeElapsedSinceLastConsume := time.Now().Sub(time.Unix(0, match.lastConsume)).Seconds()
	tokensToConsume := timeElapsedSinceLastConsume * consumeRate
	newTokens := math.Max(0, match.tokens-tokensToConsume)

	if newTokens+m.requestCost < float64(maxTokens) {
		slog.Debug("Consuming tokens in the leaky bucket", "key", key, "tokens", newTokens)
		updateLeakyBucket(newTokens - m.requestCost)
		return true, nil
	}

	return false, nil
}

func (m *MemoryStorage) GetRateByKey(key string) (Rate, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemoryStorage) DeleteRateByKey(key string) error {
	//TODO implement me
	panic("implement me")
}
