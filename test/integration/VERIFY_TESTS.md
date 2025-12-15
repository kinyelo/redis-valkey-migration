# Verification Tests

This directory contains comprehensive tests for the `--verify` flag functionality in the Redis to Valkey migration tool.

## Test Files

### 1. `verify_test.go` - Engine-Level Verification Tests
Tests the verification functionality at the migration engine level:

- **TestVerifyEnabled**: Tests migration with verification enabled (`VerifyAfterMigration: true`)
- **TestVerifyDisabled**: Tests migration with verification disabled (`VerifyAfterMigration: false`)
- **TestVerifyWithCorruptedData**: Tests verification behavior when data integrity issues are detected
- **TestVerifyLargeDataset**: Tests verification with a large number of keys (500 keys)
- **TestVerifyComplexDataTypes**: Tests verification with complex Redis data structures

### 2. `cli_verify_test.go` - CLI-Level Verification Tests
Tests the `--verify` flag from the command-line interface using Docker containers:

- **TestCLIVerifyFlagEnabled**: Tests `--verify=true` flag
- **TestCLIVerifyFlagDisabled**: Tests `--verify=false` flag
- **TestCLIVerifyFlagDefault**: Tests default behavior (verification should be enabled by default)
- **TestCLIVerifyWithCorruptedData**: Tests CLI verification failure detection
- **TestCLIVerifyLargeDataset**: Tests CLI verification with larger datasets (200 keys)

### 3. `verify_flag_test.go` - Flag Parsing Tests
Unit tests for the `--verify` flag parsing logic:

- **TestVerifyFlagParsing**: Tests various flag formats (`--verify=true`, `--verify=false`, etc.)
- **TestVerifyFlagInEngineConfig**: Tests how the flag affects engine configuration
- **TestDefaultEngineConfigVerifyValue**: Tests that verification is enabled by default
- **TestEngineConfigVerifyField**: Tests the `VerifyAfterMigration` field behavior

## Test Coverage

The tests cover:

1. **Flag Parsing**: Ensures `--verify` flag is correctly parsed in various formats
2. **Default Behavior**: Verifies that verification is enabled by default
3. **Engine Integration**: Tests verification at the migration engine level
4. **CLI Integration**: Tests verification through the command-line interface
5. **Error Detection**: Tests that verification correctly detects data corruption
6. **Performance**: Tests verification with large datasets
7. **Data Types**: Tests verification with all Redis data types (strings, hashes, lists, sets, sorted sets)

## Running the Tests

### Unit Tests (Fast)
```bash
go test -v ./test/integration -run TestVerifyFlag
```

### Engine Tests (Requires Redis/Valkey)
```bash
go test -v ./test/integration -run TestVerifyTestSuite
```

### CLI Tests (Requires Docker)
```bash
go test -v ./test/integration -run TestCLIVerifyTestSuite
```

### All Verification Tests
```bash
go test -v ./test/integration -run "TestVerify|TestCLIVerify"
```

## Test Dependencies

- **Unit Tests**: No external dependencies
- **Engine Tests**: Requires Redis and Valkey instances (uses environment variables for connection)
- **CLI Tests**: Requires Docker and Docker Compose for container orchestration

## Environment Variables

For engine tests, you can configure:
- `REDIS_HOST` (default: 127.0.0.1)
- `REDIS_PORT` (default: 16379)
- `VALKEY_HOST` (default: 127.0.0.1)
- `VALKEY_PORT` (default: 16380)
- `REDIS_PASSWORD` (optional)
- `VALKEY_PASSWORD` (optional)

## Test Data

The tests use various test datasets:
- **Basic Data**: Simple examples of each Redis data type
- **Large Dataset**: 200-500 keys for performance testing
- **Complex Data**: Large hashes, lists, sets, and sorted sets with many elements
- **Corrupted Data**: Intentionally modified data to test verification failure detection