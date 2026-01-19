.PHONY: help up down build restart logs logs-app logs-redis clean test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start all services in detached mode
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build or rebuild services
	docker compose build

restart: ## Restart all services
	docker compose restart

logs: ## View logs from all services
	docker compose logs -f

logs-app: ## View logs from rate-limiter service
	docker compose logs -f rate-limiter

logs-redis: ## View logs from redis service
	docker compose logs -f redis

clean: ## Stop and remove all containers, networks, and volumes
	docker compose down -v

ps: ## List running containers
	docker compose ps

test: ## Run tests
	go test -v ./...

run: ## Run the application locally (without Docker)
	go run cmd/server/main.go
