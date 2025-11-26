.PHONY: help build run test test-coverage test-race lint docker-build docker-run clean fmt fmt-check vet security staticcheck ci pre-commit install-tools

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build the Go application"
	@echo "  run            - Run the application locally"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-race      - Run tests with race detection"
	@echo "  lint           - Run linters"
	@echo "  vet            - Run go vet"
	@echo "  fmt            - Format code"
	@echo "  fmt-check      - Check code formatting"
	@echo "  security       - Run security scanner (gosec)"
	@echo "  staticcheck    - Run staticcheck"
	@echo "  ci             - Run all CI checks"
	@echo "  pre-commit     - Run pre-commit checks"
	@echo "  install-tools  - Install development tools"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-test    - Build and test Docker image"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  tidy           - Tidy dependencies"

# Build the application
build:
	@echo "Building GetMentor API..."
	@go build -o bin/getmentor-api cmd/api/main.go

# Run the application
run:
	@echo "Running GetMentor API..."
	@go run cmd/api/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out
	@echo "\nTo view HTML coverage report, run: go tool cover -html=coverage.out"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@go test -v -race ./...

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t getmentor-api:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 --env-file .env getmentor-api:latest

# Build and test Docker image
docker-test:
	@echo "Building and testing Docker image..."
	@./docker-build-test.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf logs/
	@go clean

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -w .

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "❌ Code is not formatted. Run 'make fmt'"; \
		gofmt -d .; \
		exit 1; \
	fi
	@echo "✅ All files are properly formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Run security scanner
security:
	@echo "Running security scanner..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi

# Run staticcheck
staticcheck:
	@echo "Running staticcheck..."
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed. Install with: go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi

# Run all CI checks
ci: fmt-check vet lint test-race security
	@echo ""
	@echo "✅ All CI checks passed!"

# Pre-commit hook
pre-commit: fmt vet test
	@echo "✅ Pre-commit checks passed!"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "✅ All tools installed!"
