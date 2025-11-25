# Gemini Code Assistant Context

This document provides a comprehensive overview of the GetMentor.dev API project, intended to be used as a context file for the Gemini Code Assistant.

## Project Overview

The project is the backend API for the GetMentor.dev platform, a service that connects mentors with mentees. It's written in Go and is a rewrite of a previous Next.js backend.

The API handles all backend operations, including:
-   Serving mentor data from an Airtable base.
-   Providing in-memory caching with auto-refresh to reduce latency and load on Airtable.
-   Managing user profiles and profile picture uploads to Azure Blob Storage.
-   Handling contact form submissions with reCAPTCHA verification.
-   Exposing a set of public and internal API endpoints for various clients.
-   Receiving webhooks from Airtable to invalidate caches.
-   Providing comprehensive observability with Prometheus metrics, structured logging, and distributed tracing.

## Architecture

The project follows a classic layered architecture:

-   **`cmd/api/main.go`**: The application entry point, responsible for initializing all components and setting up the HTTP server.
-   **`config`**: Handles application configuration, loading values from environment variables and `.env` files using Viper.
-   **`internal/handlers`**: Contains the HTTP handlers for each API route. They are responsible for parsing requests, calling the appropriate services, and writing JSON responses.
-   **`internal/services`**: Implements the business logic of the application. They orchestrate calls to repositories and other services.
-   **`internal/repository`**: The data access layer, responsible for interacting with the database (Airtable) and caches.
-   **`internal/middleware`**: Provides HTTP middleware for concerns like authentication, rate limiting, observability, and security headers.
-   **`internal/models`**: Defines the data structures used throughout the application.
-   **`pkg`**: Contains shared libraries and clients for external services like Airtable, Azure, and the logger.

### Key Technologies

-   **Language**: Go
-   **Web Framework**: Gin
-   **Configuration**: Viper
-   **Logging**: Zap
-   **Database**: Airtable
-   **Caching**: In-memory cache (`go-cache`)
-   **File Storage**: Azure Blob Storage
-   **Metrics**: Prometheus
-   **Tracing**: OpenTelemetry
-   **Testing**: Go's standard testing package and `stretchr/testify`
-   **Linting**: `golangci-lint`
-   **Containerization**: Docker

## Building and Running

### Prerequisites

-   Go (version 1.22 or higher)
-   Docker
-   Make

### Key Commands

The `Makefile` provides several useful commands for development:

-   `make build`: Build the application binary.
-   `make run`: Run the application locally.
-   `make test`: Run the unit tests.
-   `make test-coverage`: Run tests with a coverage report.
-   `make lint`: Run the linter.
-   `make docker-build`: Build the Docker image.
-   `make docker-run`: Run the application in a Docker container.

### Local Development

1.  Copy `.env.example` to `.env` and fill in the required credentials for Airtable, Azure, etc.
2.  Run `go mod tidy` to install dependencies.
3.  Run `make run` to start the application.

The API will be available at `http://localhost:8080`.

## Development Conventions

-   **Code Style**: The project follows standard Go conventions. `golangci-lint` is used to enforce a consistent style.
-   **Testing**: Unit tests are located in the same package as the code they are testing, with the `_test.go` suffix. The project aims for high test coverage.
-   **Commits**: Commit messages should be clear and concise.
-   **CI/CD**: The project uses GitHub Actions for continuous integration. The workflow in `.github/workflows/build-and-test.yml` runs tests, builds the Docker image, and runs the linter on every push and pull request to the `main` branch.
-   **Dependencies**: Dependencies are managed with Go modules.
-   **Observability**: The application is instrumented with Prometheus metrics and OpenTelemetry traces. Structured logging is used throughout the application.
-   **Error Handling**: Errors are handled explicitly and bubble up to the handlers, where they are converted into appropriate HTTP error responses.
-   **Security**: The application implements several security best practices, including:
    -   Using a non-root user in the Docker container.
    -   Implementing rate limiting on all endpoints.
    -   Using authentication tokens to protect endpoints.
    -   Validating webhook secrets.
    -   Using reCAPTCHA to prevent spam.
    -   Setting security headers.

## Key Files

-   `README.md`: The main entry point for understanding the project.
-   `go.mod`: Defines the project's dependencies.
-   `Makefile`: Contains common development commands.
-   `cmd/api/main.go`: The application entry point.
-   `config/config.go`: Defines the application's configuration structure.
-   `internal/handlers/mentor_handler.go`: An example of an HTTP handler.
-   `internal/services/mentor_service.go`: An example of a service.
-   `internal/repository/mentor_repository.go`: An example of a repository.
-   `.github/workflows/build-and-test.yml`: The CI/CD pipeline.
-   `Dockerfile`: Defines the Docker image for the application.
-   `.env.example`: Shows the required environment variables.
