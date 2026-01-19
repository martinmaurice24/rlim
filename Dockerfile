# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Copy config directory
COPY --from=builder /app/config ./config

# Expose port
EXPOSE 8080

# Run the application
CMD ["./server"]
