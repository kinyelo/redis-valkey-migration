# Usage Guide

This guide provides detailed instructions for using the Redis to Valkey Migration Tool.

## Overview

The Redis to Valkey Migration Tool is designed to safely and efficiently migrate all data from a Redis database to a Valkey database while maintaining data integrity and providing comprehensive monitoring.

## Basic Usage

### Command Structure

```bash
redis-valkey-migration [global-flags] <command> [command-flags]
```

### Global Flags

- `--config, -c`: Configuration file path
- `--verbose, -v`: Enable verbose output
- `--dry-run`: Perform dry run without actual migration
- `--help, -h`: Show help information

## Commands

### migrate

Performs the actual data migration from Redis to Valkey.

```bash
redis-valkey-migration migrate [flags]
```

#### Connection Flags

**Redis Connection:**
- `--redis-host`: Redis server hostname (default: localhost)
- `--redis-port`: Redis server port (default: 6379)
- `--redis-password`: Redis authentication password
- `--redis-database`: Redis database number (default: 0)

**Valkey Connection:**
- `--valkey-host`: Valkey server hostname (default: localhost)
- `--valkey-port`: Valkey server port (default: 6380)
- `--valkey-password`: Valkey authentication password
- `--valkey-database`: Valkey database number (default: 0)

#### Migration Behavior Flags

- `--batch-size`: Keys per batch (default: 1000)
- `--retry-attempts`: Retry attempts for failures (default: 3)
- `--log-level`: Logging level (default: info)
- `--verify`: Verify migration after completion (default: true)
- `--continue-on-error`: Continue on individual key failures (default: true)
- `--resume-file`: Resume state file (default: migration_resume.json)
- `--progress-interval`: Progress reporting interval (default: 5s)
- `--max-concurrency`: Maximum concurrent operations (default: 10)

#### Collection Pattern Flags

The tool supports migrating specific collections of keys using glob-style patterns:

- `--pattern`: Key patterns to migrate (can be specified multiple times)
- `--collections`: Alias for `--pattern` (can be specified multiple times)

**Pattern Syntax:**
- Use `*` to match any characters: `user:*` matches `user:123`, `user:abc`, etc.
- Use `?` to match single character: `user:?` matches `user:1`, `user:a`, etc.
- Use `[abc]` to match any character in brackets: `user:[123]` matches `user:1`, `user:2`, `user:3`
- Patterns are case-sensitive
- Multiple patterns can be specified to migrate different collections
- If no patterns are specified, all keys will be migrated

**Environment Variables:**
Collection patterns can also be set using environment variables:
- `RVM_MIGRATION_COLLECTION_PATTERNS`: Comma-separated list of patterns

#### Timeout Configuration Flags

The tool provides configurable timeouts for different operations to handle large data structures and varying network conditions:

**Connection Timeouts:**
- `--connection-timeout`: Database connection timeout (default: 30s)

**Operation-Specific Timeouts:**
- `--string-timeout`: Timeout for string operations (default: 10s)
- `--hash-timeout`: Timeout for hash operations (default: 30s)
- `--list-timeout`: Timeout for list operations (default: 30s)
- `--set-timeout`: Timeout for set operations (default: 30s)
- `--sorted-set-timeout`: Timeout for sorted set operations (default: 30s)

**Large Data Handling:**
- `--large-data-threshold`: Size threshold for large data detection (default: 1000 elements/fields)
- `--large-data-multiplier`: Timeout multiplier for large data (default: 3.0)

**Environment Variables:**
All timeout flags can also be set using environment variables:
- `REDIS_VALKEY_CONNECTION_TIMEOUT`
- `REDIS_VALKEY_STRING_TIMEOUT`
- `REDIS_VALKEY_HASH_TIMEOUT`
- `REDIS_VALKEY_LIST_TIMEOUT`
- `REDIS_VALKEY_SET_TIMEOUT`
- `REDIS_VALKEY_SORTED_SET_TIMEOUT`
- `REDIS_VALKEY_LARGE_DATA_THRESHOLD`
- `REDIS_VALKEY_LARGE_DATA_MULTIPLIER`

### version

Display version and build information.

```bash
redis-valkey-migration version
```

## Usage Examples

### Basic Migration

Migrate from local Redis to local Valkey with default settings:

```bash
redis-valkey-migration migrate
```

### Custom Hosts and Ports

Migrate between remote servers:

```bash
redis-valkey-migration migrate \
  --redis-host redis.example.com \
  --redis-port 6379 \
  --valkey-host valkey.example.com \
  --valkey-port 6380
```

### With Authentication

Migrate with password authentication:

```bash
redis-valkey-migration migrate \
  --redis-password "redis-secret-123" \
  --valkey-password "valkey-secret-456"
```

### Different Databases

Migrate from Redis DB 1 to Valkey DB 2:

```bash
redis-valkey-migration migrate \
  --redis-database 1 \
  --valkey-database 2
```

### Performance Tuning

High-performance migration with custom settings:

```bash
redis-valkey-migration migrate \
  --batch-size 2000 \
  --max-concurrency 20 \
  --progress-interval 10s \
  --log-level warn
```

### Dry Run

Preview what would be migrated without actual transfer:

```bash
redis-valkey-migration migrate --dry-run --verbose
```

### Resume Interrupted Migration

Resume a previously interrupted migration:

```bash
redis-valkey-migration migrate \
  --resume-file /path/to/previous/migration_resume.json
```

### Debug Mode

Run with maximum logging for troubleshooting:

```bash
redis-valkey-migration migrate \
  --log-level debug \
  --verbose
```

### Collection Pattern Migration

#### Migrate Specific Collections

Migrate only user-related keys:

```bash
redis-valkey-migration migrate --pattern "user:*"
```

#### Multiple Collections

Migrate user and session data:

```bash
redis-valkey-migration migrate \
  --pattern "user:*" \
  --pattern "session:*"
```

#### Complex Patterns

Migrate specific data structures:

```bash
redis-valkey-migration migrate \
  --pattern "user:*:profile" \
  --pattern "cache:data:*" \
  --pattern "temp_*"
```

#### Using Collections Alias

Alternative syntax using `--collections`:

```bash
redis-valkey-migration migrate \
  --collections "user:*" \
  --collections "session:*"
```

#### Dry Run with Patterns

Preview what would be migrated with patterns:

```bash
redis-valkey-migration migrate \
  --dry-run \
  --pattern "production:*" \
  --verbose
```

#### Environment Variable Patterns

Set patterns via environment variable:

```bash
export RVM_MIGRATION_COLLECTION_PATTERNS="user:*,session:*,cache:*"
redis-valkey-migration migrate
```

### Timeout Configuration

#### Default Timeout Settings

For most use cases, the default timeout settings work well:

```bash
redis-valkey-migration migrate
```

#### Custom Timeout Settings

For databases with large data structures or slow networks:

```bash
redis-valkey-migration migrate \
  --connection-timeout 60s \
  --hash-timeout 120s \
  --list-timeout 90s \
  --large-data-threshold 5000 \
  --large-data-multiplier 5.0
```

#### High-Performance Networks

For fast, reliable networks with smaller data:

```bash
redis-valkey-migration migrate \
  --connection-timeout 10s \
  --string-timeout 5s \
  --hash-timeout 15s \
  --list-timeout 15s \
  --set-timeout 15s \
  --sorted-set-timeout 15s
```

#### Using Environment Variables

Set timeouts via environment variables:

```bash
export REDIS_VALKEY_CONNECTION_TIMEOUT=45s
export REDIS_VALKEY_HASH_TIMEOUT=60s
export REDIS_VALKEY_LARGE_DATA_THRESHOLD=2000
redis-valkey-migration migrate
```

## Migration Process

### Phase 1: Connection and Discovery

1. Validates configuration parameters
2. Establishes connections to Redis and Valkey
3. Discovers keys in the Redis database (all keys or filtered by patterns)
4. Reports total number of keys to migrate (filtered count if patterns used)

### Phase 2: Data Transfer

1. Processes keys in batches
2. Identifies data type for each key
3. Applies appropriate timeout based on data type and size
4. Transfers data using appropriate commands
5. Automatically scales timeouts for large data structures
6. Reports progress at regular intervals
7. Handles errors with retry logic

### Phase 3: Verification

1. Verifies each transferred key exists in Valkey
2. Compares data integrity between Redis and Valkey
3. Reports any verification failures
4. Generates final migration statistics

## Data Types Supported

The tool supports all standard Redis data types:

- **Strings**: Simple key-value pairs
- **Hashes**: Field-value mappings
- **Lists**: Ordered collections of strings
- **Sets**: Unordered collections of unique strings
- **Sorted Sets**: Ordered sets with scores

## Error Handling

### Automatic Recovery

- **Connection Loss**: Automatic reconnection with exponential backoff
- **Network Errors**: Up to 3 retry attempts per operation
- **Timeout Errors**: Automatic timeout scaling for large data structures
- **Partial Failures**: Continue migration for remaining keys

### Resume Capability

If migration is interrupted:

1. State is saved to resume file
2. Restart with same resume file
3. Tool skips already migrated keys
4. Continues from last checkpoint

### Error Reporting

All errors are logged with:
- Timestamp and severity level
- Affected key name and operation
- Detailed error message
- Recovery actions taken

## Monitoring and Logging

### Progress Reporting

Real-time progress includes:
- Total keys discovered
- Keys processed and remaining
- Current processing rate
- Estimated time to completion
- Error count and failed keys

### Log Files

Detailed logs are written to `migration.log` with:
- Structured JSON or text format
- Configurable log levels
- Automatic log rotation
- Complete audit trail

### Performance Metrics

Final statistics include:
- Total migration time
- Keys per second throughput
- Success and failure counts
- Data volume transferred

## Best Practices

### Before Migration

1. **Backup**: Create backups of both databases
2. **Test**: Run dry-run to validate configuration
3. **Patterns**: Use collection patterns for targeted migrations when appropriate
4. **Resources**: Ensure adequate network and memory resources
5. **Monitoring**: Set up monitoring for both databases

### During Migration

1. **Monitor**: Watch progress and error logs
2. **Resources**: Monitor CPU, memory, and network usage
3. **Databases**: Ensure both databases remain accessible
4. **Interruption**: Use Ctrl+C for graceful shutdown if needed

### After Migration

1. **Verify**: Check final statistics and verification results
2. **Test**: Validate application functionality with Valkey
3. **Cleanup**: Remove temporary files and logs if desired
4. **Monitor**: Monitor Valkey performance and stability

## Troubleshooting

### Common Issues

**Connection Problems:**
```bash
# Test connectivity
redis-cli -h redis-host -p 6379 ping
valkey-cli -h valkey-host -p 6380 ping
```

**Authentication Issues:**
```bash
# Test with credentials
redis-cli -h redis-host -p 6379 -a password ping
```

**Performance Issues:**
- Reduce batch size for memory constraints
- Increase concurrency for faster networks
- Adjust progress interval for less logging overhead

**Timeout Issues:**
- Increase operation timeouts for slow networks or large data
- Adjust large data threshold and multiplier for your data patterns
- Monitor logs for timeout scaling messages
- Use environment variables for persistent timeout configuration

**Verification Failures:**
- Check for data type mismatches
- Verify TTL and expiration handling
- Review error logs for specific failures

### Getting Help

```bash
# Show general help
redis-valkey-migration --help

# Show command-specific help
redis-valkey-migration migrate --help

# Show version information
redis-valkey-migration version
```