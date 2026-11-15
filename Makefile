FILE=data/zomato.csv
MONGO_URI ?= mongodb://localhost:27017/?directConnection=true
DB ?= biteexpress
TENANT ?= biteexpress

.DEFAULT_GOAL := help

.PHONY: help build-seed seed test server up down

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n",$$1,$$2}'

build-seed: # build seed target
	go build -o ./build/seed ./cmd/seed

seed: ## Seed the data
	go run ./cmd/seed --file $(FILE) --mongo-uri "$(MONGO_URI)" --db $(DB) --tenant $(TENANT)

test: ## Run the Go tests
	go test ./...

server:
	go run ./cmd/server

up: ## Build and start Mongo + API
	docker-compose up --build -d --wait
	@echo "API: http://localhost:8082/api/v1/healthz"

down: ## stopping everything
	docker-compose down
