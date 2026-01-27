package env

import (
	"errors"
	"github.com/kelseyhightower/envconfig"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type logLevel slog.Level

func (ll *logLevel) Decode(value string) error {
	switch strings.ToLower(value) {
	case "debug":
		*ll = logLevel(slog.LevelDebug)
	case "info":
		*ll = logLevel(slog.LevelInfo)
	case "warn":
		*ll = logLevel(slog.LevelWarn)
	case "error":
		*ll = logLevel(slog.LevelError)

	default:
		return errors.New("log level value must be one of debug, info, warn, error")
	}

	return nil
}

type Specification struct {
	Version string
	Env     string `default:"production"`

	ServerPort                 string        `default:":8080" split_words:"true"`
	ServerReadTimeoutInSecond  time.Duration `default:"10s" split_words:"true"`
	ServerWriteTimeoutInSecond time.Duration `default:"10s" split_words:"true"`
	ServerMaxHeaderBytes       int           `default:"1048576" split_words:"true"`

	RedisAddr     string `required:"true" split_words:"true"`
	RedisPassword string `default:"" split_words:"true"`
	RedisDb       int    `default:"0" split_words:"true"`
	RedisPoolSize int    `default:"100" split_words:"true"`

	UseMemoryStorage bool `default:"false" split_words:"true"`

	ConfigFile string `default:"./config.yaml" split_words:"true"`

	AppName  string   `default:"rlim" split_words:"true"`
	LogLevel logLevel `default:"debug" split_words:"true"`
}

var (
	once        sync.Once
	envInstance Specification
)

func GetEnv() *Specification {
	once.Do(func() {
		slog.Info("initializing env...")
		err := envconfig.Process("rlim", &envInstance)
		if err != nil {
			log.Fatal(err.Error())
		}
	})

	return &envInstance
}
