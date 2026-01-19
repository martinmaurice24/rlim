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

	if disableRateLimiter {
		slog.Warn("rate limiter is disabled")
	}

	envObj := env.GetEnv()
	slog.Info(fmt.Sprintf("env file %s loaded\nVersion:%d\nEnv:%s", envFilePath, envObj.Version, envObj.Env))

	raterLimiter := rate_limiter.New()

	srv := server.NewServer(raterLimiter, server.WithDisableRateLimiter(disableRateLimiter))
	srv.Run()
}
