# GetMentor API (Go)

Backend API service for GetMentor.dev platform, written in Go. This service handles all backend operations including Airtable integration, caching, profile management, and contact form submissions.

## Overview

This is a complete rewrite of the Next.js backend API in Go, providing:
- High-performance API endpoints
- In-memory caching with auto-refresh
- Comprehensive observability (Prometheus metrics + structured logging)
- Airtable integration for mentor data
- Azure Blob Storage for profile images
- ReCAPTCHA verification for forms

## Architecture

```
├── cmd/api/              # Application entry point
├── config/               # Configuration management
├── internal/
│   ├── cache/           # In-memory caching layer
│   ├── handlers/        # HTTP handlers (API routes)
│   ├── middleware/      # HTTP middleware (auth, observability)
│   ├── models/          # Data models
│   ├── repository/      # Data access layer
│   └── services/        # Business logic layer
└── pkg/
    ├── airtable/        # Airtable client
    ├── azure/           # Azure Storage client
    ├── logger/          # Structured logging
    └── metrics/         # Prometheus metrics
```

## Prerequisites

- Go 1.22 or higher
- Docker (for containerized deployment)
- Airtable account with API key
- Azure Storage account
- Grafana Cloud account (for observability)

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/getmentor/getmentor-api.git
cd getmentor-api
```

### 2. Install dependencies

```bash
go mod download
go mod tidy
```

### 3. Configure environment variables

Copy `.env.example` to `.env` and fill in your credentials:

```bash
cp .env.example .env
```

Required variables:
- `AIRTABLE_API_KEY` - Your Airtable API key
- `AIRTABLE_BASE_ID` - Your Airtable base ID
- `AZURE_STORAGE_CONNECTION_STRING` - Azure Storage connection string
- `INTERNAL_MENTORS_API` - Internal API authentication token
- All auth tokens and Grafana Cloud credentials

### 4. Build the application

```bash
go build -o bin/getmentor-api cmd/api/main.go
```

### 5. Run locally

```bash
./bin/getmentor-api
```

The API will start on `http://localhost:8080`

## Development

### Running with auto-reload

Use `air` for hot reloading during development:

```bash
go install github.com/cosmtrek/air@latest
air
```

### Running tests

```bash
go test ./...
```

### Linting

```bash
golangci-lint run
```

## API Endpoints

### Public Endpoints

- `GET /api/mentors` - Get all visible mentors (requires `mentors_api_auth_token` header)
- `GET /api/mentor/:id` - Get single mentor by ID (requires auth token)
- `POST /api/contact-mentor` - Submit contact form (with ReCAPTCHA)

### Internal Endpoints

- `POST /api/internal/mentors` - Main cached mentor API (requires `x-internal-mentors-api-auth-token`)
  - Query params: `id`, `slug`, `rec`, `force_reset_cache`
  - Body params: `only_visible`, `show_hidden`, `drop_long_fields`

### Profile Management

- `POST /api/save-profile?id=X&token=Y` - Update mentor profile
- `POST /api/upload-profile-picture?id=X&token=Y` - Upload profile picture

### Webhooks

- `POST /api/webhooks/airtable` - Receive Airtable updates (requires `X-Webhook-Secret` header)
- `POST /api/revalidate-nextjs?slug=X&secret=Y` - Trigger Next.js ISR revalidation

### Utility

- `GET /api/healthcheck` - Health check endpoint
- `GET /api/metrics` - Prometheus metrics endpoint

## Deployment

### Docker

Build the Docker image:

```bash
docker build -t getmentor-api:latest .
```

Run the container:

```bash
docker run -p 8080:8080 --env-file .env getmentor-api:latest
```

### Digital Ocean App Platform

1. Push code to GitHub
2. Create new App in Digital Ocean App Platform
3. Select this repository
4. Configure environment variables
5. Deploy

See `DEPLOYMENT.md` for detailed deployment instructions.

## Observability

### Metrics

Prometheus metrics are exposed at `/api/metrics` and include:
- HTTP request duration and count
- Airtable API request metrics
- Cache hit/miss rates
- Azure Storage metrics
- Business metrics (profile views, contact submissions, etc.)

### Logging

Structured JSON logs are written to:
- `stdout` (always)
- `/app/logs/app.log` (production)
- `/app/logs/error.log` (errors only, production)

### Grafana Alloy

Grafana Alloy runs in the same container and:
- Scrapes `/api/metrics` every 60s → Grafana Cloud Prometheus
- Tails log files → Grafana Cloud Loki

## Configuration

All configuration is managed via environment variables. See `.env.example` for a complete list.

Key configurations:
- `PORT` - Server port (default: 8080)
- `GIN_MODE` - Gin mode (debug/release)
- `LOG_LEVEL` - Logging level (debug/info/warn/error)
- `AIRTABLE_WORK_OFFLINE` - Use test data instead of real Airtable

## Caching

### Mentor Cache
- TTL: 60 seconds
- Auto-refresh on expiry
- Force refresh via `?force_reset_cache=true`

### Tags Cache
- TTL: 24 hours
- Auto-populated on startup

## Security

- All public endpoints require authentication tokens
- Profile endpoints verify mentor-specific auth tokens
- Webhook endpoints require secret validation
- ReCAPTCHA verification for contact forms
- No sensitive data in logs
- Secure fields (auth tokens, calendar URLs) not serialized by default

## Performance

Expected performance characteristics:
- Memory usage: 50-100MB (vs 512MB for Next.js)
- Request latency: 20-100ms p95
- Throughput: 5000-10000 req/s
- Startup time: <1s

## Contributing

1. Create a feature branch
2. Make your changes
3. Run tests and linting
4. Submit a pull request

## License

See LICENSE file

## Support

For issues and questions, please open a GitHub issue.
