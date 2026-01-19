package env

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func setRequiredEnvVars(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_VERSION", "1")
	t.Setenv("APP_REDIS_ADDR", "localhost")
}

func TestGetEnv_IsSingletonAndConcurrentSafe(t *testing.T) {
	setRequiredEnvVars(t)
	goroutine := 100

	wg := sync.WaitGroup{}
	wg.Add(goroutine)

	instances := make(chan *Specification, goroutine)

	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			instances <- GetEnv()
		}()
	}
	wg.Wait()
	close(instances)

	var first *Specification
	for instance := range instances {
		if first == nil {
			first = instance
			continue
		}
		require.Same(t, first, instance)
	}
}

func TestGetEnv_SpecificationIsValid(t *testing.T) {
	setRequiredEnvVars(t)
	envObj := GetEnv()

	assert.Equal(t, envObj.Version, 1, "Version")
	assert.Equal(t, envObj.Env, "test", "Env")
	assert.Equal(t, envObj.ServerPort, ":8080", "Server Port")
	assert.Equal(t, envObj.ServerWriteTimeoutInSecond, 10*time.Second, "Server Write Timeout")
	assert.Equal(t, envObj.ServerReadTimeoutInSecond, 10*time.Second, "Server Read Timeout")
	assert.Equal(t, envObj.ServerMaxHeaderBytes, 1048576, "Server Max Header Bytes")
	assert.Equal(t, envObj.RedisAddr, "localhost", "Redis Addr")
	assert.Equal(t, envObj.RedisDb, 0, "Redis DB")
	assert.Equal(t, envObj.RedisPassword, "", "Redis Password")
	assert.Equal(t, envObj.RedisPoolSize, 100, "Redis Pool Size")
	assert.Equal(t, envObj.ConfigFile, "./config.yaml", "Config Dir Path")
}
