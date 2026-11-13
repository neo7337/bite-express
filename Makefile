FILE=data/zomato.csv
MONGO_URI ?= mongodb://localhost:27017/?directConnection=true
DB ?= biteexpress
TENANT ?= biteexpress

.DEFAULT_GOAL := help

.PHONY: build-seed seed test

build-seed: # build seed target
	go build -o ./build/seed ./cmd/seed

seed: ## Seed the data
	go run ./cmd/seed --file $(FILE) --mongo-uri "$(MONGO_URI)" --db $(DB) --tenant $(TENANT)

test: ## Run the Go tests
	go test ./...