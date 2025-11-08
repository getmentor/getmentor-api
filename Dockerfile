# Multi-stage Dockerfile for Go API production deployment
# Creates a minimal final image by separating build and runtime stages

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

# Stage 2: Get Grafana Alloy binary from official image
FROM grafana/alloy:latest AS alloy

# Stage 3: Production runtime image
# Using Debian instead of Alpine for glibc compatibility with Grafana Alloy
FROM debian:bookworm-slim AS runner
WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Copy Grafana Alloy binary from official image
COPY --from=alloy /bin/alloy /usr/local/bin/alloy
RUN chmod +x /usr/local/bin/alloy

# Create non-root user for security
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -m -s /bin/bash appuser

# Create necessary directories
RUN mkdir -p /app/logs /var/lib/alloy/data && \
    chown -R appuser:appgroup /app /var/lib/alloy/data

# Copy Go binary from builder
COPY --from=builder /app/bin/getmentor-api /app/getmentor-api
RUN chmod +x /app/getmentor-api

# Copy Grafana Alloy configuration and startup script
COPY config.alloy /app/config.alloy
COPY start-with-alloy.sh /app/start-with-alloy.sh
RUN chmod +x /app/start-with-alloy.sh

# Set proper ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose application port and Alloy metrics port
# 8081: Application HTTP server (internal port, not exposed to internet)
# 12345: Grafana Alloy self-monitoring metrics
EXPOSE 8081 12345

# Set environment variables
ENV PORT=8081
ENV GIN_MODE=release
ENV LOG_DIR=/app/logs

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8081/api/healthcheck || exit 1

# Use the startup script that launches both Grafana Alloy and the Go app
CMD ["/app/start-with-alloy.sh"]
