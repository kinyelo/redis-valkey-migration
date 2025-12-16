package config

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createValidConfig creates a valid configuration for testing
func createValidConfig() *Config {
	return &Config{
		Redis: DatabaseConfig{
			Host:              "localhost",
			Port:              6379,
			Password:          "test-password",
			Database:          0,
			ConnectionTimeout: 30 * time.Second,
			OperationTimeout:  10 * time.Second,
			LargeDataTimeout:  60 * time.Second,
		},
		Valkey: DatabaseConfig{
			Host:              "valkey-host",
			Port:              6380,
			Password:          "valkey-password",
			Database:          1,
			ConnectionTimeout: 30 * time.Second,
			OperationTimeout:  10 * time.Second,
			LargeDataTimeout:  60 * time.Second,
		},
		Migration: MigrationConfig{
			BatchSize:          1000,
			RetryAttempts:      3,
			LogLevel:           "info",
			CollectionPatterns: []string{},
			TimeoutConfig: TimeoutConfig{
				ConnectionTimeout:   30 * time.Second,
				DefaultOperation:    10 * time.Second,
				StringOperation:     5 * time.Second,
				HashOperation:       15 * time.Second,
				ListOperation:       15 * time.Second,
				SetOperation:        15 * time.Second,
				SortedSetOperation:  20 * time.Second,
				LargeDataThreshold:  10000,
				LargeDataMultiplier: 2.0,
			},
		},
	}
}

func TestValidateConfig_ValidConfiguration(t *testing.T) {
	config := createValidConfig()
	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidRedisHost(t *testing.T) {
	config := createValidConfig()
	config.Redis.Host = "" // Invalid empty host

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis host cannot be empty")
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"negative port", -1},
		{"zero port", 0},
		{"port too high", 70000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := createValidConfig()
			config.Redis.Port = tc.port

			err := ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "port must be between 1 and 65535")
		})
	}
}

func TestValidateConfig_InvalidDatabase(t *testing.T) {
	testCases := []struct {
		name     string
		database int
	}{
		{"negative database", -1},
		{"database too high", 16},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := createValidConfig()
			config.Redis.Database = tc.database

			err := ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "database must be between 0 and 15")
		})
	}
}

func TestValidateConfig_InvalidBatchSize(t *testing.T) {
	config := createValidConfig()
	config.Migration.BatchSize = 0 // Invalid batch size

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch size must be greater than 0")
}

func TestValidateConfig_InvalidRetryAttempts(t *testing.T) {
	config := createValidConfig()
	config.Migration.RetryAttempts = -1 // Invalid retry attempts

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry attempts must be non-negative")
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	config := createValidConfig()
	config.Migration.LogLevel = "invalid" // Invalid log level

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestLoadConfigFromEnv_WithDefaults(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	config, err := LoadConfigFromEnv()
	require.NoError(t, err)

	// Check default values
	assert.Equal(t, "localhost", config.Redis.Host)
	assert.Equal(t, 6379, config.Redis.Port)
	assert.Equal(t, "", config.Redis.Password)
	assert.Equal(t, 0, config.Redis.Database)

	assert.Equal(t, "localhost", config.Valkey.Host)
	assert.Equal(t, 6380, config.Valkey.Port)
	assert.Equal(t, "", config.Valkey.Password)
	assert.Equal(t, 0, config.Valkey.Database)

	assert.Equal(t, 1000, config.Migration.BatchSize)
	assert.Equal(t, 3, config.Migration.RetryAttempts)
	assert.Equal(t, "info", config.Migration.LogLevel)

	// Check timeout defaults
	assert.Equal(t, 30*time.Second, config.Redis.ConnectionTimeout)
	assert.Equal(t, 10*time.Second, config.Redis.OperationTimeout)
	assert.Equal(t, 60*time.Second, config.Redis.LargeDataTimeout)

	assert.Equal(t, 30*time.Second, config.Valkey.ConnectionTimeout)
	assert.Equal(t, 10*time.Second, config.Valkey.OperationTimeout)
	assert.Equal(t, 60*time.Second, config.Valkey.LargeDataTimeout)

	assert.Equal(t, 30*time.Second, config.Migration.TimeoutConfig.ConnectionTimeout)
	assert.Equal(t, 10*time.Second, config.Migration.TimeoutConfig.DefaultOperation)
	assert.Equal(t, 5*time.Second, config.Migration.TimeoutConfig.StringOperation)
	assert.Equal(t, 15*time.Second, config.Migration.TimeoutConfig.HashOperation)
	assert.Equal(t, 15*time.Second, config.Migration.TimeoutConfig.ListOperation)
	assert.Equal(t, 15*time.Second, config.Migration.TimeoutConfig.SetOperation)
	assert.Equal(t, 20*time.Second, config.Migration.TimeoutConfig.SortedSetOperation)
	assert.Equal(t, int64(10000), config.Migration.TimeoutConfig.LargeDataThreshold)
	assert.Equal(t, 2.0, config.Migration.TimeoutConfig.LargeDataMultiplier)
}

func TestLoadConfigFromEnv_WithEnvironmentVariables(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set environment variables
	os.Setenv("RVM_REDIS_HOST", "redis-server")
	os.Setenv("RVM_REDIS_PORT", "6380")
	os.Setenv("RVM_REDIS_PASSWORD", "redis-pass")
	os.Setenv("RVM_REDIS_DATABASE", "2")

	os.Setenv("RVM_VALKEY_HOST", "valkey-server")
	os.Setenv("RVM_VALKEY_PORT", "6381")
	os.Setenv("RVM_VALKEY_PASSWORD", "valkey-pass")
	os.Setenv("RVM_VALKEY_DATABASE", "3")

	os.Setenv("RVM_MIGRATION_BATCH_SIZE", "2000")
	os.Setenv("RVM_MIGRATION_RETRY_ATTEMPTS", "5")
	os.Setenv("RVM_MIGRATION_LOG_LEVEL", "debug")

	defer clearEnvVars()

	config, err := LoadConfigFromEnv()
	require.NoError(t, err)

	// Check environment variable values
	assert.Equal(t, "redis-server", config.Redis.Host)
	assert.Equal(t, 6380, config.Redis.Port)
	assert.Equal(t, "redis-pass", config.Redis.Password)
	assert.Equal(t, 2, config.Redis.Database)

	assert.Equal(t, "valkey-server", config.Valkey.Host)
	assert.Equal(t, 6381, config.Valkey.Port)
	assert.Equal(t, "valkey-pass", config.Valkey.Password)
	assert.Equal(t, 3, config.Valkey.Database)

	assert.Equal(t, 2000, config.Migration.BatchSize)
	assert.Equal(t, 5, config.Migration.RetryAttempts)
	assert.Equal(t, "debug", config.Migration.LogLevel)
}

func TestLoadConfigFromEnv_InvalidEnvironmentVariables(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set invalid environment variables
	os.Setenv("RVM_REDIS_PORT", "invalid-port")
	os.Setenv("RVM_MIGRATION_BATCH_SIZE", "invalid-batch-size")

	defer clearEnvVars()

	config, err := LoadConfigFromEnv()
	require.NoError(t, err)

	// Should fall back to defaults for invalid values
	assert.Equal(t, 6379, config.Redis.Port)          // Default port
	assert.Equal(t, 1000, config.Migration.BatchSize) // Default batch size
}

func TestGetEnvString(t *testing.T) {
	os.Setenv("TEST_STRING", "test-value")
	defer os.Unsetenv("TEST_STRING")

	result := getEnvString("TEST_STRING", "default")
	assert.Equal(t, "test-value", result)

	result = getEnvString("NON_EXISTENT", "default")
	assert.Equal(t, "default", result)
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	result := getEnvInt("TEST_INT", 10)
	assert.Equal(t, 42, result)

	result = getEnvInt("NON_EXISTENT", 10)
	assert.Equal(t, 10, result)

	// Test invalid integer
	os.Setenv("TEST_INVALID_INT", "not-a-number")
	defer os.Unsetenv("TEST_INVALID_INT")

	result = getEnvInt("TEST_INVALID_INT", 10)
	assert.Equal(t, 10, result) // Should return default for invalid integer
}

// clearEnvVars clears all environment variables used by the configuration
func clearEnvVars() {
	envVars := []string{
		"RVM_REDIS_HOST", "RVM_REDIS_PORT", "RVM_REDIS_PASSWORD", "RVM_REDIS_DATABASE",
		"RVM_REDIS_CONNECTION_TIMEOUT", "RVM_REDIS_OPERATION_TIMEOUT", "RVM_REDIS_LARGE_DATA_TIMEOUT",
		"RVM_VALKEY_HOST", "RVM_VALKEY_PORT", "RVM_VALKEY_PASSWORD", "RVM_VALKEY_DATABASE",
		"RVM_VALKEY_CONNECTION_TIMEOUT", "RVM_VALKEY_OPERATION_TIMEOUT", "RVM_VALKEY_LARGE_DATA_TIMEOUT",
		"RVM_MIGRATION_BATCH_SIZE", "RVM_MIGRATION_RETRY_ATTEMPTS", "RVM_MIGRATION_LOG_LEVEL",
		"RVM_TIMEOUT_CONNECTION", "RVM_TIMEOUT_DEFAULT_OPERATION", "RVM_TIMEOUT_STRING_OPERATION",
		"RVM_TIMEOUT_HASH_OPERATION", "RVM_TIMEOUT_LIST_OPERATION", "RVM_TIMEOUT_SET_OPERATION",
		"RVM_TIMEOUT_SORTED_SET_OPERATION", "RVM_TIMEOUT_LARGE_DATA_THRESHOLD", "RVM_TIMEOUT_LARGE_DATA_MULTIPLIER",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
func TestValidateTimeoutConfig_ValidConfiguration(t *testing.T) {
	timeoutConfig := &TimeoutConfig{
		ConnectionTimeout:   30 * time.Second,
		DefaultOperation:    10 * time.Second,
		StringOperation:     5 * time.Second,
		HashOperation:       15 * time.Second,
		ListOperation:       15 * time.Second,
		SetOperation:        15 * time.Second,
		SortedSetOperation:  20 * time.Second,
		LargeDataThreshold:  10000,
		LargeDataMultiplier: 2.0,
	}

	err := validateTimeoutConfig(timeoutConfig)
	assert.NoError(t, err)
}

func TestValidateTimeoutConfig_InvalidTimeouts(t *testing.T) {
	testCases := []struct {
		name          string
		modifyConfig  func(*TimeoutConfig)
		expectedError string
	}{
		{
			name: "negative connection timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.ConnectionTimeout = -1 * time.Second
			},
			expectedError: "connection timeout must be positive",
		},
		{
			name: "zero default operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.DefaultOperation = 0
			},
			expectedError: "default operation timeout must be positive",
		},
		{
			name: "negative string operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.StringOperation = -5 * time.Second
			},
			expectedError: "string operation timeout must be positive",
		},
		{
			name: "zero hash operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.HashOperation = 0
			},
			expectedError: "hash operation timeout must be positive",
		},
		{
			name: "negative list operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.ListOperation = -10 * time.Second
			},
			expectedError: "list operation timeout must be positive",
		},
		{
			name: "zero set operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.SetOperation = 0
			},
			expectedError: "set operation timeout must be positive",
		},
		{
			name: "negative sorted set operation timeout",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.SortedSetOperation = -20 * time.Second
			},
			expectedError: "sorted set operation timeout must be positive",
		},
		{
			name: "zero large data threshold",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.LargeDataThreshold = 0
			},
			expectedError: "large data threshold must be positive",
		},
		{
			name: "negative large data threshold",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.LargeDataThreshold = -1000
			},
			expectedError: "large data threshold must be positive",
		},
		{
			name: "multiplier too small",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.LargeDataMultiplier = 1.0
			},
			expectedError: "large data multiplier must be greater than 1.0",
		},
		{
			name: "multiplier negative",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.LargeDataMultiplier = -2.0
			},
			expectedError: "large data multiplier must be greater than 1.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeoutConfig := &TimeoutConfig{
				ConnectionTimeout:   30 * time.Second,
				DefaultOperation:    10 * time.Second,
				StringOperation:     5 * time.Second,
				HashOperation:       15 * time.Second,
				ListOperation:       15 * time.Second,
				SetOperation:        15 * time.Second,
				SortedSetOperation:  20 * time.Second,
				LargeDataThreshold:  10000,
				LargeDataMultiplier: 2.0,
			}

			tc.modifyConfig(timeoutConfig)

			err := validateTimeoutConfig(timeoutConfig)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestValidateTimeoutConfig_TimeoutTooLarge(t *testing.T) {
	testCases := []struct {
		name          string
		modifyConfig  func(*TimeoutConfig)
		expectedError string
	}{
		{
			name: "connection timeout too large",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.ConnectionTimeout = 15 * time.Minute
			},
			expectedError: "connection timeout too large",
		},
		{
			name: "default operation timeout too large",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.DefaultOperation = 12 * time.Minute
			},
			expectedError: "default operation timeout too large",
		},
		{
			name: "string operation timeout too large",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.StringOperation = 11 * time.Minute
			},
			expectedError: "string operation timeout too large",
		},
		{
			name: "multiplier too large",
			modifyConfig: func(tc *TimeoutConfig) {
				tc.LargeDataMultiplier = 15.0
			},
			expectedError: "large data multiplier too large",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeoutConfig := &TimeoutConfig{
				ConnectionTimeout:   30 * time.Second,
				DefaultOperation:    10 * time.Second,
				StringOperation:     5 * time.Second,
				HashOperation:       15 * time.Second,
				ListOperation:       15 * time.Second,
				SetOperation:        15 * time.Second,
				SortedSetOperation:  20 * time.Second,
				LargeDataThreshold:  10000,
				LargeDataMultiplier: 2.0,
			}

			tc.modifyConfig(timeoutConfig)

			err := validateTimeoutConfig(timeoutConfig)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestValidateConfig_WithTimeoutValidation(t *testing.T) {
	config := createValidConfig()
	config.Migration.TimeoutConfig.ConnectionTimeout = -1 * time.Second // Invalid

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection timeout must be positive")
}

func TestLoadConfigFromEnv_WithTimeoutEnvironmentVariables(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set timeout-related environment variables
	os.Setenv("RVM_TIMEOUT_CONNECTION", "45s")
	os.Setenv("RVM_TIMEOUT_DEFAULT_OPERATION", "12s")
	os.Setenv("RVM_TIMEOUT_STRING_OPERATION", "6s")
	os.Setenv("RVM_TIMEOUT_HASH_OPERATION", "18s")
	os.Setenv("RVM_TIMEOUT_LIST_OPERATION", "18s")
	os.Setenv("RVM_TIMEOUT_SET_OPERATION", "18s")
	os.Setenv("RVM_TIMEOUT_SORTED_SET_OPERATION", "25s")
	os.Setenv("RVM_TIMEOUT_LARGE_DATA_THRESHOLD", "15000")
	os.Setenv("RVM_TIMEOUT_LARGE_DATA_MULTIPLIER", "2.5")

	defer clearEnvVars()

	config, err := LoadConfigFromEnv()
	require.NoError(t, err)

	// Check timeout configuration values
	assert.Equal(t, 45*time.Second, config.Migration.TimeoutConfig.ConnectionTimeout)
	assert.Equal(t, 12*time.Second, config.Migration.TimeoutConfig.DefaultOperation)
	assert.Equal(t, 6*time.Second, config.Migration.TimeoutConfig.StringOperation)
	assert.Equal(t, 18*time.Second, config.Migration.TimeoutConfig.HashOperation)
	assert.Equal(t, 18*time.Second, config.Migration.TimeoutConfig.ListOperation)
	assert.Equal(t, 18*time.Second, config.Migration.TimeoutConfig.SetOperation)
	assert.Equal(t, 25*time.Second, config.Migration.TimeoutConfig.SortedSetOperation)
	assert.Equal(t, int64(15000), config.Migration.TimeoutConfig.LargeDataThreshold)
	assert.Equal(t, 2.5, config.Migration.TimeoutConfig.LargeDataMultiplier)
}

// TestValidateCollectionPatterns tests collection pattern validation
func TestValidateCollectionPatterns_ValidPatterns(t *testing.T) {
	validPatterns := [][]string{
		{},                              // Empty patterns (valid)
		{"user:*"},                      // Single pattern
		{"user:*", "session:*"},         // Multiple patterns
		{"cache:data:*", "temp_*"},      // Mixed patterns
		{"*:profile", "admin:*:config"}, // Complex patterns
	}

	for i, patterns := range validPatterns {
		t.Run(fmt.Sprintf("valid_patterns_%d", i), func(t *testing.T) {
			err := validateCollectionPatterns(patterns)
			assert.NoError(t, err)
		})
	}
}

func TestValidateCollectionPatterns_InvalidPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		wantErr  string
	}{
		{
			name:     "empty_pattern",
			patterns: []string{""},
			wantErr:  "collection pattern 1 cannot be empty",
		},
		{
			name:     "invalid_double_star",
			patterns: []string{"user:**"},
			wantErr:  "collection pattern 1 contains invalid '**' sequence",
		},
		{
			name:     "invalid_bracket_pattern",
			patterns: []string{"user:["},
			wantErr:  "collection pattern 1 is invalid",
		},
		{
			name:     "multiple_with_one_invalid",
			patterns: []string{"user:*", ""},
			wantErr:  "collection pattern 2 cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCollectionPatterns(tc.patterns)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestValidateConfig_WithCollectionPatterns(t *testing.T) {
	config := createValidConfig()
	config.Migration.CollectionPatterns = []string{"user:*", "session:*"}

	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_WithInvalidCollectionPatterns(t *testing.T) {
	config := createValidConfig()
	config.Migration.CollectionPatterns = []string{""}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection pattern 1 cannot be empty")
}

func TestLoadConfigFromEnv_WithCollectionPatterns(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()
	defer clearEnvVars()

	// Set collection patterns environment variable
	os.Setenv("RVM_MIGRATION_COLLECTION_PATTERNS", "user:*,session:*,cache:*")

	config, err := LoadConfigFromEnv()
	require.NoError(t, err)

	expected := []string{"user:*", "session:*", "cache:*"}
	assert.Equal(t, expected, config.Migration.CollectionPatterns)
}
