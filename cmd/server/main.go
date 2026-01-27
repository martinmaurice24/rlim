package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"github/martinmaurice/rlim/internal/server"
	"github/martinmaurice/rlim/pkg/env"
	"github/martinmaurice/rlim/pkg/rate_limiter"
	"log/slog"
)

var (
	envFilePath        string
	disableRateLimiter bool
)

func init() {
	flag.StringVar(&envFilePath, "env", "", "Enter the env file path you want to load if any")
	flag.BoolVar(&disableRateLimiter, "disableRateLimiter", false, "Disable the rate limiters")
}

func main() {
	slog.Info("Rate rate_limiter v0")

	flag.Parse()

	if envFilePath != "" {
		slog.Info(fmt.Sprintf("loading env file %s", envFilePath))
		if err := godotenv.Load(envFilePath); err != nil {
			panic(fmt.Errorf("could not be able to load the env file: %v", err))
		}
	}

	envObj := env.GetEnv()
	slog.Info("Env loaded", "envFilePath", envFilePath, "envVersion", envObj.Version, "Environment", envObj.Env)

	if disableRateLimiter {
		slog.Warn("rate limiter is disabled")
	}

	rateLimiter := rate_limiter.New(
		&rate_limiter.ClientOptions{
			UseMemoryStorage: envObj.UseMemoryStorage,
		},
	)

	srv := server.NewServer(rateLimiter, server.WithDisableRateLimiter(disableRateLimiter))
	srv.Run()
}
