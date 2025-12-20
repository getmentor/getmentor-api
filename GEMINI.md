# GEMINI.md

## Project Overview

This is the backend API for the GetMentor.dev platform, written in Go. It provides a high-performance, observable, and secure API for managing mentor data, profiles, and user interactions.

The project is a rewrite of a previous Next.js backend, aiming for better performance and a smaller resource footprint.

### Key Technologies

*   **Language:** Go
*   **Web Framework:** Gin
*   **Configuration:** Viper
*   **Data Storage:**
    *   Airtable (primary mentor data)
    *   Azure Blob Storage (profile images)
*   **Caching:** In-memory caching with `go-cache`
*   **Observability:**
    *   Prometheus for metrics
    *   OpenTelemetry for tracing
    *   Zap for structured logging
*   **Deployment:** Docker

### Architecture

The project follows a standard layered architecture:

*   `cmd/api/main.go`: The application entry point, responsible for wiring all components together.
*   `config`: Handles loading configuration from environment variables.
*   `internal/handlers`: Contains HTTP handlers for different API routes.
*   `internal/services`: Encapsulates the business logic.
*   `internal/repository`: The data access layer, interacting with Airtable and the cache.
*   `internal/models`: Defines the data structures used in the application.
*   `internal/middleware`: Implements HTTP middleware for concerns like authentication, rate limiting, and security headers.
*   `pkg`: Contains reusable packages for external services like Airtable, Azure, and internal utilities like logging and metrics.

## Building and Running

### Prerequisites

*   Go 1.22+
*   Docker
*   Airtable API Key
*   Azure Storage Connection String

### Setup and Running

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/getmentor/getmentor-api.git
    cd getmentor-api
    ```

2.  **Install dependencies:**
    ```bash
    go mod download
    ```

3.  **Configure environment variables:**
    Copy `.env.example` to `.env` and fill in the required credentials.
    ```bash
    cp .env.example .env
    ```

4.  **Build the application:**
    ```bash
    go build -o bin/getmentor-api cmd/api/main.go
    ```

5.  **Run the application:**
    ```bash
    ./bin/getmentor-api
    ```
    The API will be available at `http://localhost:8080`.

### Running with Docker

```bash
# Build the image
docker build -t getmentor-api:latest .

# Run the container
docker run -p 8080:8080 --env-file .env getmentor-api:latest
```

### Running Tests

```bash
go test ./...
```

## Development Conventions

*   **Linting:** The project uses `golangci-lint` for code quality. Run `golangci-lint run` to check for issues.
*   **Hot Reload:** Use `air` for live-reloading during development.
*   **Testing:** Tests are located alongside the code in `_test.go` files. The project uses the `testify` library for assertions.
*   **Contribution:** The `README.md` suggests creating a feature branch and submitting a pull request for contributions.
