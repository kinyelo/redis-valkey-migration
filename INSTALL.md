# Installation Guide

This guide provides instructions for installing and setting up the Redis to Valkey Migration Tool.

## Prerequisites

- Go 1.25 or later (for building from source)
- Redis server (source database)
- Valkey server (target database)
- Docker and Docker Compose (for containerized deployment)

## Installation Methods

### Method 1: Download Pre-built Binary

1. Download the appropriate binary for your platform from the releases page:
   - Linux AMD64: `redis-valkey-migration-linux-amd64`
   - Linux ARM64: `redis-valkey-migration-linux-arm64`
   - macOS AMD64: `redis-valkey-migration-darwin-amd64`
   - macOS ARM64: `redis-valkey-migration-darwin-arm64`
   - Windows AMD64: `redis-valkey-migration-windows-amd64.exe`

2. Make the binary executable (Linux/macOS):
   ```bash
   chmod +x redis-valkey-migration-*
   ```

3. Move to a directory in your PATH:
   ```bash
   sudo mv redis-valkey-migration-* /usr/local/bin/redis-valkey-migration
   ```

### Method 2: Build from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/your-org/redis-valkey-migration.git
   cd redis-valkey-migration
   ```

2. Set up development environment:
   ```bash
   make dev-setup
   ```

3. Build the application:
   ```bash
   make build
   ```

4. Install to GOPATH/bin:
   ```bash
   make install
   ```

### Method 3: Docker Container

1. Build the Docker image:
   ```bash
   make docker-build
   ```

2. Or pull from registry (if available):
   ```bash
   docker pull your-registry/redis-valkey-migration:latest
   ```

## Quick Start

### Basic Migration

1. Ensure both Redis and Valkey servers are running
2. Run a dry-run to preview the migration:
   ```bash
   redis-valkey-migration migrate --dry-run
   ```

3. Perform the actual migration:
   ```bash
   redis-valkey-migration migrate
   ```

### With Custom Configuration

```bash
redis-valkey-migration migrate \
  --redis-host redis.example.com \
  --redis-port 6379 \
  --redis-password myredispass \
  --valkey-host valkey.example.com \
  --valkey-port 6380 \
  --valkey-password myvalkeypass \
  --batch-size 1000 \
  --log-level debug
```

### With Timeout Configuration

For databases with large data structures or slow networks:

```bash
redis-valkey-migration migrate \
  --connection-timeout 60s \
  --hash-timeout 120s \
  --list-timeout 90s \
  --large-data-threshold 5000 \
  --large-data-multiplier 5.0
```

### Using Docker Compose

1. Start Redis and Valkey services:
   ```bash
   docker-compose up -d redis valkey
   ```

2. Wait for services to be healthy, then run migration:
   ```bash
   docker-compose run --rm migration-tool migrate \
     --redis-host redis \
     --valkey-host valkey \
     --dry-run
   ```

## Configuration

### Environment Variables

The tool supports configuration via environment variables:

```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=secret
export REDIS_DATABASE=0
export VALKEY_HOST=localhost
export VALKEY_PORT=6380
export VALKEY_PASSWORD=secret
export VALKEY_DATABASE=0
export LOG_LEVEL=info

# Timeout Configuration
export REDIS_VALKEY_CONNECTION_TIMEOUT=30s
export REDIS_VALKEY_STRING_TIMEOUT=10s
export REDIS_VALKEY_HASH_TIMEOUT=30s
export REDIS_VALKEY_LIST_TIMEOUT=30s
export REDIS_VALKEY_SET_TIMEOUT=30s
export REDIS_VALKEY_SORTED_SET_TIMEOUT=30s
export REDIS_VALKEY_LARGE_DATA_THRESHOLD=1000
export REDIS_VALKEY_LARGE_DATA_MULTIPLIER=3.0
```

### Configuration File

Create a configuration file at `~/.redis-valkey-migration/config.yaml`:

```yaml
redis:
  host: localhost
  port: 6379
  password: ""
  database: 0

valkey:
  host: localhost
  port: 6380
  password: ""
  database: 0

migration:
  batch_size: 1000
  retry_attempts: 3
  log_level: info

timeouts:
  connection: 30s
  string: 10s
  hash: 30s
  list: 30s
  set: 30s
  sorted_set: 30s
  large_data_threshold: 1000
  large_data_multiplier: 3.0
```

## Verification

Verify the installation:

```bash
redis-valkey-migration version
```

Run help to see all available options:

```bash
redis-valkey-migration --help
redis-valkey-migration migrate --help
```

## Troubleshooting

### Common Issues

1. **Connection refused**: Ensure Redis and Valkey servers are running and accessible
2. **Authentication failed**: Verify passwords and authentication settings
3. **Permission denied**: Ensure the user has read access to Redis and write access to Valkey
4. **Out of memory**: Reduce batch size or increase available memory

### Logs

Check the migration log file for detailed information:
```bash
tail -f migration.log
```

### Support

For issues and support:
1. Check the logs for detailed error messages
2. Run with `--verbose` flag for debug information
3. Use `--dry-run` to test configuration without data transfer
4. Consult the documentation and examples

## Next Steps

- Read the [Usage Guide](USAGE.md) for detailed usage instructions
- Review the [Configuration Reference](CONFIG.md) for all configuration options
- See [Examples](examples/) for common migration scenarios