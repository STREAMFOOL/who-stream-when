.PHONY: help build run test test-verbose test-coverage clean fmt lint start stop restart clear-cache clear-db dev

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Build the server binary"
	@echo "  make run            - Run the server"
	@echo "  make start          - Build and run the server"
	@echo "  make dev            - Run with hot-reload using Air"
	@echo "  make test           - Run all tests"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter (requires golangci-lint)"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make clear-cache    - Clear application cache (if implemented)"
	@echo "  make clear-db       - Remove SQLite database"
	@echo "  make restart        - Stop and restart the server"

# Build the server
build:
	@echo "Building server..."
	go build -o server main.go

# Run the server (assumes already built)
run:
	@echo "Starting server..."
	./server

# Build and run
start: build run

# Run in development mode with hot-reload
dev:
	@echo "Starting development server with hot-reload..."
	air

# Run all tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	@echo ""
	@echo "Generating detailed coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: brew install golangci-lint"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f server
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Clear application cache
clear-cache:
	@echo "Clearing application cache..."
	@echo "Note: Cache clearing not yet implemented in application"

# Clear SQLite database
clear-db:
	@echo "Removing SQLite database..."
	rm -f data/who-live-when.db
	@echo "Database removed. It will be recreated on next server start."

# Stop the server (if running)
stop:
	@echo "Stopping server..."
	@pkill -f "./server" || echo "Server not running"

# Restart the server
restart: stop start
