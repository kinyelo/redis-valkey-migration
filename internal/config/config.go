package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

// MigrationConfig holds migration-specific settings
type MigrationConfig struct {
	BatchSize     int    `mapstructure:"batch_size"`
	RetryAttempts int    `mapstructure:"retry_attempts"`
	LogLevel      string `mapstructure:"log_level"`
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

	// Valkey defaults
	viper.SetDefault("valkey.host", "localhost")
	viper.SetDefault("valkey.port", 6380)
	viper.SetDefault("valkey.password", "")
	viper.SetDefault("valkey.database", 0)

	// Migration defaults
	viper.SetDefault("migration.batch_size", 1000)
	viper.SetDefault("migration.retry_attempts", 3)
	viper.SetDefault("migration.log_level", "info")
}

// bindEnvVars binds environment variables to configuration keys
func bindEnvVars() {
	// Redis environment variables
	viper.BindEnv("redis.host", "RVM_REDIS_HOST")
	viper.BindEnv("redis.port", "RVM_REDIS_PORT")
	viper.BindEnv("redis.password", "RVM_REDIS_PASSWORD")
	viper.BindEnv("redis.database", "RVM_REDIS_DATABASE")

	// Valkey environment variables
	viper.BindEnv("valkey.host", "RVM_VALKEY_HOST")
	viper.BindEnv("valkey.port", "RVM_VALKEY_PORT")
	viper.BindEnv("valkey.password", "RVM_VALKEY_PASSWORD")
	viper.BindEnv("valkey.database", "RVM_VALKEY_DATABASE")

	// Migration environment variables
	viper.BindEnv("migration.batch_size", "RVM_MIGRATION_BATCH_SIZE")
	viper.BindEnv("migration.retry_attempts", "RVM_MIGRATION_RETRY_ATTEMPTS")
	viper.BindEnv("migration.log_level", "RVM_MIGRATION_LOG_LEVEL")
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

// LoadConfigFromEnv loads configuration from environment variables only
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{
		Redis: DatabaseConfig{
			Host:     getEnvString("RVM_REDIS_HOST", "localhost"),
			Port:     getEnvInt("RVM_REDIS_PORT", 6379),
			Password: getEnvString("RVM_REDIS_PASSWORD", ""),
			Database: getEnvInt("RVM_REDIS_DATABASE", 0),
		},
		Valkey: DatabaseConfig{
			Host:     getEnvString("RVM_VALKEY_HOST", "localhost"),
			Port:     getEnvInt("RVM_VALKEY_PORT", 6380),
			Password: getEnvString("RVM_VALKEY_PASSWORD", ""),
			Database: getEnvInt("RVM_VALKEY_DATABASE", 0),
		},
		Migration: MigrationConfig{
			BatchSize:     getEnvInt("RVM_MIGRATION_BATCH_SIZE", 1000),
			RetryAttempts: getEnvInt("RVM_MIGRATION_RETRY_ATTEMPTS", 3),
			LogLevel:      getEnvString("RVM_MIGRATION_LOG_LEVEL", "info"),
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
