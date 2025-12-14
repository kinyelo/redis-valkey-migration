#!/bin/bash

# Redis to Valkey Migration Tool Build Script
# This script handles building, testing, and packaging the application

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERSION=${VERSION:-"1.0.0"}
GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}
BUILD_DIR=${BUILD_DIR:-"build"}
DIST_DIR=${DIST_DIR:-"dist"}

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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    GO_VERSION=$(go version | cut -d' ' -f3 | sed 's/go//')
    log_info "Go version: $GO_VERSION"
    
    if ! command -v git &> /dev/null; then
        log_warning "Git is not installed, using default commit hash"
    fi
}

# Clean build artifacts
clean() {
    log_info "Cleaning build artifacts..."
    rm -rf "$BUILD_DIR" "$DIST_DIR"
    go clean
    log_success "Clean completed"
}

# Format code
format() {
    log_info "Formatting code..."
    go fmt ./...
    log_success "Code formatting completed"
}

# Run tests
test() {
    log_info "Running tests..."
    go test -v ./...
    log_success "Tests completed"
}

# Run tests with coverage
test_coverage() {
    log_info "Running tests with coverage..."
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    log_success "Coverage report generated: coverage.html"
}

# Build for current platform
build() {
    log_info "Building for current platform..."
    mkdir -p "$BUILD_DIR"
    
    LDFLAGS="-X redis-valkey-migration/internal/version.Version=$VERSION \
             -X redis-valkey-migration/internal/version.GitCommit=$GIT_COMMIT \
             -X redis-valkey-migration/internal/version.BuildDate=$BUILD_DATE"
    
    go build -ldflags "$LDFLAGS" -o "$BUILD_DIR/redis-valkey-migration" .
    
    log_success "Build completed: $BUILD_DIR/redis-valkey-migration"
}

# Build for all platforms
build_all() {
    log_info "Building for all platforms..."
    mkdir -p "$DIST_DIR"
    
    LDFLAGS="-X redis-valkey-migration/internal/version.Version=$VERSION \
             -X redis-valkey-migration/internal/version.GitCommit=$GIT_COMMIT \
             -X redis-valkey-migration/internal/version.BuildDate=$BUILD_DATE"
    
    # Define platforms
    platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "$platform"
        output_name="redis-valkey-migration-$GOOS-$GOARCH"
        
        if [ "$GOOS" = "windows" ]; then
            output_name="${output_name}.exe"
        fi
        
        log_info "Building for $GOOS/$GOARCH..."
        
        env GOOS="$GOOS" GOARCH="$GOARCH" go build \
            -ldflags "$LDFLAGS" \
            -o "$DIST_DIR/$output_name" .
        
        if [ $? -eq 0 ]; then
            log_success "Built: $DIST_DIR/$output_name"
        else
            log_error "Failed to build for $GOOS/$GOARCH"
            exit 1
        fi
    done
    
    log_success "All platform builds completed"
    ls -la "$DIST_DIR/"
}

# Create release packages
package() {
    log_info "Creating release packages..."
    
    if [ ! -d "$DIST_DIR" ]; then
        log_error "Distribution directory not found. Run build_all first."
        exit 1
    fi
    
    mkdir -p "$DIST_DIR/packages"
    
    # Create tar.gz for Unix systems
    cd "$DIST_DIR"
    
    for binary in redis-valkey-migration-linux-* redis-valkey-migration-darwin-*; do
        if [ -f "$binary" ]; then
            package_name="${binary}-${VERSION}.tar.gz"
            tar -czf "packages/$package_name" "$binary"
            log_success "Created: packages/$package_name"
        fi
    done
    
    # Create zip for Windows
    for binary in redis-valkey-migration-windows-*.exe; do
        if [ -f "$binary" ]; then
            package_name="${binary%.exe}-${VERSION}.zip"
            zip "packages/$package_name" "$binary"
            log_success "Created: packages/$package_name"
        fi
    done
    
    cd - > /dev/null
    
    log_success "Release packages created:"
    ls -la "$DIST_DIR/packages/"
}

# Docker build
docker_build() {
    log_info "Building Docker image..."
    
    docker build \
        --build-arg VERSION="$VERSION" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        --build-arg BUILD_DATE="$BUILD_DATE" \
        -t "redis-valkey-migration:$VERSION" \
        -t "redis-valkey-migration:latest" \
        .
    
    log_success "Docker image built: redis-valkey-migration:$VERSION"
}

# Show usage
usage() {
    echo "Redis to Valkey Migration Tool Build Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  clean         - Clean build artifacts"
    echo "  format        - Format Go code"
    echo "  test          - Run tests"
    echo "  test-coverage - Run tests with coverage"
    echo "  build         - Build for current platform"
    echo "  build-all     - Build for all platforms"
    echo "  package       - Create release packages"
    echo "  docker-build  - Build Docker image"
    echo "  all           - Run clean, format, test, and build"
    echo "  release       - Run all, build-all, and package"
    echo ""
    echo "Environment Variables:"
    echo "  VERSION       - Version to build (default: $VERSION)"
    echo "  GIT_COMMIT    - Git commit hash (default: auto-detected)"
    echo "  BUILD_DATE    - Build timestamp (default: current time)"
    echo "  BUILD_DIR     - Build directory (default: $BUILD_DIR)"
    echo "  DIST_DIR      - Distribution directory (default: $DIST_DIR)"
}

# Main execution
main() {
    case "${1:-}" in
        "clean")
            clean
            ;;
        "format")
            format
            ;;
        "test")
            test
            ;;
        "test-coverage")
            test_coverage
            ;;
        "build")
            check_prerequisites
            format
            build
            ;;
        "build-all")
            check_prerequisites
            format
            build_all
            ;;
        "package")
            package
            ;;
        "docker-build")
            docker_build
            ;;
        "all")
            check_prerequisites
            clean
            format
            test
            build
            ;;
        "release")
            check_prerequisites
            clean
            format
            test
            build_all
            package
            ;;
        "help"|"--help"|"-h")
            usage
            ;;
        "")
            usage
            ;;
        *)
            log_error "Unknown command: $1"
            usage
            exit 1
            ;;
    esac
}

# Execute main function
main "$@"