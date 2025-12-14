#!/bin/bash

# Integration Test Script for Redis to Valkey Migration Tool
# This script sets up the test environment and runs integration tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_PROJECT="migration-integration-test"
TEST_TIMEOUT="300s"
REDIS_PORT="16379"
VALKEY_PORT="16380"
docker_compose_cmd=""

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test environment..."
    $docker_compose_cmd -p "$COMPOSE_PROJECT" down -v --remove-orphans 2>/dev/null || true
    docker network prune -f 2>/dev/null || true
}

# Trap cleanup on exit
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! docker compose version &> /dev/null; then
        if ! command -v docker-compose &> /dev/null; then
            log_error "Docker Compose is not installed or not in PATH"
            exit 1
        fi
        # Create a function to use docker-compose instead of docker compose
        docker_compose_cmd="docker-compose"
    else
        docker_compose_cmd="docker compose"
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Build the migration tool
build_tool() {
    log_info "Building migration tool..."
    make build
    log_success "Migration tool built successfully"
}

# Start test environment
start_environment() {
    log_info "Starting test environment..."
    
    # Create custom docker-compose for testing
    cat > docker-compose.test.yml << EOF
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: ${COMPOSE_PROJECT}-redis
    ports:
      - "${REDIS_PORT}:6379"
    command: redis-server --appendonly yes
    networks:
      - test-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

  valkey:
    image: valkey/valkey:7.2-alpine
    container_name: ${COMPOSE_PROJECT}-valkey
    ports:
      - "${VALKEY_PORT}:6379"
    command: valkey-server --appendonly yes
    networks:
      - test-network
    healthcheck:
      test: ["CMD", "valkey-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

networks:
  test-network:
    driver: bridge
EOF

    # Start services
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be ready..."
    # Wait for Redis
    for i in {1..30}; do
        if $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli ping > /dev/null 2>&1; then
            log_info "Redis is ready"
            break
        fi
        echo "Waiting for Redis... (attempt $i/30)"
        sleep 2
        if [ $i -eq 30 ]; then
            log_error "Redis failed to start within 60 seconds"
            exit 1
        fi
    done
    
    # Wait for Valkey
    for i in {1..30}; do
        if $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli ping > /dev/null 2>&1; then
            log_info "Valkey is ready"
            break
        fi
        echo "Waiting for Valkey... (attempt $i/30)"
        sleep 2
        if [ $i -eq 30 ]; then
            log_error "Valkey failed to start within 60 seconds"
            exit 1
        fi
    done
    
    log_success "Test environment started successfully"
}

# Run unit tests first
run_unit_tests() {
    log_info "Running unit tests..."
    go test -v -timeout="$TEST_TIMEOUT" ./internal/... ./pkg/...
    log_success "Unit tests passed"
}

# Run property-based tests
run_property_tests() {
    log_info "Running property-based tests..."
    go test -v -timeout="$TEST_TIMEOUT" -run "Property" ./...
    log_success "Property-based tests passed"
}

# Populate test data
populate_test_data() {
    log_info "Populating Redis with test data..."
    
    # Basic data types
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli SET test:string "hello world"
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli HSET test:hash field1 value1 field2 value2
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli LPUSH test:list item1 item2 item3
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli SADD test:set member1 member2 member3
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli ZADD test:zset 1 member1 2 member2 3 member3
    
    # Large dataset
    for i in {1..100}; do
        $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli SET "large:key:$i" "value_$i"
    done
    
    log_success "Test data populated"
}

# Run migration test
run_migration_test() {
    log_info "Running migration test..."
    
    # Test dry run first
    log_info "Testing dry run..."
    ./build/redis-valkey-migration migrate \
        --redis-host localhost \
        --redis-port "$REDIS_PORT" \
        --valkey-host localhost \
        --valkey-port "$VALKEY_PORT" \
        --dry-run \
        --log-level debug
    
    # Run actual migration
    log_info "Running actual migration..."
    ./build/redis-valkey-migration migrate \
        --redis-host localhost \
        --redis-port "$REDIS_PORT" \
        --valkey-host localhost \
        --valkey-port "$VALKEY_PORT" \
        --batch-size 50 \
        --log-level info \
        --verify true
    
    log_success "Migration completed successfully"
}

# Verify migration results
verify_migration() {
    log_info "Verifying migration results..."
    
    # Check key counts
    redis_count=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli DBSIZE | tr -d '\r')
    valkey_count=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli DBSIZE | tr -d '\r')
    
    log_info "Redis keys: $redis_count, Valkey keys: $valkey_count"
    
    if [ "$redis_count" != "$valkey_count" ]; then
        log_error "Key count mismatch: Redis=$redis_count, Valkey=$valkey_count"
        exit 1
    fi
    
    # Verify specific data
    string_val=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli GET test:string | tr -d '\r')
    if [ "$string_val" != "hello world" ]; then
        log_error "String value mismatch: expected 'hello world', got '$string_val'"
        exit 1
    fi
    
    hash_val=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli HGET test:hash field1 | tr -d '\r')
    if [ "$hash_val" != "value1" ]; then
        log_error "Hash value mismatch: expected 'value1', got '$hash_val'"
        exit 1
    fi
    
    list_len=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli LLEN test:list | tr -d '\r')
    if [ "$list_len" != "3" ]; then
        log_error "List length mismatch: expected '3', got '$list_len'"
        exit 1
    fi
    
    log_success "Migration verification passed"
}

# Run integration tests
run_integration_tests() {
    log_info "Running Go integration tests..."
    
    # Set environment variables for tests
    export REDIS_HOST=localhost
    export REDIS_PORT="$REDIS_PORT"
    export VALKEY_HOST=localhost
    export VALKEY_PORT="$VALKEY_PORT"
    
    # Run integration tests
    go test -v -timeout="$TEST_TIMEOUT" ./test/integration/...
    
    log_success "Integration tests passed"
}

# Test error scenarios
test_error_scenarios() {
    log_info "Testing error scenarios..."
    
    # Test with invalid Redis host (should fail gracefully)
    log_info "Testing invalid Redis host..."
    if ./build/redis-valkey-migration migrate \
        --redis-host invalid-host \
        --redis-port "$REDIS_PORT" \
        --valkey-host localhost \
        --valkey-port "$VALKEY_PORT" \
        --dry-run 2>/dev/null; then
        log_error "Migration should have failed with invalid Redis host"
        exit 1
    fi
    
    # Test with invalid Valkey host (should fail gracefully)
    log_info "Testing invalid Valkey host..."
    if ./build/redis-valkey-migration migrate \
        --redis-host localhost \
        --redis-port "$REDIS_PORT" \
        --valkey-host invalid-host \
        --valkey-port "$VALKEY_PORT" \
        --dry-run 2>/dev/null; then
        log_error "Migration should have failed with invalid Valkey host"
        exit 1
    fi
    
    log_success "Error scenario tests passed"
}

# Performance test
run_performance_test() {
    log_info "Running performance test..."
    
    # Clear databases
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli FLUSHDB
    $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli FLUSHDB
    
    # Create larger dataset
    log_info "Creating large dataset (1000 keys)..."
    for i in {1..1000}; do
        $docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T redis redis-cli SET "perf:key:$i" "performance_test_value_$i" > /dev/null
    done
    
    # Run migration with timing
    start_time=$(date +%s)
    ./build/redis-valkey-migration migrate \
        --redis-host localhost \
        --redis-port "$REDIS_PORT" \
        --valkey-host localhost \
        --valkey-port "$VALKEY_PORT" \
        --batch-size 100 \
        --max-concurrency 5 \
        --log-level warn
    end_time=$(date +%s)
    
    duration=$((end_time - start_time))
    log_success "Performance test completed in ${duration}s (1000 keys)"
    
    # Verify count
    valkey_count=$($docker_compose_cmd -f docker-compose.test.yml -p "$COMPOSE_PROJECT" exec -T valkey valkey-cli DBSIZE | tr -d '\r')
    if [ "$valkey_count" != "1000" ]; then
        log_error "Performance test failed: expected 1000 keys, got $valkey_count"
        exit 1
    fi
}

# Show usage
usage() {
    echo "Integration Test Script for Redis to Valkey Migration Tool"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --unit-only       Run only unit tests"
    echo "  --integration-only Run only integration tests"
    echo "  --performance     Include performance tests"
    echo "  --skip-build      Skip building the migration tool"
    echo "  --help            Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  REDIS_PORT        Redis port for testing (default: $REDIS_PORT)"
    echo "  VALKEY_PORT       Valkey port for testing (default: $VALKEY_PORT)"
    echo "  TEST_TIMEOUT      Test timeout (default: $TEST_TIMEOUT)"
}

# Main execution
main() {
    local unit_only=false
    local integration_only=false
    local include_performance=false
    local skip_build=false
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --unit-only)
                unit_only=true
                shift
                ;;
            --integration-only)
                integration_only=true
                shift
                ;;
            --performance)
                include_performance=true
                shift
                ;;
            --skip-build)
                skip_build=true
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    log_info "Starting integration test suite..."
    
    check_prerequisites
    
    if [ "$skip_build" = false ]; then
        build_tool
    fi
    
    if [ "$integration_only" = false ]; then
        run_unit_tests
        run_property_tests
    fi
    
    if [ "$unit_only" = false ]; then
        start_environment
        populate_test_data
        run_migration_test
        verify_migration
        run_integration_tests
        test_error_scenarios
        
        if [ "$include_performance" = true ]; then
            run_performance_test
        fi
    fi
    
    log_success "All tests passed successfully!"
}

# Execute main function
main "$@"