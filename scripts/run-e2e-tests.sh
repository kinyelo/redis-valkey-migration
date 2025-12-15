#!/bin/bash

set -e

PROJECT_NAME="migration-e2e-test"
COMPOSE_FILE="docker-compose.test.yml"

echo "Starting e2e test containers..."

# Clean up any existing containers
docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans 2>/dev/null || true

# Start containers
docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d --remove-orphans

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up containers..."
    docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans 2>/dev/null || true
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Wait for Redis
echo "Waiting for Redis..."
for i in $(seq 1 30); do
    if docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T redis redis-cli ping >/dev/null 2>&1; then
        echo "Redis is ready"
        break
    fi
    echo "Waiting for Redis... (attempt $i/30)"
    sleep 2
    if [ $i -eq 30 ]; then
        echo "Redis failed to start"
        docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs redis
        exit 1
    fi
done

# Wait for Valkey
echo "Waiting for Valkey..."
for i in $(seq 1 30); do
    if docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T valkey valkey-cli ping >/dev/null 2>&1; then
        echo "Valkey is ready"
        break
    fi
    echo "Waiting for Valkey... (attempt $i/30)"
    sleep 2
    if [ $i -eq 30 ]; then
        echo "Valkey failed to start"
        docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs valkey
        exit 1
    fi
done

echo "All containers are ready, running tests..."

# Run the tests (excluding CLI tests which require different Docker setup)
REDIS_HOST=127.0.0.1 REDIS_PORT=16379 VALKEY_HOST=127.0.0.1 VALKEY_PORT=16380 \
    go test -v -timeout=300s -run "^Test(E2E|Verify)" ./test/integration/...

echo "E2E tests completed successfully"