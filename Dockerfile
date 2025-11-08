# Multi-stage Dockerfile for Go API production deployment
# Creates a minimal final image by separating build and runtime stages
# Note: Grafana Alloy runs in a separate container in Docker Compose

# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
# -ldflags="-w -s" strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/getmentor-api \
    ./cmd/api/main.go

# Stage 2: Production runtime image
# Using Debian for better compatibility with various dependencies
FROM debian:bookworm-slim AS runner
WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -m -s /bin/bash appuser

# Create necessary directories
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app

# Copy Go binary from builder
COPY --from=builder /app/bin/getmentor-api /app/getmentor-api
RUN chmod +x /app/getmentor-api

# Set proper ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose application port
# 8081: Application HTTP server (internal in Docker Compose, no public exposure)
EXPOSE 8081

# Set environment variables
ENV PORT=8081
ENV GIN_MODE=release
ENV LOG_DIR=/app/logs

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8081/api/healthcheck || exit 1

# Run the Go application directly
CMD ["/app/getmentor-api"]
