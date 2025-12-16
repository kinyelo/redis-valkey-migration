package engine

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// RetryConfig defines configuration for retry mechanisms
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Exponential backoff multiplier
	RetryableErrors []string      // List of error patterns that should trigger retries
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection refused",
			"connection reset",
			"timeout",
			"network",
			"i/o timeout",
			"broken pipe",
			"connection lost",
		},
	}
}

// ConnectionRecovery handles connection recovery and retry logic
type ConnectionRecovery struct {
	config RetryConfig
	logger logger.Logger
}

// NewConnectionRecovery creates a new connection recovery handler
func NewConnectionRecovery(config RetryConfig, logger logger.Logger) *ConnectionRecovery {
	return &ConnectionRecovery{
		config: config,
		logger: logger,
	}
}

// RecoverableClient wraps a DatabaseClient with recovery capabilities
type RecoverableClient struct {
	client   client.DatabaseClient
	config   *client.ClientConfig
	recovery *ConnectionRecovery
	logger   logger.Logger
	name     string // "Redis" or "Valkey" for logging
}

// NewRecoverableClient creates a new recoverable database client
func NewRecoverableClient(client client.DatabaseClient, config *client.ClientConfig, recovery *ConnectionRecovery, logger logger.Logger, name string) *RecoverableClient {
	return &RecoverableClient{
		client:   client,
		config:   config,
		recovery: recovery,
		logger:   logger,
		name:     name,
	}
}

// WithRetry executes an operation with retry logic
func (cr *ConnectionRecovery) WithRetry(operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cr.config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := cr.calculateDelay(attempt)
			cr.logger.Infof("Retrying %s (attempt %d/%d) after %v", operation, attempt, cr.config.MaxAttempts, delay)
			time.Sleep(delay)
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				cr.logger.Infof("Operation %s succeeded after %d retries", operation, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !cr.isRetryableError(err) {
			cr.logError(operation, "", err.Error(), cr.getStackTrace(), attempt)
			return fmt.Errorf("non-retryable error in %s: %w", operation, err)
		}

		cr.logError(operation, "", err.Error(), "", attempt)

		// If this was the last attempt, don't log retry message
		if attempt == cr.config.MaxAttempts {
			break
		}
	}

	cr.logger.Errorf("Operation %s failed after %d attempts: %v", operation, cr.config.MaxAttempts, lastErr)
	return fmt.Errorf("operation %s failed after %d attempts: %w", operation, cr.config.MaxAttempts, lastErr)
}

// calculateDelay calculates exponential backoff delay
func (cr *ConnectionRecovery) calculateDelay(attempt int) time.Duration {
	delay := float64(cr.config.InitialDelay) * math.Pow(cr.config.BackoffFactor, float64(attempt-1))

	if delay > float64(cr.config.MaxDelay) {
		delay = float64(cr.config.MaxDelay)
	}

	return time.Duration(delay)
}

// isRetryableError checks if an error should trigger a retry
func (cr *ConnectionRecovery) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, pattern := range cr.config.RetryableErrors {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// getStackTrace captures the current stack trace
func (cr *ConnectionRecovery) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// logError logs error with context using the logger's LogError method
func (cr *ConnectionRecovery) logError(operation, key, errorMsg, stackTrace string, retryAttempt int) {
	cr.logger.LogError(operation, key, errorMsg, stackTrace, retryAttempt)
}

// RecoverableClient methods with automatic recovery

// Connect establishes connection with retry logic
func (rc *RecoverableClient) Connect() error {
	return rc.recovery.WithRetry(fmt.Sprintf("%s connection", rc.name), func() error {
		start := time.Now()
		err := rc.client.Connect()
		duration := time.Since(start)

		success := err == nil
		rc.logger.LogConnection("connect", rc.config.Host, rc.config.Port, rc.config.Database, success, duration)

		return err
	})
}

// Disconnect closes connection
func (rc *RecoverableClient) Disconnect() error {
	start := time.Now()
	err := rc.client.Disconnect()
	duration := time.Since(start)

	success := err == nil
	rc.logger.LogConnection("disconnect", rc.config.Host, rc.config.Port, rc.config.Database, success, duration)

	return err
}

// Ping tests connection with retry logic
func (rc *RecoverableClient) Ping() error {
	return rc.recovery.WithRetry(fmt.Sprintf("%s ping", rc.name), func() error {
		return rc.client.Ping()
	})
}

// GetAllKeys retrieves all keys with retry logic
func (rc *RecoverableClient) GetAllKeys() ([]string, error) {
	var result []string
	err := rc.recovery.WithRetry(fmt.Sprintf("%s get all keys", rc.name), func() error {
		keys, err := rc.client.GetAllKeys()
		if err != nil {
			return err
		}
		result = keys
		return nil
	})
	return result, err
}

// GetKeysByPattern retrieves keys matching a pattern with retry logic
func (rc *RecoverableClient) GetKeysByPattern(pattern string) ([]string, error) {
	var result []string
	err := rc.recovery.WithRetry(fmt.Sprintf("%s get keys by pattern", rc.name), func() error {
		keys, err := rc.client.GetKeysByPattern(pattern)
		if err != nil {
			return err
		}
		result = keys
		return nil
	})
	return result, err
}

// GetKeyType gets key type with retry logic
func (rc *RecoverableClient) GetKeyType(key string) (string, error) {
	var result string
	err := rc.recovery.WithRetry(fmt.Sprintf("%s get key type", rc.name), func() error {
		keyType, err := rc.client.GetKeyType(key)
		if err != nil {
			return err
		}
		result = keyType
		return nil
	})
	return result, err
}

// GetValue retrieves value with retry logic
func (rc *RecoverableClient) GetValue(key string) (interface{}, error) {
	var result interface{}
	err := rc.recovery.WithRetry(fmt.Sprintf("%s get value", rc.name), func() error {
		value, err := rc.client.GetValue(key)
		if err != nil {
			return err
		}
		result = value
		return nil
	})
	return result, err
}

// SetValue stores value with retry logic
func (rc *RecoverableClient) SetValue(key string, value interface{}) error {
	return rc.recovery.WithRetry(fmt.Sprintf("%s set value", rc.name), func() error {
		return rc.client.SetValue(key, value)
	})
}

// Exists checks key existence with retry logic
func (rc *RecoverableClient) Exists(key string) (bool, error) {
	var result bool
	err := rc.recovery.WithRetry(fmt.Sprintf("%s exists check", rc.name), func() error {
		exists, err := rc.client.Exists(key)
		if err != nil {
			return err
		}
		result = exists
		return nil
	})
	return result, err
}

// GetTTL gets TTL with retry logic
func (rc *RecoverableClient) GetTTL(key string) (time.Duration, error) {
	var result time.Duration
	err := rc.recovery.WithRetry(fmt.Sprintf("%s get TTL", rc.name), func() error {
		ttl, err := rc.client.GetTTL(key)
		if err != nil {
			return err
		}
		result = ttl
		return nil
	})
	return result, err
}

// SetTTL sets TTL with retry logic
func (rc *RecoverableClient) SetTTL(key string, ttl time.Duration) error {
	return rc.recovery.WithRetry(fmt.Sprintf("%s set TTL", rc.name), func() error {
		return rc.client.SetTTL(key, ttl)
	})
}

// ResumeState tracks migration state for resume functionality
type ResumeState struct {
	ProcessedKeys map[string]bool `json:"processed_keys"`
	StartTime     time.Time       `json:"start_time"`
	LastKey       string          `json:"last_key"`
	TotalKeys     int             `json:"total_keys"`
}

// NewResumeState creates a new resume state
func NewResumeState() *ResumeState {
	return &ResumeState{
		ProcessedKeys: make(map[string]bool),
		StartTime:     time.Now(),
	}
}

// IsProcessed checks if a key has already been processed
func (rs *ResumeState) IsProcessed(key string) bool {
	return rs.ProcessedKeys[key]
}

// MarkProcessed marks a key as processed
func (rs *ResumeState) MarkProcessed(key string) {
	rs.ProcessedKeys[key] = true
	rs.LastKey = key
}

// GetProcessedCount returns the number of processed keys
func (rs *ResumeState) GetProcessedCount() int {
	return len(rs.ProcessedKeys)
}

// CriticalErrorHandler handles critical errors that require graceful shutdown
type CriticalErrorHandler struct {
	logger logger.Logger
}

// NewCriticalErrorHandler creates a new critical error handler
func NewCriticalErrorHandler(logger logger.Logger) *CriticalErrorHandler {
	return &CriticalErrorHandler{
		logger: logger,
	}
}

// HandleCriticalError handles critical errors with graceful shutdown
func (ceh *CriticalErrorHandler) HandleCriticalError(operation string, err error, cleanup func() error) error {
	// Log the critical error with full context
	stackTrace := ceh.getStackTrace()
	ceh.logger.LogError(operation, "", err.Error(), stackTrace, 0)

	ceh.logger.Errorf("Critical error in %s: %v", operation, err)
	ceh.logger.Info("Initiating graceful shutdown...")

	// Perform cleanup operations
	if cleanup != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			ceh.logger.Errorf("Error during cleanup: %v", cleanupErr)
		} else {
			ceh.logger.Info("Cleanup completed successfully")
		}
	}

	ceh.logger.Info("Graceful shutdown completed")
	return fmt.Errorf("critical error in %s: %w", operation, err)
}

// getStackTrace captures the current stack trace
func (ceh *CriticalErrorHandler) getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// ErrorContext provides detailed error context for logging
type ErrorContext struct {
	Operation    string            `json:"operation"`
	Key          string            `json:"key,omitempty"`
	ErrorMessage string            `json:"error_message"`
	StackTrace   string            `json:"stack_trace,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// NewErrorContext creates a new error context
func NewErrorContext(operation, key, errorMessage string) *ErrorContext {
	return &ErrorContext{
		Operation:    operation,
		Key:          key,
		ErrorMessage: errorMessage,
		Timestamp:    time.Now(),
		Metadata:     make(map[string]string),
	}
}

// WithStackTrace adds stack trace to error context
func (ec *ErrorContext) WithStackTrace() *ErrorContext {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	ec.StackTrace = string(buf[:n])
	return ec
}

// WithMetadata adds metadata to error context
func (ec *ErrorContext) WithMetadata(key, value string) *ErrorContext {
	if ec.Metadata == nil {
		ec.Metadata = make(map[string]string)
	}
	ec.Metadata[key] = value
	return ec
}

// LogWithContext logs error context using the provided logger
func (ec *ErrorContext) LogWithContext(logger logger.Logger, retryAttempt int) {
	logger.LogError(ec.Operation, ec.Key, ec.ErrorMessage, ec.StackTrace, retryAttempt)
}
