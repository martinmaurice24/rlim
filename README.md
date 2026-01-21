# rlim

> A flexible, rate limiting library for Go with support for multiple algorithms and storage backends.

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**rlim** is a high-performance rate limiting library for Go that helps you protect your APIs and services from abuse. It supports multiple rate limiting algorithms and can be easily integrated as middleware in your Go web applications.

## Features

- **Multiple Algorithms**: Token bucket and leaky bucket implementations
- **Flexible Storage**: Redis-backed and in-memory storage options
- **Middleware Ready**: Use middleware for easy integration
- **Multi-Tier Support**: Configure different rate limits for different user tiers
- **YAML Configuration**: Simple, declarative configuration format

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Architecture](#architecture)
- [Contributing](#contributing)
- [License](#license)

## Installation

**Install the library:**

```bash
go get github/martinmaurice/rlim/pkg/rate_limiter
```

## Quick Start

Import the rate limiter in your Go application:

```go
import (
    "github.com/gin-gonic/gin"
    "github/martinmaurice/rlim/internal/server/middleware"
    "github/martinmaurice/rlim/pkg/rate_limiter"
)

func main() {
    // Initialize the rate limiter (loads config.yaml by default)
    rateLimiter := rate_limiter.New()

    router := gin.Default()

    // Apply rate limiting middleware
    router.Use(func(c *gin.Context) { 
        key := fmt.Sprintf(c.ClientIP())
        if ok := rateLimiter.CheckRateLimit(key, "default"); ok {
            c.Next()
            return
        }
        
        c.AbortWithStatus(http.StatusTooManyRequests)
	})

    router.GET("/api/data", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "success"})
    })

    router.Run(":8080")
}
``` 

## Configuration

Create a `config.yaml` file in your project root:

```yaml
rate_limits:
  default:
    algorithm: token_bucket
    capacity: 100
    refill_rate: 10  # tokens per second
    expiration: 3600

  items:
    # Free tier users
    free_tier:
      algorithm: token_bucket
      requests_per_minute: 60
      requests_per_hour: 1000
      capacity: 10
      expiration: 3600

    # Premium tier users
    premium_tier:
      algorithm: token_bucket
      requests_per_minute: 600
      requests_per_hour: 20000
      capacity: 100

    # Specific endpoint protection (e.g., login)
    login_endpoint:
      algorithm: leaky_bucket
      requests_per_minute: 10
      requests_per_hour: 100
      capacity: 20

metrics:
  enabled: true
  path: "/metrics"
```

### Configuration Options

Each rate limiter configuration accepts the following options:

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `algorithm` | string | Yes | Rate limiting algorithm: `token_bucket` or `leaky_bucket` |
| `requests_per_minute` | int | No* | Maximum requests allowed per minute |
| `requests_per_hour` | int | No* | Maximum requests allowed per hour |
| `capacity` | int | Yes | Token bucket capacity (burst size) |
| `expiration` | int | No | Time in seconds before the limiter state expires (default: 3600) |

At least one of `requests_per_minute` or `requests_per_hour` must be specified.

### Algorithm Details

**Token Bucket**
- Allows burst traffic up to the capacity limit
- Tokens refill at a steady rate
- Best for APIs that can handle occasional spikes

**Leaky Bucket**
- Smooths out traffic to a constant rate
- Prevents burst traffic
- Best for rate-sensitive operations (e.g., login attempts)

## Reference Implementation

The `cmd/server` directory contains a reference server implementation that demonstrates how to use the library. This is optional and provided as an example.

### Running the Example Server

```bash
# Clone the repository
git clone https://github.com/martinmaurice/rlim.git
cd rlim

# Run the example server
go run cmd/server/main.go

# Or build it
go build -o rlim-server cmd/server/main.go
./rlim-server
```

### Command Line Options

```bash
# Run with custom env file
./rlim-server -env=.env.production

# Disable rate limiting (for testing)
./rlim-server -disableRateLimiter
```

### Running with Docker

```bash
docker build -t rlim .
docker-compose up
```

## Architecture

```
rlim/
├── cmd/
│   └── server/          # Example server implementation (optional)
├── internal/
│   └── server/          # Middleware implementations
├── pkg/
│   ├── rate_limiter/    # Core library - import this in your app
│   ├── config/          # Configuration management
│   └── env/             # Environment handling
└── config.yaml          # Rate limit configuration
```

**What you need to import:**
- `github/martinmaurice/rlim/pkg/rate_limiter` - Core rate limiting functionality

### Storage Backends

- **Redis**: Distributed rate limiting across multiple instances
- **In-Memory**: Fast, single-instance rate limiting

## Contributing

Contributions are welcome! This project is designed to be a learning resource and a practical tool for the community.

### Development Setup

1. Clone the repository
```bash
git clone https://github.com/martinmaurice/rlim.git
cd rlim
```

2. Install dependencies
```bash
go mod download
```

3. Run tests
```bash
go test -race ./...
```

### Roadmap

- [x] Token bucket algorithm (Redis)
- [x] Token bucket algorithm (In-memory)
- [x] Leaky bucket algorithm (Redis)
- [x] Leaky bucket algorithm (In-memory)
- [ ] Gin middleware integration
- [ ] Prometheus' metrics integration
- [ ] Performance benchmarks
- [ ] Additional middleware support (Echo, Chi, etc.)
- [ ] Rate limit headers (X-RateLimit-*)
- [ ] Advanced configuration options

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with:
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [Redis](https://github.com/redis/go-redis) - Redis client
- [Viper](https://github.com/spf13/viper) - Configuration management

---

**Note**: This project is part of my portfolio and is open for anyone to use and contribute to. Feel free to open issues or submit pull requests!