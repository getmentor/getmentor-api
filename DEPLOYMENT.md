# Deployment Guide - GetMentor API (Go)

This guide covers deploying the GetMentor API to DigitalOcean App Platform.

## Prerequisites

- Docker installed locally (for testing)
- GitHub repository with the code
- DigitalOcean account
- Environment variables configured

## Table of Contents

1. [Local Development](#local-development)
2. [Docker Deployment](#docker-deployment)
3. [DigitalOcean App Platform](#digitalocean-app-platform)
4. [Environment Variables](#environment-variables)
5. [Monitoring & Observability](#monitoring--observability)
6. [Troubleshooting](#troubleshooting)

---

## Local Development

### Running Locally (without Docker)

```bash
# Copy environment variables
cp .env.example .env

# Edit .env with your credentials
vim .env

# Download dependencies
go mod download

# Run the application
go run cmd/api/main.go
```

The API will be available at `http://localhost:8080`

### Using Make

```bash
# Build the application
make build

# Run the application
make run

# Run tests
make test

# Run linters
make lint

# Build Docker image
make docker-build

# Run Docker container
make docker-run
```

---

## Docker Deployment

### Building the Docker Image

```bash
# Build the image
docker build -t getmentor-api:latest .

# Or use the test script
./docker-build-test.sh
```

### Running the Docker Container

```bash
# Run with .env file
docker run -p 8080:8080 --env-file .env getmentor-api:latest

# Run in detached mode
docker run -d -p 8080:8080 --env-file .env --name getmentor-api getmentor-api:latest

# Check logs
docker logs getmentor-api

# Stop container
docker stop getmentor-api
docker rm getmentor-api
```

### Testing the Container

```bash
# Health check
curl http://localhost:8080/api/healthcheck

# Metrics
curl http://localhost:8080/api/metrics

# Internal API (with auth)
curl -H "x-internal-mentors-api-auth-token: YOUR_TOKEN" \
  -X POST http://localhost:8080/api/internal/mentors
```

---

## DigitalOcean App Platform

### Step 1: Create New App

1. Go to DigitalOcean Dashboard → Apps
2. Click "Create App"
3. Select GitHub as source
4. Choose `getmentor/getmentor-api` repository
5. Select `main` branch
6. Click "Next"

### Step 2: Configure Resources

**Service Configuration:**
- **Name**: `getmentor-api`
- **Type**: Web Service
- **Environment**: Production
- **Instance Size**: Basic (512MB RAM, 1 vCPU) or higher
- **Instance Count**: 1 (can scale later)

**Build Configuration:**
- **Dockerfile Path**: `Dockerfile`
- **Build Command**: (leave default - uses Dockerfile)

**HTTP Configuration:**
- **HTTP Port**: 8080
- **HTTP Request Routes**: `/`
- **Health Check Path**: `/api/healthcheck`

### Step 3: Set Environment Variables

Add all required environment variables from `.env.example`:

**Required Variables:**

```bash
# Airtable
AIRTABLE_API_KEY=your_key
AIRTABLE_BASE_ID=your_base_id
AIRTABLE_WORK_OFFLINE=0

# Azure Storage
AZURE_STORAGE_CONNECTION_STRING=your_connection_string
AZURE_STORAGE_CONTAINER_NAME=mentor-images
AZURE_STORAGE_DOMAIN=your_domain.blob.core.windows.net

# Authentication
MENTORS_API_LIST_AUTH_TOKEN=your_token_1
MENTORS_API_LIST_AUTH_TOKEN_INNO=your_token_2
MENTORS_API_LIST_AUTH_TOKEN_AIKB=your_token_3
INTERNAL_MENTORS_API=your_internal_token
REVALIDATE_SECRET_TOKEN=your_secret
WEBHOOK_SECRET=your_webhook_secret

# ReCAPTCHA
RECAPTCHA_V2_SECRET_KEY=your_secret
NEXT_PUBLIC_RECAPTCHA_V2_SITE_KEY=your_site_key

# Next.js Integration
NEXTJS_BASE_URL=https://your-nextjs-app.com
NEXTJS_REVALIDATE_SECRET=your_secret

# Grafana Cloud
GCLOUD_HOSTED_METRICS_URL=your_metrics_url
GCLOUD_HOSTED_METRICS_ID=your_username
GCLOUD_HOSTED_LOGS_URL=your_logs_url
GCLOUD_HOSTED_LOGS_ID=your_username
GCLOUD_RW_API_KEY=your_api_key

# Logging
LOG_LEVEL=info
LOG_DIR=/app/logs
```

**System Variables:**
```bash
PORT=8080
GIN_MODE=release
APP_ENV=production
```

### Step 4: Configure Health Checks

- **Path**: `/api/healthcheck`
- **Initial Delay**: 10 seconds
- **Period**: 30 seconds
- **Timeout**: 3 seconds
- **Success Threshold**: 1
- **Failure Threshold**: 3

### Step 5: Deploy

1. Review configuration
2. Click "Create Resources"
3. Wait for initial deployment (~5-10 minutes)

### Step 6: Verify Deployment

Once deployed, verify the following endpoints:

```bash
# Replace with your app URL
APP_URL="https://your-app.ondigitalocean.app"

# Health check
curl $APP_URL/api/healthcheck

# Metrics
curl $APP_URL/api/metrics

# Internal API
curl -H "x-internal-mentors-api-auth-token: YOUR_TOKEN" \
  -X POST $APP_URL/api/internal/mentors
```

---

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AIRTABLE_API_KEY` | Airtable API key | `pat...` |
| `AIRTABLE_BASE_ID` | Airtable base ID | `app...` |
| `INTERNAL_MENTORS_API` | Internal API token | UUID |
| `AZURE_STORAGE_CONNECTION_STRING` | Azure connection string | `DefaultEndpoints...` |
| `GCLOUD_RW_API_KEY` | Grafana Cloud API key | `glc_...` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AIRTABLE_WORK_OFFLINE` | Use offline mode (testing) | `0` |
| `LOG_LEVEL` | Logging level | `info` |
| `PORT` | Server port | `8080` |
| `GIN_MODE` | Gin framework mode | `release` |

---

## Monitoring & Observability

### Grafana Cloud Integration

The application automatically sends metrics and logs to Grafana Cloud when properly configured.

**Metrics Endpoint:** `/api/metrics`
- HTTP request metrics (duration, count, active requests)
- Airtable API metrics
- Cache metrics (hits, misses, size)
- Azure Storage metrics
- Business metrics (profile views, contact forms)
- Infrastructure metrics (goroutines, memory, heap)

**Logs:** Structured JSON logs written to `/app/logs/`
- Application logs: `/app/logs/app.log`
- Error logs: `/app/logs/error.log`

**Grafana Alloy:**
- Scrapes `/api/metrics` every 60s
- Tails log files and sends to Loki
- Pushes to Grafana Cloud automatically

### DigitalOcean Monitoring

- **Insights Tab**: View CPU, memory, and bandwidth usage
- **Logs Tab**: View application stdout/stderr
- **Metrics Tab**: View HTTP request metrics

### Custom Dashboards

Import the provided Grafana dashboard (if available) or create custom dashboards using the metrics:

```promql
# Request rate
rate(http_server_request_total[5m])

# Request duration (p95)
histogram_quantile(0.95, rate(http_server_request_duration_seconds_bucket[5m]))

# Cache hit rate
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))
```

---

## Troubleshooting

### Application Won't Start

**Check logs:**
```bash
# DigitalOcean
# Go to App → Runtime Logs tab

# Docker
docker logs <container-name>
```

**Common issues:**
1. Missing required environment variables
2. Invalid Airtable credentials
3. Invalid Azure Storage connection string
4. Port already in use (local only)

### Grafana Alloy Not Working

**Symptoms:**
- No metrics in Grafana Cloud
- Alloy errors in logs

**Solutions:**
1. Verify `GCLOUD_*` environment variables are set
2. Check Grafana Cloud API key is valid
3. Verify URLs are correct (metrics and logs)
4. Check if Alloy is running: `ps aux | grep alloy` (in container)

### High Memory Usage

**Normal:**
- Initial: ~50-100MB
- With cache: ~100-150MB
- Under load: ~150-200MB

**If higher:**
1. Check for memory leaks
2. Review cache configuration
3. Monitor goroutine count: check `/api/metrics`
4. Restart the application

### Slow API Responses

**Check:**
1. Airtable API latency (in metrics)
2. Cache hit rate (should be >80%)
3. Network latency to Airtable
4. Database connection pool settings

**Solutions:**
1. Increase cache TTL if data changes infrequently
2. Add more instances (horizontal scaling)
3. Optimize database queries

### Docker Build Fails

**Common issues:**
1. Go dependencies not downloadable
2. Grafana Alloy image not available
3. Network issues during build

**Solutions:**
```bash
# Clear Docker cache
docker builder prune -a

# Build with no cache
docker build --no-cache -t getmentor-api:latest .

# Check Go module cache
go clean -modcache
```

### DigitalOcean Deployment Fails

**Check:**
1. Dockerfile is valid
2. All environment variables are set
3. Health check endpoint is working
4. Port 8080 is exposed

**Debug:**
1. View build logs in DigitalOcean dashboard
2. Check runtime logs for startup errors
3. Verify resource limits (memory, CPU)

---

## Scaling

### Horizontal Scaling

To handle more traffic, increase instance count in DigitalOcean:

1. Go to App → Settings → Resources
2. Adjust instance count (e.g., 2-5 instances)
3. DigitalOcean handles load balancing automatically

**Considerations:**
- Cache is in-memory (per instance)
- Airtable rate limits apply
- Monitor costs

### Vertical Scaling

Increase resources per instance:

1. Go to App → Settings → Resources
2. Select larger instance size
3. Redeploy

**Recommended sizes:**
- **Basic**: 512MB RAM, 1 vCPU (low traffic)
- **Professional**: 1GB RAM, 1 vCPU (medium traffic)
- **Professional XL**: 2GB RAM, 2 vCPU (high traffic)

---

## Continuous Deployment

### GitHub Actions

The repository includes a CI/CD pipeline that:
1. Runs tests on every push
2. Builds Docker image
3. Runs linters

**Trigger deployment:**
- Push to `main` branch
- DigitalOcean auto-deploys on push

**Manual deployment:**
1. Go to DigitalOcean App
2. Click "Create Deployment"
3. Select branch/commit
4. Deploy

---

## Rollback

If a deployment fails or has issues:

1. Go to DigitalOcean App → Deployments
2. Find previous successful deployment
3. Click "..." menu → "Rollback to this deployment"
4. Confirm rollback

**Best practices:**
- Always test locally before deploying
- Monitor logs after deployment
- Keep previous version running during deployment
- Set up alerts in Grafana for critical metrics

---

## Security Checklist

- [ ] All environment variables use secrets (not committed to git)
- [ ] Authentication tokens are strong UUIDs
- [ ] HTTPS is enabled (automatic on DigitalOcean)
- [ ] Health check endpoint requires no auth
- [ ] All API endpoints validate input
- [ ] ReCAPTCHA is enabled for contact forms
- [ ] Grafana Cloud credentials are secured
- [ ] Non-root user runs the application (in Docker)
- [ ] Docker image is scanned for vulnerabilities

---

## Support

For issues or questions:
1. Check this documentation
2. Review application logs
3. Check Grafana dashboards
4. Open GitHub issue: https://github.com/getmentor/getmentor-api/issues
