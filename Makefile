.PHONY: help build build-go package-binaries lint test migrate migrate-down seed \
       deploy-up deploy-down deploy-migrate clean

SHELL := /bin/bash
COMPOSE_PROD_FILE := infra/docker/docker-compose.yml
ENV_FILE ?= .env
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: build-go ## Build all Go services

build-go: ## Build backend binaries
	mkdir -p bin
	cd services/api && go build $(GO_LDFLAGS) -o ../../bin/api ./cmd/api
	cd services/realtime && go build $(GO_LDFLAGS) -o ../../bin/realtime ./cmd/realtime
	cd services/media && go build $(GO_LDFLAGS) -o ../../bin/media ./cmd/media
	cd services/worker && go build $(GO_LDFLAGS) -o ../../bin/worker ./cmd/worker

package-binaries: build-go ## Package backend binaries as a zip archive
	mkdir -p release
	cd bin && zip -q ../release/relayforge-backend-$(VERSION).zip api realtime media worker

lint: ## Run Go lint checks
	cd services/api && golangci-lint run ./...
	cd services/realtime && golangci-lint run ./...
	cd services/media && golangci-lint run ./...
	cd services/worker && golangci-lint run ./...

test: ## Run backend tests
	go test ./services/api/... ./services/realtime/... ./services/media/... ./services/worker/...

migrate: ## Run database migrations
	cd services/api && go run ./cmd/api migrate up

migrate-down: ## Roll back the latest migration
	cd services/api && go run ./cmd/api migrate down

seed: ## Seed the database
	cd services/api && go run ./cmd/api seed

deploy-up: ## Start the backend Docker stack
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_PROD_FILE) up -d

deploy-down: ## Stop the backend Docker stack
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_PROD_FILE) down

deploy-migrate: ## Run database migrations inside the backend Docker stack
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_PROD_FILE) exec api api migrate up

clean: ## Clean backend build artifacts
	rm -rf bin release
