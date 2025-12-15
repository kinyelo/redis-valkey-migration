# Redis to Valkey Migration Tool Makefile

# Build variables
BINARY_NAME=redis-valkey-migration
VERSION?=1.0.0
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION?=$(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS=-ldflags "-X github.com/kinyelo/redis-valkey-migration/internal/version.Version=$(VERSION) \
                  -X github.com/kinyelo/redis-valkey-migration/internal/version.GitCommit=$(GIT_COMMIT) \
                  -X github.com/kinyelo/redis-valkey-migration/internal/version.BuildDate=$(BUILD_DATE)"

# Directories
BUILD_DIR=build
DIST_DIR=dist

# Default target
.PHONY: all
all: clean test build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@go clean

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping lint"; \
	fi

# Run tests (including e2e tests if Docker is available)
.PHONY: test
test:
	@echo "Running unit and property tests..."
	@go test -v -short ./...
	@echo "Checking for Docker availability..."
	@if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then \
		echo "Docker available, running e2e tests..."; \
		./scripts/run-e2e-tests.sh || echo "E2E tests failed, but continuing..."; \
	else \
		echo "Docker not available, skipping e2e tests"; \
	fi

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -short -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run property-based tests
.PHONY: test-property
test-property:
	@echo "Running property-based tests..."
	@go test -v -short -run "Property" ./...

# Run e2e tests with container management
.PHONY: test-e2e
test-e2e:
	@echo "Running e2e tests with container management..."
	@./scripts/run-e2e-tests.sh

# Start e2e test containers
.PHONY: e2e-start-containers
e2e-start-containers:
	@echo "Starting e2e test containers..."
	@docker compose -f docker-compose.test.yml -p migration-e2e-test up -d --remove-orphans

# Wait for containers to be ready
.PHONY: e2e-wait-ready
e2e-wait-ready:
	@echo "Waiting for containers to be ready..."
	@timeout=60; \
	 while [ $$timeout -gt 0 ]; do \
		if docker compose -f docker-compose.test.yml -p migration-e2e-test exec -T redis redis-cli ping >/dev/null 2>&1 && \
		   docker compose -f docker-compose.test.yml -p migration-e2e-test exec -T valkey valkey-cli ping >/dev/null 2>&1; then \
			echo "Containers are ready"; \
			break; \
		fi; \
		echo "Waiting for containers... ($$timeout seconds remaining)"; \
		sleep 2; \
		timeout=$$((timeout-2)); \
	 done; \
	 if [ $$timeout -le 0 ]; then \
		echo "Containers failed to start within 60 seconds"; \
		exit 1; \
	 fi

# Run e2e tests
.PHONY: e2e-run-tests
e2e-run-tests:
	@echo "Running e2e tests..."
	@REDIS_HOST=127.0.0.1 REDIS_PORT=16379 VALKEY_HOST=127.0.0.1 VALKEY_PORT=16380 \
	 go test -v -timeout=300s ./test/integration/...

# Stop e2e test containers
.PHONY: e2e-stop-containers
e2e-stop-containers:
	@echo "Stopping e2e test containers..."
	@docker compose -f docker-compose.test.yml -p migration-e2e-test down -v --remove-orphans 2>/dev/null || true

# Run integration tests (requires Docker) - legacy target
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@./scripts/test-integration.sh

# Run all tests including integration tests
.PHONY: test-all
test-all:
	@echo "Running all tests including integration tests..."
	@go test -v ./...

# Build for current platform
.PHONY: build
build: fmt
	@echo "Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for all platforms
.PHONY: build-all
build-all: fmt
	@echo "Building $(BINARY_NAME) for all platforms..."
	@mkdir -p $(DIST_DIR)
	
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	
	# macOS ARM64 (Apple Silicon)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	
	@echo "Built binaries:"
	@ls -la $(DIST_DIR)/

# Create release packages
.PHONY: package
package: build-all
	@echo "Creating release packages..."
	@mkdir -p $(DIST_DIR)/packages
	
	# Create tar.gz for Unix systems
	@cd $(DIST_DIR) && tar -czf packages/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(DIST_DIR) && tar -czf packages/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd $(DIST_DIR) && tar -czf packages/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(DIST_DIR) && tar -czf packages/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	
	# Create zip for Windows
	@cd $(DIST_DIR) && zip packages/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	
	@echo "Release packages created:"
	@ls -la $(DIST_DIR)/packages/

# Install binary to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Run with sample migration
.PHONY: run-sample
run-sample: build
	@echo "Running sample migration (dry-run)..."
	@$(BUILD_DIR)/$(BINARY_NAME) migrate --dry-run --log-level debug

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	@go mod download
	@go mod tidy
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME):$(VERSION) .
	@docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

# Docker run
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@docker run --rm -it $(BINARY_NAME):latest

# Show help
.PHONY: help
help:
	@echo "Redis to Valkey Migration Tool - Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  all           - Clean, test, and build"
	@echo "  clean         - Remove build artifacts"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run linter (requires golangci-lint)"
	@echo "  test          - Run unit, property, and e2e tests (auto-detects Docker)"
	@echo "  test-e2e      - Run e2e tests with container management"
	@echo "  test-all      - Run all tests including integration tests"
	@echo "  test-integration - Run integration tests with Docker services"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-property - Run property-based tests only"
	@echo "  build         - Build for current platform"
	@echo "  build-all     - Build for all supported platforms"
	@echo "  package       - Create release packages"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  run           - Build and run the application"
	@echo "  run-sample    - Run sample migration (dry-run)"
	@echo "  dev-setup     - Set up development environment"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION       - Version to build (default: $(VERSION))"
	@echo "  GIT_COMMIT    - Git commit hash (default: auto-detected)"
	@echo "  BUILD_DATE    - Build timestamp (default: current time)"

# Default help target
.DEFAULT_GOAL := help