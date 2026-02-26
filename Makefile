.PHONY: build install test clean lint run-example verify-fix

# Binary name
BINARY := funcorder-fix
BUILD_DIR := bin

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build flags
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# Main targets
all: build

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/funcorder-fix

install: ## Install to GOPATH/bin
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY) ./cmd/funcorder-fix

test: ## Run tests
	$(GOTEST) -v -race ./...

clean: ## Clean build artifacts
	@rm -rf $(BUILD_DIR)
	@rm -f $(GOPATH)/bin/$(BINARY)

lint: ## Run linters
	golangci-lint run ./...

# Development targets
run-example: build ## Run on example file (check only)
	$(BUILD_DIR)/$(BINARY) -v ./examples/input/

run-example-fix: build ## Run on example file (with fix, output to stdout)
	$(BUILD_DIR)/$(BINARY) --fix -v ./examples/input/

run-example-write: build ## Run on example file (with fix, write back)
	$(BUILD_DIR)/$(BINARY) --fix -w -v ./examples/input/

verify-fix: ## Verify that golangci-lint shows no funcorder warnings
	golangci-lint run -E funcorder ./examples/golden/...

# Dependencies
deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Watch mode (requires entr or similar tool)
watch: ## Watch for changes and rebuild
	@find . -name "*.go" | entr -r make build
