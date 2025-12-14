#!/bin/bash

# Simple integration test runner
# This script runs integration tests with proper Docker container management

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not available. Skipping integration tests."
    exit 0
fi

# Check if docker compose is available
if ! docker compose version &> /dev/null 2>&1; then
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose is not available. Skipping integration tests."
        exit 0
    fi
    DOCKER_COMPOSE_CMD="docker-compose"
else
    DOCKER_COMPOSE_CMD="docker compose"
fi

log_info "Running integration tests..."

# Set environment variables for the tests
export REDIS_HOST=localhost
export REDIS_PORT=16379
export VALKEY_HOST=localhost
export VALKEY_PORT=16380

# Run Docker-based integration tests only
log_info "Running Docker integration tests..."
go test -v -timeout=300s ./test/integration/... -run TestDockerTestSuite

if [ $? -eq 0 ]; then
    log_success "Integration tests passed!"
else
    log_error "Integration tests failed!"
    exit 1
fi