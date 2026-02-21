[![Test](https://github.com/getmentor/getmentor-api/actions/workflows/test.yml/badge.svg)](https://github.com/getmentor/getmentor-api/actions/workflows/test.yml)
[![PR Checks](https://github.com/getmentor/getmentor-api/actions/workflows/pr-checks.yml/badge.svg)](https://github.com/getmentor/getmentor-api/actions/workflows/pr-checks.yml)

# GetMentor API (Go)

Backend API service for GetMentor.dev platform, written in Go. This service handles all backend operations including PostgreSQL database, caching, profile management, and contact form submissions.

## Overview

This is a complete rewrite of the Next.js backend API in Go, providing:
- High-performance API endpoints
- In-memory caching with auto-refresh
- Comprehensive observability (Prometheus metrics + structured logging)
- PostgreSQL database for mentor data
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
    ├── db/              # PostgreSQL database client
    ├── azure/           # Azure Storage client
    ├── logger/          # Structured logging
    └── metrics/         # Prometheus metrics
```

## Prerequisites

- Go 1.22 or higher
- Docker (for containerized deployment)
- PostgreSQL database (14 or higher)
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
- `DATABASE_URL` - PostgreSQL connection string (e.g., `postgresql://user:pass@host:5432/dbname`)
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
- `POST /api/register-mentor` - Register a new mentor

### Authentication (Mentor Portal)

- `POST /api/v1/auth/mentor/request-login` - Send magic login link to mentor email
- `POST /api/v1/auth/mentor/verify` - Verify login token and create session (sets HttpOnly cookie)
- `GET /api/v1/auth/mentor/session` - Check current session validity
- `POST /api/v1/auth/mentor/logout` - Clear session cookie

Login tokens are single-use, expire in `LOGIN_TOKEN_TTL_MINUTES` (default: 15 min), and are sent via email + Telegram. Rate limited to 2 req/5 min per IP.

### Mentor Portal (session-authenticated)

- `GET /api/v1/mentor/profile` - Get own profile (with hidden fields)
- `POST /api/v1/mentor/profile` - Update own profile
- `POST /api/v1/mentor/profile/picture` - Upload profile picture
- `GET /api/v1/mentor/requests?group=active|past` - List requests
- `GET /api/v1/mentor/requests/:id` - Get single request
- `POST /api/v1/mentor/requests/:id/status` - Update request status
- `POST /api/v1/mentor/requests/:id/decline` - Decline request with reason

### Reviews

- `GET /api/v1/reviews/:requestId/check` - Check review eligibility
- `POST /api/v1/reviews/:requestId` - Submit mentee review

### Internal Endpoints

- `POST /api/internal/mentors` - Main cached mentor API (requires `x-internal-mentors-api-auth-token`)
  - Query params: `id`, `slug`, `rec`, `force_reset_cache`
  - Body params: `only_visible`, `show_hidden`, `drop_long_fields`

### Profile Management (legacy token-based)

- `POST /api/save-profile?id=X&token=Y` - Update mentor profile
- `POST /api/upload-profile-picture?id=X&token=Y` - Upload profile picture

### Webhooks

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
- Database query metrics
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
- `DATABASE_URL` - PostgreSQL connection string

## Caching

### Mentor Cache
- TTL: 60 seconds
- Auto-refresh on expiry
- Force refresh via `?force_reset_cache=true`

### Tags Cache
- TTL: 24 hours
- Auto-populated on startup

## Error Logging

HTTP errors are logged with rich context by the observability middleware:
- **4xx responses**: logged at `info` level (expected client errors: 401 unauthorized, 403 forbidden, 404 not found); `warn` for 429 rate limit and other unexpected 4xx
- **5xx responses**: logged at `error` level
- All error responses include: `error` reason (from handler via `c.Error()`), `route_params` (e.g. `{id: "abc"}`), sanitized `query_params` (sensitive params like `token`, `secret`, `key` are redacted)

Handlers use `respondError(c, status, message, err)` / `respondErrorWithDetails(...)` helpers which attach the internal error to gin context for the middleware to pick up.

## Security

- All public endpoints require authentication tokens
- Profile endpoints verify mentor-specific auth tokens
- Session-authenticated endpoints use JWT stored in HttpOnly cookie (`mentor_session`)
- Webhook endpoints require secret validation
- ReCAPTCHA verification for contact forms
- Sensitive query params (`token`, `secret`, `key`, `password`, `auth`) are redacted from logs
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
