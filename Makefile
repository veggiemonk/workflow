# Makefile for workflow project

.PHONY: test lint build examples clean help

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run tests with coverage
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	golangci-lint run ./...

build: ## Build the project
	go build ./...

examples: ## Run all examples
	@echo "Running basic example..."
	@cd examples/basic && go run main.go
	@echo "\n\nRunning CI/CD example..."
	@cd examples/cicd && go run main.go
	@echo "\n\nRunning advanced example..."
	@cd examples/advanced && go run main.go
	@echo "\n\nRunning middleware example..."
	@cd examples/middleware && go run main.go

tidy: ## Run go mod tidy on all modules
	go mod tidy
	cd examples/basic && go mod tidy
	cd examples/cicd && go mod tidy
	cd examples/advanced && go mod tidy
	cd examples/middleware && go mod tidy

clean: ## Clean build artifacts
	go clean ./...
	rm -f coverage.out coverage.html
	rm -f examples/advanced/results.json

fmt: ## Format code
	go fmt ./...
	goimports -w .

check: lint test ## Run all checks (lint + test)

docs: ## Generate documentation
	@echo "Documentation available at:"
	@echo "  - README.md (main documentation)"
	@echo "  - docs/architecture.md (architecture overview)"
	@echo "  - docs/best-practices.md (best practices guide)"
	@echo "  - examples/ (working examples)"

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

ci: tidy fmt lint test ## Run CI pipeline locally

.DEFAULT_GOAL := help
