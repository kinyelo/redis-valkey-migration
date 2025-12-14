# Redis to Valkey Migration Tool

A robust Go-based command-line application that provides complete data migration between Redis and Valkey databases with comprehensive monitoring, error handling, and verification capabilities.

## Features

- **Complete Data Migration**: Migrate all Redis data types to Valkey
- **Data Type Support**: Strings, hashes, lists, sets, and sorted sets
- **Progress Monitoring**: Real-time progress reporting and statistics
- **Data Integrity Verification**: Automatic verification after migration
- **Error Handling & Recovery**: Robust error handling with retry logic
- **Resume Capability**: Resume interrupted migrations from checkpoints
- **Graceful Shutdown**: Safe interruption with state preservation
- **Flexible Configuration**: CLI flags, environment variables, and config files
- **Docker Support**: Containerized deployment with Docker Compose
- **Comprehensive Testing**: Unit, property-based, and end-to-end tests

## Quick Start

```bash
# Download and install
go install github.com/kinyelo/redis-valkey-migration@latest

# Basic migration (dry run first)
redis-valkey-migration migrate --dry-run

# Perform actual migration
redis-valkey-migration migrate
```

## Project Structure

```
├── main.go                       # Application entry point
├── internal/                     # Private application code
│   ├── client/                   # Redis and Valkey client implementations
│   ├── config/                   # Configuration management and CLI
│   ├── engine/                   # Migration engine with error handling
│   ├── monitor/                  # Progress monitoring and statistics
│   ├── processor/                # Data type-specific processors
│   ├── scanner/                  # Key discovery and scanning
│   ├── verifier/                 # Data integrity verification
│   └── version/                  # Version and build information
├── pkg/                          # Public library code
│   ├── logger/                   # Structured logging with rotation
│   └── types/                    # Shared type definitions
├── test/                         # Test files and fixtures
│   ├── integration/              # End-to-end integration tests
│   └── fixtures/                 # Test data and scenarios
├── scripts/                      # Build and test automation
├── build/                        # Build artifacts (generated)
└── dist/                         # Distribution packages (generated)
```

## Dependencies

- **Core Libraries**:
  - `github.com/redis/go-redis/v9` - Modern Redis client library
  - `github.com/spf13/cobra` - CLI framework and command handling
  - `github.com/spf13/viper` - Configuration management
  - `github.com/sirupsen/logrus` - Structured logging

- **Testing**:
  - `github.com/stretchr/testify` - Testing utilities and assertions
  - `github.com/leanovate/gopter` - Property-based testing

## Installation

### Pre-built Binaries

Download from releases for your platform:
- Linux (AMD64/ARM64)
- macOS (Intel/Apple Silicon) 
- Windows (AMD64)

### Build from Source

```bash
# Clone repository
git clone https://github.com/kinyelo/redis-valkey-migration.git
cd redis-valkey-migration

# Set up development environment
make dev-setup

# Build for current platform
make build

# Build for all platforms
make build-all
```

### Docker

```bash
# Build Docker image
make docker-build

# Or use Docker Compose
docker-compose up -d redis valkey
docker-compose run --rm migration-tool migrate --dry-run
```

## Usage Examples

### Basic Migration

```bash
# Dry run to preview migration
redis-valkey-migration migrate --dry-run

# Basic migration with default settings
redis-valkey-migration migrate
```

### Custom Configuration

```bash
# Migration with custom hosts and authentication
redis-valkey-migration migrate \
  --redis-host redis.example.com \
  --redis-password secret123 \
  --valkey-host valkey.example.com \
  --valkey-password secret456 \
  --batch-size 1000 \
  --log-level debug
```

### Performance Tuning

```bash
# High-performance migration
redis-valkey-migration migrate \
  --batch-size 2000 \
  --max-concurrency 20 \
  --progress-interval 10s
```

### Resume Interrupted Migration

```bash
# Resume from previous state
redis-valkey-migration migrate \
  --resume-file migration_state.json
```

## Development

### Setup

```bash
make dev-setup    # Install dependencies and tools
make fmt          # Format code
make lint         # Run linter
```

### Testing

```bash
make test              # Run all tests (auto-detects Docker)
make test-e2e          # Run end-to-end tests with containers
make test-coverage     # Generate coverage report
make test-property     # Run property-based tests
```

### Building

```bash
make build        # Build for current platform
make build-all    # Build for all platforms
make package      # Create release packages
```

## Configuration

The tool supports multiple configuration methods:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (`~/.redis-valkey-migration/config.yaml`)

See [INSTALL.md](INSTALL.md) for detailed installation instructions and [USAGE.md](USAGE.md) for comprehensive usage examples.

## Architecture

The application follows a modular architecture:

- **Engine**: Orchestrates the migration process with error handling
- **Scanner**: Discovers and categorizes keys in the source database
- **Processor**: Handles data type-specific migration logic
- **Monitor**: Tracks progress and generates statistics
- **Verifier**: Ensures data integrity after migration
- **Client**: Abstracts database operations for Redis and Valkey

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `make test-all`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.