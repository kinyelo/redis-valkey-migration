package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete configuration for the migration tool
type Config struct {
	Redis     DatabaseConfig  `mapstructure:"redis"`
	Valkey    DatabaseConfig  `mapstructure:"valkey"`
	Migration MigrationConfig `mapstructure:"migration"`
}

// DatabaseConfig holds connection parameters for Redis or Valkey
type DatabaseConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	Password          string        `mapstructure:"password"`
	Database          int           `mapstructure:"database"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	OperationTimeout  time.Duration `mapstructure:"operation_timeout"`
	LargeDataTimeout  time.Duration `mapstructure:"large_data_timeout"`
}

// MigrationConfig holds migration-specific settings
type MigrationConfig struct {
	BatchSize          int           `mapstructure:"batch_size"`
	RetryAttempts      int           `mapstructure:"retry_attempts"`
	LogLevel           string        `mapstructure:"log_level"`
	TimeoutConfig      TimeoutConfig `mapstructure:"timeout_config"`
	CollectionPatterns []string      `mapstructure:"collection_patterns"`
}

// TimeoutConfig holds operation-specific timeout settings
type TimeoutConfig struct {
	ConnectionTimeout   time.Duration `mapstructure:"connection_timeout"`
	DefaultOperation    time.Duration `mapstructure:"default_operation"`
	StringOperation     time.Duration `mapstructure:"string_operation"`
	HashOperation       time.Duration `mapstructure:"hash_operation"`
	ListOperation       time.Duration `mapstructure:"list_operation"`
	SetOperation        time.Duration `mapstructure:"set_operation"`
	SortedSetOperation  time.Duration `mapstructure:"sorted_set_operation"`
	LargeDataThreshold  int64         `mapstructure:"large_data_threshold"`
	LargeDataMultiplier float64       `mapstructure:"large_data_multiplier"`
}

// LoadConfig loads configuration from environment variables and command-line flags
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.redis-valkey-migration")
	viper.AddConfigPath("/etc/redis-valkey-migration/")

	// Set default values
	setDefaults()

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("RVM") // Redis Valkey Migration
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file if it exists (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Override with environment variables
	bindEnvVars()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.connection_timeout", "30s")
	viper.SetDefault("redis.operation_timeout", "10s")
	viper.SetDefault("redis.large_data_timeout", "60s")

	// Valkey defaults
	viper.SetDefault("valkey.host", "localhost")
	viper.SetDefault("valkey.port", 6380)
	viper.SetDefault("valkey.password", "")
	viper.SetDefault("valkey.database", 0)
	viper.SetDefault("valkey.connection_timeout", "30s")
	viper.SetDefault("valkey.operation_timeout", "10s")
	viper.SetDefault("valkey.large_data_timeout", "60s")

	// Migration defaults
	viper.SetDefault("migration.batch_size", 1000)
	viper.SetDefault("migration.retry_attempts", 3)
	viper.SetDefault("migration.log_level", "info")

	// Timeout configuration defaults
	viper.SetDefault("migration.timeout_config.connection_timeout", "30s")
	viper.SetDefault("migration.timeout_config.default_operation", "10s")
	viper.SetDefault("migration.timeout_config.string_operation", "5s")
	viper.SetDefault("migration.timeout_config.hash_operation", "15s")
	viper.SetDefault("migration.timeout_config.list_operation", "15s")
	viper.SetDefault("migration.timeout_config.set_operation", "15s")
	viper.SetDefault("migration.timeout_config.sorted_set_operation", "20s")
	viper.SetDefault("migration.timeout_config.large_data_threshold", 10000)
	viper.SetDefault("migration.timeout_config.large_data_multiplier", 2.0)
}

// bindEnvVars binds environment variables to configuration keys
func bindEnvVars() {
	// Redis environment variables
	viper.BindEnv("redis.host", "RVM_REDIS_HOST")
	viper.BindEnv("redis.port", "RVM_REDIS_PORT")
	viper.BindEnv("redis.password", "RVM_REDIS_PASSWORD")
	viper.BindEnv("redis.database", "RVM_REDIS_DATABASE")
	viper.BindEnv("redis.connection_timeout", "RVM_REDIS_CONNECTION_TIMEOUT")
	viper.BindEnv("redis.operation_timeout", "RVM_REDIS_OPERATION_TIMEOUT")
	viper.BindEnv("redis.large_data_timeout", "RVM_REDIS_LARGE_DATA_TIMEOUT")

	// Valkey environment variables
	viper.BindEnv("valkey.host", "RVM_VALKEY_HOST")
	viper.BindEnv("valkey.port", "RVM_VALKEY_PORT")
	viper.BindEnv("valkey.password", "RVM_VALKEY_PASSWORD")
	viper.BindEnv("valkey.database", "RVM_VALKEY_DATABASE")
	viper.BindEnv("valkey.connection_timeout", "RVM_VALKEY_CONNECTION_TIMEOUT")
	viper.BindEnv("valkey.operation_timeout", "RVM_VALKEY_OPERATION_TIMEOUT")
	viper.BindEnv("valkey.large_data_timeout", "RVM_VALKEY_LARGE_DATA_TIMEOUT")

	// Migration environment variables
	viper.BindEnv("migration.batch_size", "RVM_MIGRATION_BATCH_SIZE")
	viper.BindEnv("migration.retry_attempts", "RVM_MIGRATION_RETRY_ATTEMPTS")
	viper.BindEnv("migration.log_level", "RVM_MIGRATION_LOG_LEVEL")
	viper.BindEnv("migration.collection_patterns", "RVM_MIGRATION_COLLECTION_PATTERNS")

	// Timeout configuration environment variables
	viper.BindEnv("migration.timeout_config.connection_timeout", "RVM_TIMEOUT_CONNECTION")
	viper.BindEnv("migration.timeout_config.default_operation", "RVM_TIMEOUT_DEFAULT_OPERATION")
	viper.BindEnv("migration.timeout_config.string_operation", "RVM_TIMEOUT_STRING_OPERATION")
	viper.BindEnv("migration.timeout_config.hash_operation", "RVM_TIMEOUT_HASH_OPERATION")
	viper.BindEnv("migration.timeout_config.list_operation", "RVM_TIMEOUT_LIST_OPERATION")
	viper.BindEnv("migration.timeout_config.set_operation", "RVM_TIMEOUT_SET_OPERATION")
	viper.BindEnv("migration.timeout_config.sorted_set_operation", "RVM_TIMEOUT_SORTED_SET_OPERATION")
	viper.BindEnv("migration.timeout_config.large_data_threshold", "RVM_TIMEOUT_LARGE_DATA_THRESHOLD")
	viper.BindEnv("migration.timeout_config.large_data_multiplier", "RVM_TIMEOUT_LARGE_DATA_MULTIPLIER")
}

// ValidateConfig validates the configuration parameters
func ValidateConfig(config *Config) error {
	if err := validateDatabaseConfig("Redis", &config.Redis); err != nil {
		return err
	}

	if err := validateDatabaseConfig("Valkey", &config.Valkey); err != nil {
		return err
	}

	if err := validateMigrationConfig(&config.Migration); err != nil {
		return err
	}

	if err := validateTimeoutConfig(&config.Migration.TimeoutConfig); err != nil {
		return err
	}

	if err := validateCollectionPatterns(config.Migration.CollectionPatterns); err != nil {
		return err
	}

	return nil
}

// validateDatabaseConfig validates database connection parameters
func validateDatabaseConfig(name string, dbConfig *DatabaseConfig) error {
	if dbConfig.Host == "" {
		return fmt.Errorf("%s host cannot be empty", name)
	}

	if dbConfig.Port <= 0 || dbConfig.Port > 65535 {
		return fmt.Errorf("%s port must be between 1 and 65535, got %d", name, dbConfig.Port)
	}

	if dbConfig.Database < 0 || dbConfig.Database > 15 {
		return fmt.Errorf("%s database must be between 0 and 15, got %d", name, dbConfig.Database)
	}

	if dbConfig.ConnectionTimeout <= 0 {
		return fmt.Errorf("%s connection timeout must be positive, got %v", name, dbConfig.ConnectionTimeout)
	}

	if dbConfig.OperationTimeout <= 0 {
		return fmt.Errorf("%s operation timeout must be positive, got %v", name, dbConfig.OperationTimeout)
	}

	if dbConfig.LargeDataTimeout <= 0 {
		return fmt.Errorf("%s large data timeout must be positive, got %v", name, dbConfig.LargeDataTimeout)
	}

	return nil
}

// validateMigrationConfig validates migration-specific parameters
func validateMigrationConfig(migConfig *MigrationConfig) error {
	if migConfig.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0, got %d", migConfig.BatchSize)
	}

	if migConfig.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts must be non-negative, got %d", migConfig.RetryAttempts)
	}

	validLogLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"panic": true,
	}

	if !validLogLevels[strings.ToLower(migConfig.LogLevel)] {
		return fmt.Errorf("invalid log level '%s', must be one of: trace, debug, info, warn, error, fatal, panic", migConfig.LogLevel)
	}

	return nil
}

// validateTimeoutConfig validates timeout configuration parameters
func validateTimeoutConfig(timeoutConfig *TimeoutConfig) error {
	if timeoutConfig.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive, got %v", timeoutConfig.ConnectionTimeout)
	}

	if timeoutConfig.DefaultOperation <= 0 {
		return fmt.Errorf("default operation timeout must be positive, got %v", timeoutConfig.DefaultOperation)
	}

	if timeoutConfig.StringOperation <= 0 {
		return fmt.Errorf("string operation timeout must be positive, got %v", timeoutConfig.StringOperation)
	}

	if timeoutConfig.HashOperation <= 0 {
		return fmt.Errorf("hash operation timeout must be positive, got %v", timeoutConfig.HashOperation)
	}

	if timeoutConfig.ListOperation <= 0 {
		return fmt.Errorf("list operation timeout must be positive, got %v", timeoutConfig.ListOperation)
	}

	if timeoutConfig.SetOperation <= 0 {
		return fmt.Errorf("set operation timeout must be positive, got %v", timeoutConfig.SetOperation)
	}

	if timeoutConfig.SortedSetOperation <= 0 {
		return fmt.Errorf("sorted set operation timeout must be positive, got %v", timeoutConfig.SortedSetOperation)
	}

	if timeoutConfig.LargeDataThreshold <= 0 {
		return fmt.Errorf("large data threshold must be positive, got %d", timeoutConfig.LargeDataThreshold)
	}

	if timeoutConfig.LargeDataMultiplier <= 1.0 {
		return fmt.Errorf("large data multiplier must be greater than 1.0, got %f", timeoutConfig.LargeDataMultiplier)
	}

	// Reasonable upper bounds for timeouts (10 minutes)
	maxTimeout := 10 * time.Minute
	if timeoutConfig.ConnectionTimeout > maxTimeout {
		return fmt.Errorf("connection timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.ConnectionTimeout)
	}

	if timeoutConfig.DefaultOperation > maxTimeout {
		return fmt.Errorf("default operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.DefaultOperation)
	}

	if timeoutConfig.StringOperation > maxTimeout {
		return fmt.Errorf("string operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.StringOperation)
	}

	if timeoutConfig.HashOperation > maxTimeout {
		return fmt.Errorf("hash operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.HashOperation)
	}

	if timeoutConfig.ListOperation > maxTimeout {
		return fmt.Errorf("list operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.ListOperation)
	}

	if timeoutConfig.SetOperation > maxTimeout {
		return fmt.Errorf("set operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.SetOperation)
	}

	if timeoutConfig.SortedSetOperation > maxTimeout {
		return fmt.Errorf("sorted set operation timeout too large, maximum is %v, got %v", maxTimeout, timeoutConfig.SortedSetOperation)
	}

	if timeoutConfig.LargeDataMultiplier > 10.0 {
		return fmt.Errorf("large data multiplier too large, maximum is 10.0, got %f", timeoutConfig.LargeDataMultiplier)
	}

	return nil
}

// validateCollectionPatterns validates collection pattern syntax
func validateCollectionPatterns(patterns []string) error {
	if len(patterns) == 0 {
		return nil // Empty patterns are valid (means migrate all keys)
	}

	for i, pattern := range patterns {
		if pattern == "" {
			return fmt.Errorf("collection pattern %d cannot be empty", i+1)
		}

		// Basic validation - check for obviously invalid patterns
		if strings.Contains(pattern, "**") {
			return fmt.Errorf("collection pattern %d contains invalid '**' sequence: %s", i+1, pattern)
		}

		// Test the pattern with filepath.Match to ensure it's valid
		_, err := filepath.Match(pattern, "test")
		if err != nil {
			return fmt.Errorf("collection pattern %d is invalid: %s - %w", i+1, pattern, err)
		}
	}

	return nil
}

// LoadConfigFromEnv loads configuration from environment variables only
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:              getEnvString("RVM_REDIS_HOST", "localhost"),
			Port:              getEnvInt("RVM_REDIS_PORT", 6379),
			Password:          getEnvString("RVM_REDIS_PASSWORD", ""),
			Database:          getEnvInt("RVM_REDIS_DATABASE", 0),
			ConnectionTimeout: getEnvDuration("RVM_REDIS_CONNECTION_TIMEOUT", 30*time.Second),
			OperationTimeout:  getEnvDuration("RVM_REDIS_OPERATION_TIMEOUT", 10*time.Second),
			LargeDataTimeout:  getEnvDuration("RVM_REDIS_LARGE_DATA_TIMEOUT", 60*time.Second),
		},
		Valkey: DatabaseConfig{
			Host:              getEnvString("RVM_VALKEY_HOST", "localhost"),
			Port:              getEnvInt("RVM_VALKEY_PORT", 6380),
			Password:          getEnvString("RVM_VALKEY_PASSWORD", ""),
			Database:          getEnvInt("RVM_VALKEY_DATABASE", 0),
			ConnectionTimeout: getEnvDuration("RVM_VALKEY_CONNECTION_TIMEOUT", 30*time.Second),
			OperationTimeout:  getEnvDuration("RVM_VALKEY_OPERATION_TIMEOUT", 10*time.Second),
			LargeDataTimeout:  getEnvDuration("RVM_VALKEY_LARGE_DATA_TIMEOUT", 60*time.Second),
		},
		Migration: MigrationConfig{
			BatchSize:          getEnvInt("RVM_MIGRATION_BATCH_SIZE", 1000),
			RetryAttempts:      getEnvInt("RVM_MIGRATION_RETRY_ATTEMPTS", 3),
			LogLevel:           getEnvString("RVM_MIGRATION_LOG_LEVEL", "info"),
			CollectionPatterns: getEnvStringSlice("RVM_MIGRATION_COLLECTION_PATTERNS", []string{}),
			TimeoutConfig: TimeoutConfig{
				ConnectionTimeout:   getEnvDuration("RVM_TIMEOUT_CONNECTION", 30*time.Second),
				DefaultOperation:    getEnvDuration("RVM_TIMEOUT_DEFAULT_OPERATION", 10*time.Second),
				StringOperation:     getEnvDuration("RVM_TIMEOUT_STRING_OPERATION", 5*time.Second),
				HashOperation:       getEnvDuration("RVM_TIMEOUT_HASH_OPERATION", 15*time.Second),
				ListOperation:       getEnvDuration("RVM_TIMEOUT_LIST_OPERATION", 15*time.Second),
				SetOperation:        getEnvDuration("RVM_TIMEOUT_SET_OPERATION", 15*time.Second),
				SortedSetOperation:  getEnvDuration("RVM_TIMEOUT_SORTED_SET_OPERATION", 20*time.Second),
				LargeDataThreshold:  getEnvInt64("RVM_TIMEOUT_LARGE_DATA_THRESHOLD", 10000),
				LargeDataMultiplier: getEnvFloat64("RVM_TIMEOUT_LARGE_DATA_MULTIPLIER", 2.0),
			},
		},
	}

	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// getEnvString gets a string environment variable with a default value
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvInt64 gets an int64 environment variable with a default value
func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat64 gets a float64 environment variable with a default value
func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getEnvDuration gets a duration environment variable with a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvStringSlice gets a string slice environment variable with a default value
// Expects comma-separated values in the environment variable
func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	}
	return defaultValue
}
