# Implementation TODO

This document tracks the remaining files that need to be created to complete the GetMentor API implementation.

## Status: Core Infrastructure Complete ✅

The following components are already implemented:
- ✅ Project structure
- ✅ Configuration management (`config/config.go`)
- ✅ Logging infrastructure (`pkg/logger/logger.go`)
- ✅ Metrics infrastructure (`pkg/metrics/metrics.go`)
- ✅ Middleware (observability, auth)
- ✅ Data models (Mentor, Contact, Profile, Webhook)
- ✅ Airtable client (`pkg/airtable/client.go`)
- ✅ Azure Storage client (`pkg/azure/storage.go`)
- ✅ Caching layer (Mentor cache, Tags cache)
- ✅ Repository layer (Mentor, Client Request)
- ✅ Main application (`cmd/api/main.go`)

## Files Still Needed

### 1. Service Layer (Priority: HIGH)

Create these files in `internal/services/`:

#### `mentor_service.go`
```go
package services

import (
	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
)

type MentorService struct {
	repo   *repository.MentorRepository
	config *config.Config
}

func NewMentorService(repo *repository.MentorRepository, cfg *config.Config) *MentorService {
	return &MentorService{repo: repo, config: cfg}
}

func (s *MentorService) GetAllMentors(opts models.FilterOptions) ([]*models.Mentor, error) {
	return s.repo.GetAll(opts)
}

func (s *MentorService) GetMentorByID(id int, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByID(id, opts)
}

func (s *MentorService) GetMentorBySlug(slug string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetBySlug(slug, opts)
}

func (s *MentorService) GetMentorByRecordID(recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByRecordID(recordID, opts)
}
```

#### `contact_service.go`
- Implement ReCAPTCHA verification
- Create client requests in Airtable
- Return mentor calendar URL
- See Next.js `/api/contact-mentor.js` for reference

#### `profile_service.go`
- Verify mentor auth token
- Preserve sponsor tags when updating
- Convert tag names to tag IDs
- Update Airtable mentor record
- See Next.js `/api/save-profile.js` for reference

#### `webhook_service.go`
- Handle Airtable webhooks
- Invalidate mentor cache
- Trigger Next.js ISR revalidation
- See Next.js implementation for reference

### 2. Handler Layer (Priority: HIGH)

Create these files in `internal/handlers/`:

#### `mentor_handler.go`
- `GetPublicMentors()` - Public mentor list endpoint
- `GetPublicMentorByID()` - Single public mentor
- `GetInternalMentors()` - Internal cached mentor API

#### `contact_handler.go`
- `ContactMentor()` - Handle contact form submissions

#### `profile_handler.go`
- `SaveProfile()` - Handle profile updates
- `UploadProfilePicture()` - Handle image uploads

#### `webhook_handler.go`
- `HandleAirtableWebhook()` - Process Airtable webhooks
- `RevalidateNextJS()` - Trigger Next.js revalidation

#### `health_handler.go`
- `Healthcheck()` - Simple health check endpoint

### 3. Docker & Deployment (Priority: HIGH)

#### `Dockerfile`
Multi-stage build with Grafana Alloy. See Next.js Dockerfile for reference:
```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Stage 2: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Install Grafana Alloy
RUN wget https://github.com/grafana/alloy/releases/download/v1.5.1/alloy-linux-amd64.zip && \
    unzip alloy-linux-amd64.zip && \
    chmod +x alloy-linux-amd64 && \
    mv alloy-linux-amd64 /usr/local/bin/alloy && \
    rm alloy-linux-amd64.zip

COPY --from=builder /app/main .
COPY config.alloy .
COPY start-with-alloy.sh .
RUN chmod +x start-with-alloy.sh

# Create logs directory
RUN mkdir -p /app/logs

EXPOSE 8080

CMD ["./start-with-alloy.sh"]
```

#### `config.alloy`
Grafana Alloy configuration. See Next.js `config.alloy` for reference.

#### `start-with-alloy.sh`
Startup script to launch both Alloy and the Go app:
```bash
#!/bin/sh
alloy run --server.http.listen-addr=0.0.0.0:12345 config.alloy &
./main
```

#### `.dockerignore`
```
.git
.env
.env.*
*.md
bin/
logs/
tmp/
.vscode/
.idea/
```

### 4. CI/CD (Priority: MEDIUM)

#### `.github/workflows/deploy.yml`
GitHub Actions workflow for building and deploying:
- Build Docker image
- Push to DigitalOcean registry
- Trigger deployment

### 5. Documentation (Priority: MEDIUM)

#### `DEPLOYMENT.md`
Detailed deployment guide for DigitalOcean App Platform

#### `API_DOCS.md`
Complete API documentation with request/response examples

### 6. Development Tools (Priority: LOW)

#### `docker-compose.yml`
Local development setup:
```yaml
version: '3.8'

services:
  api:
    build: .
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - ./logs:/app/logs
```

#### `Makefile`
Common development commands:
```makefile
.PHONY: build run test lint docker-build docker-run

build:
	go build -o bin/getmentor-api cmd/api/main.go

run:
	go run cmd/api/main.go

test:
	go test -v ./...

lint:
	golangci-lint run

docker-build:
	docker build -t getmentor-api:latest .

docker-run:
	docker run -p 8080:8080 --env-file .env getmentor-api:latest
```

## Quick Start Guide for Completing Implementation

### Step 1: Create Service Files
Copy the service implementations from the Next.js codebase and adapt them to Go:
- `/src/pages/api/contact-mentor.js` → `internal/services/contact_service.go`
- `/src/pages/api/save-profile.js` → `internal/services/profile_service.go`

### Step 2: Create Handler Files
Handlers are thin wrappers around services that handle HTTP request/response:
- Parse request (query params, body)
- Call service method
- Return JSON response

### Step 3: Test Locally
```bash
# Install dependencies
go mod download
go mod tidy

# Build
go build -o bin/getmentor-api cmd/api/main.go

# Run
./bin/getmentor-api
```

### Step 4: Create Docker Files
- Copy Grafana Alloy setup from Next.js project
- Adapt Dockerfile for Go binary
- Test locally with docker-compose

### Step 5: Deploy to DigitalOcean
- Push to GitHub
- Configure App Platform
- Set environment variables
- Deploy

## Reference Implementation

For detailed implementation examples, refer to:
- **Next.js API routes**: `/getmentor.dev/src/pages/api/`
- **Server utilities**: `/getmentor.dev/src/server/`
- **Observability setup**: `/getmentor.dev/src/lib/`

## Testing Checklist

Before deployment, test:
- [ ] GET /api/mentors returns mentor list
- [ ] POST /api/internal/mentors with cache
- [ ] POST /api/contact-mentor with ReCAPTCHA
- [ ] POST /api/save-profile updates mentor
- [ ] POST /api/upload-profile-picture uploads to Azure
- [ ] Webhook triggers cache invalidation
- [ ] Metrics endpoint returns Prometheus data
- [ ] Logs are written to files
- [ ] Grafana Alloy pushes to Grafana Cloud

## Notes

- The main application structure is complete
- Core infrastructure (logging, metrics, caching) is implemented
- Focus on completing service and handler implementation
- Reference the Next.js codebase for business logic
- All external service clients (Airtable, Azure) are ready to use
