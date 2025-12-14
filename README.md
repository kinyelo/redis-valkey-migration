# Redis to Valkey Migration Tool

A Go-based command-line application that provides complete data migration between Redis and Valkey databases.

## Features

- Complete data migration from Redis to Valkey
- Support for all Redis data types (strings, hashes, lists, sets, sorted sets)
- Comprehensive progress monitoring and logging
- Data integrity verification
- Error handling and recovery mechanisms
- Resume capability for interrupted migrations

## Project Structure

```
├── cmd/
│   └── redis-valkey-migration/    # Main application entry point
├── internal/                      # Private application code
│   ├── config/                   # Configuration management
│   ├── client/                   # Database client implementations
│   ├── processor/                # Data type processors
│   ├── monitor/                  # Progress monitoring
│   └── engine/                   # Migration engine orchestration
├── pkg/                          # Public library code
│   ├── logger/                   # Logging utilities
│   └── types/                    # Shared type definitions
└── test/                         # Test files
    ├── integration/              # Integration tests
    └── fixtures/                 # Test data fixtures
```

## Dependencies

- `github.com/go-redis/redis/v8` - Redis client library
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/stretchr/testify` - Testing utilities
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management

## Building

```bash
make build
```

## Usage

```bash
./bin/redis-valkey-migration --help
```

## Development

This project follows Go best practices with a clear separation of concerns:

- `cmd/` contains the main application entry points
- `internal/` contains private application logic
- `pkg/` contains reusable library code
- `test/` contains test files and fixtures

## Testing

Run tests with:

```bash
make test
```