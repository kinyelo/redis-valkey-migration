package config

import (
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

	// Valkey connection flags
	cmd.Flags().String("valkey-host", "localhost", "Valkey server hostname or IP address")
	cmd.Flags().Int("valkey-port", 6380, "Valkey server port number")
	cmd.Flags().String("valkey-password", "", "Valkey authentication password (leave empty if no auth required)")
	cmd.Flags().Int("valkey-database", 0, "Valkey database number to migrate to (0-15)")

	// Migration behavior flags
	cmd.Flags().Int("batch-size", 1000, "Number of keys to process in each batch (higher values use more memory)")
	cmd.Flags().Int("retry-attempts", 3, "Number of retry attempts for failed operations before giving up")
	cmd.Flags().String("log-level", "info", "Logging level: trace, debug, info, warn, error, fatal, panic")

	// Bind flags to viper
	viper.BindPFlag("redis.host", cmd.Flags().Lookup("redis-host"))
	viper.BindPFlag("redis.port", cmd.Flags().Lookup("redis-port"))
	viper.BindPFlag("redis.password", cmd.Flags().Lookup("redis-password"))
	viper.BindPFlag("redis.database", cmd.Flags().Lookup("redis-database"))

	viper.BindPFlag("valkey.host", cmd.Flags().Lookup("valkey-host"))
	viper.BindPFlag("valkey.port", cmd.Flags().Lookup("valkey-port"))
	viper.BindPFlag("valkey.password", cmd.Flags().Lookup("valkey-password"))
	viper.BindPFlag("valkey.database", cmd.Flags().Lookup("valkey-database"))

	viper.BindPFlag("migration.batch_size", cmd.Flags().Lookup("batch-size"))
	viper.BindPFlag("migration.retry_attempts", cmd.Flags().Lookup("retry-attempts"))
	viper.BindPFlag("migration.log_level", cmd.Flags().Lookup("log-level"))
}

// LoadConfigWithFlags loads configuration with command-line flag support
func LoadConfigWithFlags() (*Config, error) {
	return LoadConfig()
}
