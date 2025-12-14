package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_ValidConfiguration(t *testing.T) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "test-password",
			Database: 0,
		},
		Valkey: DatabaseConfig{
			Host:     "valkey-host",
			Port:     6380,
			Password: "valkey-password",
			Database: 1,
		},
		Migration: MigrationConfig{
			BatchSize:     1000,
			RetryAttempts: 3,
			LogLevel:      "info",
		},
	}

	err := ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidRedisHost(t *testing.T) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     "", // Invalid empty host
			Port:     6379,
			Password: "",
			Database: 0,
		},
		Valkey: DatabaseConfig{
			Host:     "localhost",
			Port:     6380,
			Password: "",
			Database: 0,
		},
		Migration: MigrationConfig{
			BatchSize:     1000,
			RetryAttempts: 3,
			LogLevel:      "info",
		},
	}

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
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     tc.port,
					Password: "",
					Database: 0,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     1000,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

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
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     6379,
					Password: "",
					Database: tc.database,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     1000,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

			err := ValidateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "database must be between 0 and 15")
		})
	}
}

func TestValidateConfig_InvalidBatchSize(t *testing.T) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			Database: 0,
		},
		Valkey: DatabaseConfig{
			Host:     "localhost",
			Port:     6380,
			Password: "",
			Database: 0,
		},
		Migration: MigrationConfig{
			BatchSize:     0, // Invalid batch size
			RetryAttempts: 3,
			LogLevel:      "info",
		},
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch size must be greater than 0")
}

func TestValidateConfig_InvalidRetryAttempts(t *testing.T) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			Database: 0,
		},
		Valkey: DatabaseConfig{
			Host:     "localhost",
			Port:     6380,
			Password: "",
			Database: 0,
		},
		Migration: MigrationConfig{
			BatchSize:     1000,
			RetryAttempts: -1, // Invalid retry attempts
			LogLevel:      "info",
		},
	}

	err := ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry attempts must be non-negative")
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			Database: 0,
		},
		Valkey: DatabaseConfig{
			Host:     "localhost",
			Port:     6380,
			Password: "",
			Database: 0,
		},
		Migration: MigrationConfig{
			BatchSize:     1000,
			RetryAttempts: 3,
			LogLevel:      "invalid", // Invalid log level
		},
	}

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
		"RVM_VALKEY_HOST", "RVM_VALKEY_PORT", "RVM_VALKEY_PASSWORD", "RVM_VALKEY_DATABASE",
		"RVM_MIGRATION_BATCH_SIZE", "RVM_MIGRATION_RETRY_ATTEMPTS", "RVM_MIGRATION_LOG_LEVEL",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
