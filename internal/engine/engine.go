package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/internal/monitor"
	"github.com/kinyelo/redis-valkey-migration/internal/processor"
	"github.com/kinyelo/redis-valkey-migration/internal/scanner"
	"github.com/kinyelo/redis-valkey-migration/internal/verifier"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// MigrationEngine orchestrates the entire migration process with error handling and recovery
type MigrationEngine struct {
	sourceClient     *RecoverableClient
	targetClient     *RecoverableClient
	processor        processor.DataProcessor
	monitor          *monitor.ProgressMonitor
	verifier         verifier.DataVerifier
	scanner          scanner.KeyScanner
	logger           logger.Logger
	recovery         *ConnectionRecovery
	criticalHandler  *CriticalErrorHandler
	shutdownManager  *GracefulShutdownManager
	failureHandler   *CriticalFailureHandler
	recoveryManager  *RecoveryManager
	resumeState      *ResumeState
	config           *EngineConfig
	mu               sync.RWMutex
	ctx              context.Context
	shutdownComplete chan struct{}
}

// EngineConfig holds configuration for the migration engine
type EngineConfig struct {
	BatchSize            int           `json:"batch_size"`
	ResumeFile           string        `json:"resume_file"`
	VerifyAfterMigration bool          `json:"verify_after_migration"`
	ContinueOnError      bool          `json:"continue_on_error"`
	MaxConcurrency       int           `json:"max_concurrency"`
	ProgressInterval     time.Duration `json:"progress_interval"`
}

// DefaultEngineConfig returns default engine configuration
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		BatchSize:            1000,
		ResumeFile:           "migration_resume.json",
		VerifyAfterMigration: true,
		ContinueOnError:      true,
		MaxConcurrency:       10,
		ProgressInterval:     5 * time.Second,
	}
}

// NewMigrationEngine creates a new migration engine with error handling capabilities
func NewMigrationEngine(
	sourceClient client.DatabaseClient,
	sourceConfig *client.ClientConfig,
	targetClient client.DatabaseClient,
	targetConfig *client.ClientConfig,
	logger logger.Logger,
	config *EngineConfig,
) (*MigrationEngine, error) {
	if config == nil {
		config = DefaultEngineConfig()
	}

	// Create recovery handler
	retryConfig := DefaultRetryConfig()
	recovery := NewConnectionRecovery(retryConfig, logger)

	// Wrap clients with recovery capabilities
	recoverableSource := NewRecoverableClient(sourceClient, sourceConfig, recovery, logger, "Redis")
	recoverableTarget := NewRecoverableClient(targetClient, targetConfig, recovery, logger, "Valkey")

	// Create critical error handler
	criticalHandler := NewCriticalErrorHandler(logger)

	// Create graceful shutdown manager
	shutdownManager := NewGracefulShutdownManager(logger, 30*time.Second)

	// Create critical failure handler
	failureHandler := NewCriticalFailureHandler(logger, shutdownManager)

	// Create recovery manager
	recoveryManager := NewRecoveryManager(logger)

	// Use shutdown manager's context
	ctx := shutdownManager.Context()

	// Create components
	progressMonitor := monitor.NewProgressMonitor(logger)
	dataProcessor := processor.NewDataProcessor(logger)
	dataVerifier := verifier.NewDataVerifier(logger)
	keyScanner := scanner.NewKeyScanner(logger)

	// Load or create resume state
	resumeState, err := loadResumeState(config.ResumeFile)
	if err != nil {
		logger.Warnf("Could not load resume state: %v. Starting fresh migration.", err)
		resumeState = NewResumeState()
	}

	engine := &MigrationEngine{
		sourceClient:     recoverableSource,
		targetClient:     recoverableTarget,
		processor:        dataProcessor,
		monitor:          progressMonitor,
		verifier:         dataVerifier,
		scanner:          keyScanner,
		logger:           logger,
		recovery:         recovery,
		criticalHandler:  criticalHandler,
		shutdownManager:  shutdownManager,
		failureHandler:   failureHandler,
		recoveryManager:  recoveryManager,
		resumeState:      resumeState,
		config:           config,
		ctx:              ctx,
		shutdownComplete: make(chan struct{}),
	}

	// Register shutdown handlers
	shutdownManager.RegisterShutdownHandler(engine.cleanup)

	return engine, nil
}

// Migrate performs the complete migration with error handling and recovery
func (me *MigrationEngine) Migrate() error {
	me.logger.Info("Starting Redis to Valkey migration with error handling and recovery")

	// Setup graceful shutdown handling
	defer me.gracefulShutdown()

	// Start signal handler for graceful shutdown
	me.shutdownManager.StartSignalHandler()

	// Connect to databases with retry logic
	if err := me.connectDatabases(); err != nil {
		return me.failureHandler.HandleCriticalFailure("database connection", err)
	}

	// Discover keys to migrate
	keys, err := me.discoverKeys()
	if err != nil {
		return me.failureHandler.HandleCriticalFailure("key discovery", err)
	}

	// Initialize progress monitoring
	me.monitor.Initialize(len(keys))
	me.resumeState.TotalKeys = len(keys)

	// Start progress reporting
	go me.startProgressReporting()

	// Perform migration with error handling
	if err := me.performMigration(keys); err != nil {
		if IsCritical(err) {
			return me.failureHandler.HandleCriticalFailure("migration", err)
		}
		me.logger.Errorf("Migration completed with errors: %v", err)
	}

	// Verify migration if configured
	if me.config.VerifyAfterMigration {
		if err := me.verifyMigration(keys); err != nil {
			me.logger.Errorf("Migration verification failed: %v", err)
			return err
		}
	}

	// Clean up resume state on successful completion
	me.cleanupResumeState()

	me.logger.Info("Migration completed successfully")
	return nil
}

// connectDatabases establishes connections to both databases
func (me *MigrationEngine) connectDatabases() error {
	me.logger.Info("Connecting to databases...")

	// Connect to source (Redis)
	if err := me.sourceClient.Connect(); err != nil {
		return WrapError(err, "source database connection")
	}

	// Connect to target (Valkey)
	if err := me.targetClient.Connect(); err != nil {
		return WrapError(err, "target database connection")
	}

	// Test connections
	if err := me.sourceClient.Ping(); err != nil {
		return WrapError(err, "source database ping")
	}

	if err := me.targetClient.Ping(); err != nil {
		return WrapError(err, "target database ping")
	}

	me.logger.Info("Database connections established successfully")
	return nil
}

// discoverKeys discovers all keys to migrate
func (me *MigrationEngine) discoverKeys() ([]string, error) {
	me.logger.Info("Discovering keys to migrate...")

	keys, err := me.scanner.ScanAllKeys(me.sourceClient)
	if err != nil {
		return nil, WrapError(err, "key discovery")
	}

	me.logger.Infof("Discovered %d keys to migrate", len(keys))
	return keys, nil
}

// performMigration performs the actual migration with error handling
func (me *MigrationEngine) performMigration(keys []string) error {
	me.logger.Info("Starting key migration...")

	errorAggregator := NewErrorAggregator()

	for _, key := range keys {
		// Check for cancellation
		select {
		case <-me.ctx.Done():
			me.logger.Info("Migration cancelled")
			return me.ctx.Err()
		default:
		}

		// Skip if already processed (resume functionality)
		if me.resumeState.IsProcessed(key) {
			me.logger.Debugf("Skipping already processed key: %s", key)
			continue
		}

		// Migrate individual key with error handling
		if err := me.migrateKey(key); err != nil {
			errorAggregator.Add(err)

			// Check if error is critical
			if IsCritical(err) {
				return err
			}

			// Continue or stop based on configuration
			if !me.config.ContinueOnError {
				return errorAggregator
			}

			me.logger.Warnf("Continuing migration despite error for key %s: %v", key, err)
		} else {
			// Mark key as processed for resume functionality
			me.resumeState.MarkProcessed(key)
			me.monitor.IncrementProcessed()
		}

		// Save resume state periodically
		if me.resumeState.GetProcessedCount()%100 == 0 {
			if err := me.saveResumeState(); err != nil {
				me.logger.Warnf("Failed to save resume state: %v", err)
			}
		}
	}

	// Return aggregated errors if any
	if errorAggregator.HasErrors() {
		return errorAggregator
	}

	return nil
}

// migrateKey migrates a single key with error handling
func (me *MigrationEngine) migrateKey(key string) error {
	// Get key type
	keyType, err := me.sourceClient.GetKeyType(key)
	if err != nil {
		return WrapError(err, "get key type").WithKey(key)
	}

	// Process the key based on its type
	err = me.processor.ProcessKey(key, keyType, me.sourceClient, me.targetClient)
	if err != nil {
		me.monitor.IncrementFailed()
		return WrapError(err, "key processing").WithKey(key)
	}

	// The processor already logs the successful transfer with correct size
	return nil
}

// verifyMigration verifies the migration results
func (me *MigrationEngine) verifyMigration(keys []string) error {
	me.logger.Info("Verifying migration results...")

	errorAggregator := NewErrorAggregator()

	for _, key := range keys {
		result := me.verifier.VerifyKey(key, me.sourceClient, me.targetClient)
		if !result.Success {
			err := fmt.Errorf("verification failed: %s", result.ErrorMsg)
			errorAggregator.Add(WrapError(err, "verification").WithKey(key))
		}
	}

	if errorAggregator.HasErrors() {
		return errorAggregator
	}

	me.logger.Info("Migration verification completed successfully")
	return nil
}

// startProgressReporting starts periodic progress reporting
func (me *MigrationEngine) startProgressReporting() {
	ticker := time.NewTicker(me.config.ProgressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := me.monitor.GetStats()
			me.logger.LogProgress(stats.TotalKeys, stats.ProcessedKeys, stats.FailedKeys, stats.Throughput)
		case <-me.ctx.Done():
			return
		}
	}
}

// saveResumeState saves the current resume state to disk
func (me *MigrationEngine) saveResumeState() error {
	me.mu.Lock()
	defer me.mu.Unlock()

	data, err := json.Marshal(me.resumeState)
	if err != nil {
		return fmt.Errorf("failed to marshal resume state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(me.config.ResumeFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create resume state directory: %w", err)
	}

	// Write to temporary file first, then rename for atomicity
	tempFile := me.config.ResumeFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write resume state: %w", err)
	}

	if err := os.Rename(tempFile, me.config.ResumeFile); err != nil {
		return fmt.Errorf("failed to rename resume state file: %w", err)
	}

	return nil
}

// loadResumeState loads resume state from disk
func loadResumeState(filename string) (*ResumeState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// cleanupResumeState removes the resume state file after successful completion
func (me *MigrationEngine) cleanupResumeState() {
	if err := os.Remove(me.config.ResumeFile); err != nil && !os.IsNotExist(err) {
		me.logger.Warnf("Failed to remove resume state file: %v", err)
	}
}

// cleanup performs cleanup operations
func (me *MigrationEngine) cleanup() error {
	me.logger.Info("Performing cleanup operations...")

	var errors []error

	// Save current resume state
	if err := me.saveResumeState(); err != nil {
		errors = append(errors, fmt.Errorf("failed to save resume state: %w", err))
	}

	// Disconnect from databases
	if err := me.sourceClient.Disconnect(); err != nil {
		errors = append(errors, fmt.Errorf("failed to disconnect from source: %w", err))
	}

	if err := me.targetClient.Disconnect(); err != nil {
		errors = append(errors, fmt.Errorf("failed to disconnect from target: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// gracefulShutdown performs graceful shutdown
func (me *MigrationEngine) gracefulShutdown() {
	// Shutdown manager handles the graceful shutdown process
	me.shutdownManager.InitiateShutdown()
	close(me.shutdownComplete)
}

// Shutdown initiates graceful shutdown
func (me *MigrationEngine) Shutdown() {
	me.logger.Info("Shutdown requested")
	me.shutdownManager.InitiateShutdown()

	// Wait for shutdown to complete
	<-me.shutdownComplete
	me.logger.Info("Shutdown completed")
}

// GetStats returns current migration statistics
func (me *MigrationEngine) GetStats() monitor.MigrationStats {
	return me.monitor.GetStats()
}
