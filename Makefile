# golang-dojo — study helpers. Run `make` for the list.
.DEFAULT_GOAL := help

.PHONY: help test race vet fmt check deadlock exercises tidy

help: ## Show this help
	@grep -E '^[a-z]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

test: ## Run all reference tests
	go test ./reference/...

race: ## Run reference tests under the race detector
	go test -race ./reference/...

vet: ## go vet everything
	go vet ./...

fmt: ## Format all code
	gofmt -w .

check: fmt vet race ## Format, vet, and race-test (do this before you trust it)
	@echo "✅ all checks passed"

deadlock: ## Run the deadlock lab
	go run ./labs/deadlock

exercises: ## Run the type-it-cold exercises (RED until you implement them)
	go test ./exercises/...

tidy: ## Tidy go.mod
	go mod tidy
