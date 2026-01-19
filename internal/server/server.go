package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github/martinmaurice/rlim/internal/server/middleware"
	"github/martinmaurice/rlim/pkg/env"
	"github/martinmaurice/rlim/pkg/rate_limiter"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	DefaultGracefulShutdownTimeout = 10
)

type Config struct {
	port                  string
	readTimeoutInSeconds  time.Duration
	writeTimeoutInSeconds time.Duration
	maxHeaderBytes        int
	handler               *gin.Engine
	servicer              rate_limiter.Servicer
	disableRateLimiter    bool
}

type Option func(config *Config)

func WithDisableRateLimiter(value bool) Option {
	return func(config *Config) {
		config.disableRateLimiter = value
	}
}

func NewServer(servicer rate_limiter.Servicer, opts ...Option) *Config {
	envObj := env.GetEnv()
	c := &Config{
		port:                  envObj.ServerPort,
		readTimeoutInSeconds:  envObj.ServerReadTimeoutInSecond,
		writeTimeoutInSeconds: envObj.ServerWriteTimeoutInSecond,
		maxHeaderBytes:        envObj.ServerMaxHeaderBytes,
		handler:               gin.Default(),
		servicer:              servicer,
		disableRateLimiter:    false,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (s *Config) Run() {
	s.handler.Use(middleware.QueueTimeMiddleware)
	s.handler.Use(middleware.AuthenticationMiddleware)

	if s.disableRateLimiter == false {
		s.handler.Use(middleware.RateLimitAnonymousUserMiddleware(s.servicer))
		s.handler.Use(middleware.RateLimitAuthenticatedUserBasedOnTierMiddleware(s.servicer))
	}

	s.handler.POST("/check", CheckHandler(s.servicer)) // Check if request is allowed, consume token if yes
	s.handler.GET("/status/:id", GetStatusByIdHandler)
	s.handler.DELETE("/reset/:id", DeleteRateByIdHandler)
	s.handler.POST("/configure", UpdateRateConfigHandler)
	s.handler.GET("/health", HealthHandler)
	s.handler.GET("/metrics", MetricHandler)

	srv := &http.Server{
		Addr:           s.port,
		Handler:        s.handler,
		ReadTimeout:    s.readTimeoutInSeconds,
		WriteTimeout:   s.writeTimeoutInSeconds,
		MaxHeaderBytes: s.maxHeaderBytes,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Could not listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop // block until interrupt signal
	slog.Info("shutting down the server...")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultGracefulShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown :%v", err)
	}

	slog.Info("Server exited gracefully")
}
