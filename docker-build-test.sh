#!/bin/bash
# Docker build test script for GetMentor API
# This script builds the Docker image and can optionally run it for testing

set -e

echo "🐳 Building GetMentor API Docker image..."

# Build the Docker image
docker build \
  -t getmentor-api:test \
  -f Dockerfile \
  .

echo "✅ Docker image built successfully!"
echo ""
echo "Image details:"
docker images | grep getmentor-api | head -1

echo ""
echo "To run the container with your .env file:"
echo "  docker run -p 8080:8080 --env-file .env getmentor-api:test"
echo ""
echo "To test with healthcheck:"
echo "  docker run -d -p 8080:8080 --env-file .env --name getmentor-api-test getmentor-api:test"
echo "  sleep 10"
echo "  curl http://localhost:8080/api/healthcheck"
echo "  docker logs getmentor-api-test"
echo "  docker stop getmentor-api-test && docker rm getmentor-api-test"
