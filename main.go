package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/internal/config"
	"github.com/kinyelo/redis-valkey-migration/internal/engine"
	"github.com/kinyelo/redis-valkey-migration/internal/version"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"

	"github.com/spf13/cobra"
)

var (
	configFile string
	dryRun     bool
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "redis-valkey-migration",
	Short: "A tool to migrate data from Redis to Valkey",
	Long: `Redis to Valkey Migration Tool is a Go-based command-line application 
that provides complete data migration between Redis and Valkey databases.
It supports all Redis data types and provides comprehensive progress monitoring 
and verification capabilities.

The tool handles strings, hashes, lists, sets, and sorted sets with full
data integrity verification and comprehensive error handling.`,
	Example: `  # Basic migration with default settings
  redis-valkey-migration migrate

  # Migration with custom Redis and Valkey hosts
  redis-valkey-migration migrate --redis-host redis.example.com --valkey-host valkey.example.com

  # Migration with authentication
  redis-valkey-migration migrate --redis-password secret123 --valkey-password secret456

  # Dry run to see what would be migrated
  redis-valkey-migration migrate --dry-run

  # Migration with custom batch size and logging
  redis-valkey-migration migrate --batch-size 500 --log-level debug

  # Resume a previous migration
  redis-valkey-migration migrate --resume-file /path/to/resume.json`,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Start the migration process",
	Long: `Start migrating data from Redis to Valkey database.

This command will:
1. Connect to both Redis and Valkey databases
2. Discover all keys in the Redis database
3. Transfer each key with its data and type information
4. Verify data integrity after transfer
5. Provide comprehensive progress reporting and logging

The migration supports all Redis data types including strings, hashes, 
lists, sets, and sorted sets. It includes automatic retry logic for 
network errors and can resume interrupted migrations.`,
	Example: `  # Basic migration
  redis-valkey-migration migrate

  # Migration with custom connection settings
  redis-valkey-migration migrate \
    --redis-host localhost --redis-port 6379 --redis-database 0 \
    --valkey-host localhost --valkey-port 6380 --valkey-database 0

  # Migration with authentication and custom settings
  redis-valkey-migration migrate \
    --redis-password myredispass \
    --valkey-password myvalkeypass \
    --batch-size 1000 \
    --max-concurrency 5 \
    --log-level debug

  # Dry run to preview migration
  redis-valkey-migration migrate --dry-run

  # Resume interrupted migration
  redis-valkey-migration migrate --resume-file migration_state.json`,
	RunE: runMigration,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print detailed version information including build metadata",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetVersionString())
	},
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is $HOME/.redis-valkey-migration/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output with debug information")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "perform a dry run without actually migrating data (shows what would be migrated)")

	// Bind configuration flags to migrate command
	config.BindFlags(migrateCmd)

	// Add additional migration-specific flags with better descriptions
	migrateCmd.Flags().Bool("verify", true, "verify data integrity after migration completion")
	migrateCmd.Flags().Bool("continue-on-error", true, "continue migration even if some individual keys fail to transfer")
	migrateCmd.Flags().String("resume-file", "migration_resume.json", "file to store migration state for resume capability")
	migrateCmd.Flags().Duration("progress-interval", 5000000000, "interval for progress reporting (e.g., 5s, 1m, 30s)")
	migrateCmd.Flags().Int("max-concurrency", 10, "maximum number of concurrent key transfer operations")

	// Set up command completion
	rootCmd.CompletionOptions.DisableDefaultCmd = false

	// Add commands
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runMigration(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfigWithFlags()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set up logger
	logLevel := cfg.Migration.LogLevel
	if verbose {
		logLevel = "debug"
	}

	logConfig := logger.Config{
		Level:      logLevel,
		OutputFile: "migration.log",
		MaxSize:    10 * 1024 * 1024, // 10MB
		MaxAge:     7,                // 7 days
		Format:     "text",
	}

	log, err := logger.NewLogger(logConfig)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	log.Info("Starting Redis to Valkey migration")
	log.Infof("Configuration: Redis=%s:%d DB=%d, Valkey=%s:%d DB=%d",
		cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Database,
		cfg.Valkey.Host, cfg.Valkey.Port, cfg.Valkey.Database)

	if dryRun {
		log.Info("DRY RUN MODE: No data will be actually migrated")
		return runDryRun(cfg, log)
	}

	// Create database clients
	redisClient, err := createRedisClient(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}

	valkeyClient, err := createValkeyClient(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create Valkey client: %w", err)
	}

	// Create engine configuration
	engineConfig := createEngineConfig(cmd, cfg)

	// Create client configurations with proper timeouts
	redisConfig := client.NewClientConfig(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.Database)
	valkeyConfig := client.NewClientConfig(cfg.Valkey.Host, cfg.Valkey.Port, cfg.Valkey.Password, cfg.Valkey.Database)

	// Create migration engine
	migrationEngine, err := engine.NewMigrationEngine(
		redisClient,
		redisConfig,
		valkeyClient,
		valkeyConfig,
		log,
		engineConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration engine: %w", err)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal, initiating graceful shutdown...")
		migrationEngine.Shutdown()
	}()

	// Start migration
	if err := migrationEngine.Migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Print final statistics
	stats := migrationEngine.GetStats()
	log.Info("Migration completed successfully")
	log.Infof("Final statistics: Total=%d, Processed=%d, Failed=%d, Duration=%v, Throughput=%.2f keys/sec",
		stats.TotalKeys, stats.ProcessedKeys, stats.FailedKeys, stats.Duration, stats.Throughput)

	return nil
}

func createRedisClient(cfg *config.Config, log logger.Logger) (client.DatabaseClient, error) {
	clientConfig := client.NewClientConfig(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.Database)

	redisClient := client.NewRedisClient(clientConfig)
	return redisClient, nil
}

func createValkeyClient(cfg *config.Config, log logger.Logger) (client.DatabaseClient, error) {
	clientConfig := client.NewClientConfig(cfg.Valkey.Host, cfg.Valkey.Port, cfg.Valkey.Password, cfg.Valkey.Database)

	valkeyClient := client.NewValkeyClient(clientConfig)
	return valkeyClient, nil
}

func createEngineConfig(cmd *cobra.Command, cfg *config.Config) *engine.EngineConfig {
	engineConfig := engine.DefaultEngineConfig()

	// Override with command-line flags
	if verify, _ := cmd.Flags().GetBool("verify"); cmd.Flags().Changed("verify") {
		engineConfig.VerifyAfterMigration = verify
	}

	if continueOnError, _ := cmd.Flags().GetBool("continue-on-error"); cmd.Flags().Changed("continue-on-error") {
		engineConfig.ContinueOnError = continueOnError
	}

	if resumeFile, _ := cmd.Flags().GetString("resume-file"); cmd.Flags().Changed("resume-file") {
		engineConfig.ResumeFile = resumeFile
	}

	if progressInterval, _ := cmd.Flags().GetDuration("progress-interval"); cmd.Flags().Changed("progress-interval") {
		engineConfig.ProgressInterval = progressInterval
	}

	if maxConcurrency, _ := cmd.Flags().GetInt("max-concurrency"); cmd.Flags().Changed("max-concurrency") {
		engineConfig.MaxConcurrency = maxConcurrency
	}

	// Use batch size from migration config
	engineConfig.BatchSize = cfg.Migration.BatchSize

	return engineConfig
}

func runDryRun(cfg *config.Config, log logger.Logger) error {
	log.Info("Performing dry run - connecting to Redis to discover keys")

	// Create Redis client for discovery
	redisClient, err := createRedisClient(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Connect to Redis
	if err := redisClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Disconnect()

	// Discover keys
	keys, err := redisClient.GetAllKeys()
	if err != nil {
		return fmt.Errorf("failed to discover keys: %w", err)
	}

	log.Infof("Dry run completed: Found %d keys to migrate", len(keys))

	// Show sample of keys and their types
	sampleSize := 10
	if len(keys) < sampleSize {
		sampleSize = len(keys)
	}

	log.Info("Sample keys and types:")
	for i := 0; i < sampleSize; i++ {
		keyType, err := redisClient.GetKeyType(keys[i])
		if err != nil {
			log.Warnf("Failed to get type for key %s: %v", keys[i], err)
			continue
		}
		log.Infof("  %s (%s)", keys[i], keyType)
	}

	if len(keys) > sampleSize {
		log.Infof("  ... and %d more keys", len(keys)-sampleSize)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
