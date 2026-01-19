package env

import (
	"github.com/kelseyhightower/envconfig"
	"log"
	"log/slog"
	"sync"
	"time"
)

type Specification struct {
	Version int
	Env     string `default:"production"`

	ServerPort                 string        `default:":8080" split_words:"true"`
	ServerReadTimeoutInSecond  time.Duration `default:"10s" split_words:"true"`
	ServerWriteTimeoutInSecond time.Duration `default:"10s" split_words:"true"`
	ServerMaxHeaderBytes       int           `default:"1048576" split_words:"true"`

	RedisAddr     string `required:"true" split_words:"true"`
	RedisPassword string `default:"" split_words:"true"`
	RedisDb       int    `default:"0" split_words:"true"`
	RedisPoolSize int    `default:"100" split_words:"true"`

	ConfigFile string `default:"./config.yaml" split_words:"true"`
}

var (
	once        sync.Once
	envInstance Specification
)

func GetEnv() *Specification {
	once.Do(func() {
		slog.Info("initializing env...")
		err := envconfig.Process("app", &envInstance)
		if err != nil {
			log.Fatal(err.Error())
		}
	})

	return &envInstance
}
