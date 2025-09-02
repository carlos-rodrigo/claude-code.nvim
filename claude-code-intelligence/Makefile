# Claude Code Intelligence Service Makefile

.PHONY: build run test clean install dev help

# Variables
BINARY_NAME=claude-code-intelligence
MAIN_PATH=./cmd/server
BUILD_DIR=./bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Default target
help: ## Show this help message
	@echo "Claude Code Intelligence Service"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build the application
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# macOS (Intel)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# macOS (Apple Silicon)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	# Linux (x86_64)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# Linux (ARM64)
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	
	# Windows (x86_64)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	@echo "Binaries built in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

# Install dependencies
deps: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Run the application in development mode
dev: ## Run in development mode
	@echo "Starting development server..."
	@go run $(MAIN_PATH)

# Run the built binary
run: build ## Build and run the application
	@echo "Starting $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Lint the code
lint: ## Lint the code
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

# Format the code
fmt: ## Format the code
	@echo "Formatting code..."
	@go fmt ./...

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@go clean

# Install the binary to GOPATH/bin
install: build ## Install the binary
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Setup development environment
setup: ## Setup development environment
	@echo "Setting up development environment..."
	@go mod download
	@go mod tidy
	@mkdir -p data/backups logs
	@cp .env.example .env
	@echo "Development environment ready!"
	@echo "1. Edit .env file with your configuration"
	@echo "2. Make sure Ollama is running: ollama serve"
	@echo "3. Run: make dev"

# Database operations
db-backup: ## Create database backup
	@echo "Creating database backup..."
	@mkdir -p data/backups
	@timestamp=$$(date +%Y%m%d_%H%M%S); \
	if [ -f data/intelligence.db ]; then \
		cp data/intelligence.db data/backups/backup_$$timestamp.db; \
		echo "Backup created: data/backups/backup_$$timestamp.db"; \
	else \
		echo "No database found to backup"; \
	fi

# Check if Ollama is running
check-ollama: ## Check if Ollama is running
	@echo "Checking Ollama connection..."
	@curl -s http://localhost:11434/api/tags >/dev/null 2>&1 && \
		echo "✓ Ollama is running" || \
		echo "✗ Ollama is not running. Start with: ollama serve"

# Install recommended models
install-models: check-ollama ## Install recommended Ollama models
	@echo "Installing recommended models..."
	@echo "This may take a while depending on your internet connection..."
	@ollama pull gemma2:2b
	@ollama pull llama3.2:3b
	@echo "Models installed successfully!"

# Quick start
quick-start: setup install-models ## Quick start for new users
	@echo ""
	@echo "🚀 Quick start complete!"
	@echo ""
	@echo "Next steps:"
	@echo "1. Start the service: make dev"
	@echo "2. Test the API: curl http://localhost:7345/health"
	@echo "3. Check the documentation in docs/"
	@echo ""

# Development helpers
watch: ## Watch for changes and restart (requires 'air')
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Falling back to basic restart on change..."; \
		make dev; \
	fi

# Generate documentation
docs: ## Generate API documentation
	@echo "Generating documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/server/main.go; \
	else \
		echo "Swag not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# Security scan
security: ## Run security scan
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "Gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi