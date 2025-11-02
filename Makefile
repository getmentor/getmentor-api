.PHONY: help build run test lint docker-build docker-run clean

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build the Go application"
	@echo "  run            - Run the application locally"
	@echo "  test           - Run tests"
	@echo "  lint           - Run linters"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-test    - Build and test Docker image"
	@echo "  clean          - Clean build artifacts"

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
