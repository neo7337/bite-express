FILE=data/zomato.csv
MONGO_URI ?= mongodb://localhost:27017/?directConnection=true
DB ?= biteexpress
TENANT ?= biteexpress

.DEFAULT_GOAL := help

.PHONY: help build-seed seed test test-integration test-e2e lint server up down

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n",$$1,$$2}'

build-seed: # build seed target
	go build -o ./build/seed ./cmd/seed

seed: ## Seed the data
	go run ./cmd/seed --file $(FILE) --mongo-uri "$(MONGO_URI)" --db $(DB) --tenant $(TENANT)

test: ## Run the Go tests
	go test ./...

test-integration: ## Run integration tests against a real MongoDB (needs Docker)
	docker compose up -d --wait mongodb
	MONGO_URI="$(MONGO_URI)" go test -tags=integration -count=1 ./internal/repository/mongodb/...

test-e2e: ## Start the stack, seed the data, run API tests over real HTTP (needs Docker + Go)
	docker compose up --build -d --wait mongodb api
	$(MAKE) seed
	go test -tags=e2e -count=1 ./test/e2e/...

lint: ## Run golangci-lint
	golangci-lint run ./...

server:
	go run ./cmd/server

up: ## Build and start Mongo + API
	docker compose up --build -d --wait
	@echo "API: http://localhost:8082/api/healthz"

down: ## stopping everything
	docker compose down
