package config

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BindFlags binds command-line flags to configuration keys
func BindFlags(cmd *cobra.Command) {
	// Redis connection flags
	cmd.Flags().String("redis-host", "localhost", "Redis server hostname or IP address")
	cmd.Flags().Int("redis-port", 6379, "Redis server port number")
	cmd.Flags().String("redis-password", "", "Redis authentication password (leave empty if no auth required)")
	cmd.Flags().Int("redis-database", 0, "Redis database number to migrate from (0-15)")
	cmd.Flags().Duration("redis-connection-timeout", 30*time.Second, "Redis connection timeout")
	cmd.Flags().Duration("redis-operation-timeout", 10*time.Second, "Redis operation timeout")
	cmd.Flags().Duration("redis-large-data-timeout", 60*time.Second, "Redis large data operation timeout")

	// Valkey connection flags
	cmd.Flags().String("valkey-host", "localhost", "Valkey server hostname or IP address")
	cmd.Flags().Int("valkey-port", 6380, "Valkey server port number")
	cmd.Flags().String("valkey-password", "", "Valkey authentication password (leave empty if no auth required)")
	cmd.Flags().Int("valkey-database", 0, "Valkey database number to migrate to (0-15)")
	cmd.Flags().Duration("valkey-connection-timeout", 30*time.Second, "Valkey connection timeout")
	cmd.Flags().Duration("valkey-operation-timeout", 10*time.Second, "Valkey operation timeout")
	cmd.Flags().Duration("valkey-large-data-timeout", 60*time.Second, "Valkey large data operation timeout")

	// Migration behavior flags
	cmd.Flags().Int("batch-size", 1000, "Number of keys to process in each batch (higher values use more memory)")
	cmd.Flags().Int("retry-attempts", 3, "Number of retry attempts for failed operations before giving up")
	cmd.Flags().String("log-level", "info", "Logging level: trace, debug, info, warn, error, fatal, panic")

	// Timeout configuration flags
	cmd.Flags().Duration("connection-timeout", 30*time.Second, "Default connection timeout for database operations")
	cmd.Flags().Duration("string-timeout", 5*time.Second, "Timeout for string operations")
	cmd.Flags().Duration("hash-timeout", 15*time.Second, "Timeout for hash operations")
	cmd.Flags().Duration("list-timeout", 15*time.Second, "Timeout for list operations")
	cmd.Flags().Duration("set-timeout", 15*time.Second, "Timeout for set operations")
	cmd.Flags().Duration("sorted-set-timeout", 20*time.Second, "Timeout for sorted set operations")
	cmd.Flags().Int64("large-data-threshold", 10000, "Threshold for considering data structures as large (number of elements)")
	cmd.Flags().Float64("large-data-multiplier", 2.0, "Multiplier for extending timeouts on large data structures")

	// Bind flags to viper
	viper.BindPFlag("redis.host", cmd.Flags().Lookup("redis-host"))
	viper.BindPFlag("redis.port", cmd.Flags().Lookup("redis-port"))
	viper.BindPFlag("redis.password", cmd.Flags().Lookup("redis-password"))
	viper.BindPFlag("redis.database", cmd.Flags().Lookup("redis-database"))
	viper.BindPFlag("redis.connection_timeout", cmd.Flags().Lookup("redis-connection-timeout"))
	viper.BindPFlag("redis.operation_timeout", cmd.Flags().Lookup("redis-operation-timeout"))
	viper.BindPFlag("redis.large_data_timeout", cmd.Flags().Lookup("redis-large-data-timeout"))

	viper.BindPFlag("valkey.host", cmd.Flags().Lookup("valkey-host"))
	viper.BindPFlag("valkey.port", cmd.Flags().Lookup("valkey-port"))
	viper.BindPFlag("valkey.password", cmd.Flags().Lookup("valkey-password"))
	viper.BindPFlag("valkey.database", cmd.Flags().Lookup("valkey-database"))
	viper.BindPFlag("valkey.connection_timeout", cmd.Flags().Lookup("valkey-connection-timeout"))
	viper.BindPFlag("valkey.operation_timeout", cmd.Flags().Lookup("valkey-operation-timeout"))
	viper.BindPFlag("valkey.large_data_timeout", cmd.Flags().Lookup("valkey-large-data-timeout"))

	viper.BindPFlag("migration.batch_size", cmd.Flags().Lookup("batch-size"))
	viper.BindPFlag("migration.retry_attempts", cmd.Flags().Lookup("retry-attempts"))
	viper.BindPFlag("migration.log_level", cmd.Flags().Lookup("log-level"))

	viper.BindPFlag("migration.timeout_config.connection_timeout", cmd.Flags().Lookup("connection-timeout"))
	viper.BindPFlag("migration.timeout_config.string_operation", cmd.Flags().Lookup("string-timeout"))
	viper.BindPFlag("migration.timeout_config.hash_operation", cmd.Flags().Lookup("hash-timeout"))
	viper.BindPFlag("migration.timeout_config.list_operation", cmd.Flags().Lookup("list-timeout"))
	viper.BindPFlag("migration.timeout_config.set_operation", cmd.Flags().Lookup("set-timeout"))
	viper.BindPFlag("migration.timeout_config.sorted_set_operation", cmd.Flags().Lookup("sorted-set-timeout"))
	viper.BindPFlag("migration.timeout_config.large_data_threshold", cmd.Flags().Lookup("large-data-threshold"))
	viper.BindPFlag("migration.timeout_config.large_data_multiplier", cmd.Flags().Lookup("large-data-multiplier"))
}

// LoadConfigWithFlags loads configuration with command-line flag support
func LoadConfigWithFlags() (*Config, error) {
	return LoadConfig()
}
