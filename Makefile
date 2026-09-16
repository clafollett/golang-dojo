# golang-dojo — study helpers. Run `make` for the list.
.DEFAULT_GOAL := help

.PHONY: help test race vet fmt check deadlock exercises practice tidy

# Which exercise to copy into the sandbox (blank = all of them).
EX ?=

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

practice: ## Copy exercise(s) into the gitignored _practice/ sandbox (EX=name for one, blank=all)
	@mkdir -p _practice
	@test -f _practice/go.mod || printf 'module practice\n\ngo 1.24\n' > _practice/go.mod
	@if [ -n "$(EX)" ]; then \
		test -d "exercises/$(EX)" || { echo "no such exercise: exercises/$(EX)"; echo "available: $$(ls exercises)"; exit 1; }; \
		rm -rf "_practice/$(EX)" && cp -r "exercises/$(EX)" "_practice/$(EX)"; \
		echo "✅ fresh blank at _practice/$(EX)/ — run: (cd _practice && go test ./$(EX)/)"; \
	else \
		for d in exercises/*/; do name=$$(basename "$$d"); rm -rf "_practice/$$name" && cp -r "$$d" "_practice/$$name"; done; \
		echo "✅ copied all exercises into _practice/ — run: (cd _practice && go test ./...)"; \
	fi

tidy: ## Tidy go.mod
	go mod tidy
