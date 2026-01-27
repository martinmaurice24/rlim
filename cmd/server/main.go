package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"github/martinmaurice/rlim/internal/server"
	"github/martinmaurice/rlim/pkg/env"
	"github/martinmaurice/rlim/pkg/rate_limiter"
	"log/slog"
	"os"
)

var (
	envFilePath        string
	disableRateLimiter bool
)

func init() {
	flag.StringVar(&envFilePath, "env", "", "Enter the env file path you want to load if any")
	flag.BoolVar(&disableRateLimiter, "disableRateLimiter", false, "Disable the rate limiters")
}

func setupLogger(appName, env, version string, logLevel slog.Level) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	slog.SetDefault(logger.With("appName", appName, "env", env, "version", version))
}

func main() {
	flag.Parse()

	// load the env from the given file if any
	if envFilePath != "" {
		slog.Info(fmt.Sprintf("loading env file %s", envFilePath))
		if err := godotenv.Load(envFilePath); err != nil {
			panic(fmt.Errorf("could not be able to load the env file: %v", err))
		}
	}

	// setup the default logger
	envObj := env.GetEnv()
	setupLogger(
		envObj.AppName,
		envObj.Env,
		envObj.Version,
		slog.Level(envObj.LogLevel),
	)

	if disableRateLimiter {
		slog.Warn("rate limiter is disabled")
	}

	// initialize the rate limiter client
	rateLimiter := rate_limiter.New(
		&rate_limiter.ClientOptions{
			UseMemoryStorage: envObj.UseMemoryStorage,
		},
	)

	srv := server.NewServer(rateLimiter, server.WithDisableRateLimiter(disableRateLimiter))
	srv.Run()
}
