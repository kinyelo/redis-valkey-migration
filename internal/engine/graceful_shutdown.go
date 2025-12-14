package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"redis-valkey-migration/pkg/logger"
)

// GracefulShutdownManager handles graceful shutdown of the migration process
type GracefulShutdownManager struct {
	logger           logger.Logger
	shutdownTimeout  time.Duration
	shutdownHandlers []func() error
	mu               sync.RWMutex
	shutdownChan     chan os.Signal
	ctx              context.Context
	cancel           context.CancelFunc
	shutdownOnce     sync.Once
}

// NewGracefulShutdownManager creates a new graceful shutdown manager
func NewGracefulShutdownManager(logger logger.Logger, timeout time.Duration) *GracefulShutdownManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &GracefulShutdownManager{
		logger:           logger,
		shutdownTimeout:  timeout,
		shutdownHandlers: make([]func() error, 0),
		shutdownChan:     make(chan os.Signal, 1),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// RegisterShutdownHandler registers a function to be called during graceful shutdown
func (gsm *GracefulShutdownManager) RegisterShutdownHandler(handler func() error) {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()
	gsm.shutdownHandlers = append(gsm.shutdownHandlers, handler)
}

// StartSignalHandler starts listening for shutdown signals
func (gsm *GracefulShutdownManager) StartSignalHandler() {
	signal.Notify(gsm.shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-gsm.shutdownChan
		gsm.logger.Infof("Received shutdown signal: %v", sig)
		gsm.InitiateShutdown()
	}()
}

// InitiateShutdown initiates the graceful shutdown process
func (gsm *GracefulShutdownManager) InitiateShutdown() {
	gsm.shutdownOnce.Do(func() {
		gsm.logger.Info("Initiating graceful shutdown...")

		// Cancel the context to signal all operations to stop
		gsm.cancel()

		// Create a timeout context for the shutdown process
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gsm.shutdownTimeout)
		defer shutdownCancel()

		// Execute shutdown handlers
		gsm.executeShutdownHandlers(shutdownCtx)

		gsm.logger.Info("Graceful shutdown completed")
	})
}

// executeShutdownHandlers executes all registered shutdown handlers
func (gsm *GracefulShutdownManager) executeShutdownHandlers(ctx context.Context) {
	gsm.mu.RLock()
	handlers := make([]func() error, len(gsm.shutdownHandlers))
	copy(handlers, gsm.shutdownHandlers)
	gsm.mu.RUnlock()

	var wg sync.WaitGroup
	errorChan := make(chan error, len(handlers))

	for i, handler := range handlers {
		wg.Add(1)
		go func(index int, h func() error) {
			defer wg.Done()

			gsm.logger.Debugf("Executing shutdown handler %d", index)

			// Execute handler with timeout
			done := make(chan error, 1)
			go func() {
				done <- h()
			}()

			select {
			case err := <-done:
				if err != nil {
					gsm.logger.Errorf("Shutdown handler %d failed: %v", index, err)
					errorChan <- err
				} else {
					gsm.logger.Debugf("Shutdown handler %d completed successfully", index)
				}
			case <-ctx.Done():
				gsm.logger.Warnf("Shutdown handler %d timed out", index)
				errorChan <- fmt.Errorf("shutdown handler %d timed out", index)
			}
		}(i, handler)
	}

	// Wait for all handlers to complete
	wg.Wait()
	close(errorChan)

	// Log any errors that occurred
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		gsm.logger.Errorf("Shutdown completed with %d errors", len(errors))
		for i, err := range errors {
			gsm.logger.Errorf("Shutdown error %d: %v", i+1, err)
		}
	} else {
		gsm.logger.Info("All shutdown handlers completed successfully")
	}
}

// Context returns the shutdown context
func (gsm *GracefulShutdownManager) Context() context.Context {
	return gsm.ctx
}

// IsShuttingDown returns true if shutdown has been initiated
func (gsm *GracefulShutdownManager) IsShuttingDown() bool {
	select {
	case <-gsm.ctx.Done():
		return true
	default:
		return false
	}
}

// CriticalFailureHandler handles critical failures that require immediate shutdown
type CriticalFailureHandler struct {
	logger          logger.Logger
	shutdownManager *GracefulShutdownManager
	failureHandlers map[ErrorType]func(error) error
	mu              sync.RWMutex
}

// NewCriticalFailureHandler creates a new critical failure handler
func NewCriticalFailureHandler(logger logger.Logger, shutdownManager *GracefulShutdownManager) *CriticalFailureHandler {
	return &CriticalFailureHandler{
		logger:          logger,
		shutdownManager: shutdownManager,
		failureHandlers: make(map[ErrorType]func(error) error),
	}
}

// RegisterFailureHandler registers a handler for a specific error type
func (cfh *CriticalFailureHandler) RegisterFailureHandler(errorType ErrorType, handler func(error) error) {
	cfh.mu.Lock()
	defer cfh.mu.Unlock()
	cfh.failureHandlers[errorType] = handler
}

// HandleCriticalFailure handles a critical failure with appropriate response
func (cfh *CriticalFailureHandler) HandleCriticalFailure(operation string, err error) error {
	// Log the critical failure
	errorContext := NewErrorContext(operation, "", err.Error()).WithStackTrace()
	errorContext.LogWithContext(cfh.logger, 0)

	cfh.logger.Errorf("CRITICAL FAILURE in %s: %v", operation, err)

	// Classify the error
	errorType := ClassifyError(err)

	// Execute specific failure handler if registered
	cfh.mu.RLock()
	handler, exists := cfh.failureHandlers[errorType]
	cfh.mu.RUnlock()

	if exists {
		cfh.logger.Infof("Executing specific failure handler for %s", errorType.String())
		if handlerErr := handler(err); handlerErr != nil {
			cfh.logger.Errorf("Failure handler error: %v", handlerErr)
		}
	}

	// Initiate graceful shutdown
	cfh.logger.Info("Initiating graceful shutdown due to critical failure")
	cfh.shutdownManager.InitiateShutdown()

	return fmt.Errorf("critical failure in %s: %w", operation, err)
}

// RecoveryAttempt represents an attempt to recover from a failure
type RecoveryAttempt struct {
	Operation   string
	ErrorType   ErrorType
	Attempt     int
	MaxAttempts int
	Success     bool
	Duration    time.Duration
	Error       error
}

// RecoveryManager manages recovery attempts for different types of failures
type RecoveryManager struct {
	logger           logger.Logger
	recoveryHandlers map[ErrorType]func(error) error
	maxAttempts      map[ErrorType]int
	attempts         map[string][]RecoveryAttempt
	mu               sync.RWMutex
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(logger logger.Logger) *RecoveryManager {
	return &RecoveryManager{
		logger:           logger,
		recoveryHandlers: make(map[ErrorType]func(error) error),
		maxAttempts:      make(map[ErrorType]int),
		attempts:         make(map[string][]RecoveryAttempt),
	}
}

// RegisterRecoveryHandler registers a recovery handler for a specific error type
func (rm *RecoveryManager) RegisterRecoveryHandler(errorType ErrorType, maxAttempts int, handler func(error) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.recoveryHandlers[errorType] = handler
	rm.maxAttempts[errorType] = maxAttempts
}

// AttemptRecovery attempts to recover from an error
func (rm *RecoveryManager) AttemptRecovery(operation string, err error) error {
	errorType := ClassifyError(err)

	rm.mu.Lock()
	handler, hasHandler := rm.recoveryHandlers[errorType]
	maxAttempts, hasMaxAttempts := rm.maxAttempts[errorType]

	if !hasHandler || !hasMaxAttempts {
		rm.mu.Unlock()
		return fmt.Errorf("no recovery handler registered for error type %s", errorType.String())
	}

	// Get current attempts for this operation
	operationAttempts := rm.attempts[operation]
	attemptCount := len(operationAttempts) + 1

	if attemptCount > maxAttempts {
		rm.mu.Unlock()
		return fmt.Errorf("maximum recovery attempts (%d) exceeded for operation %s", maxAttempts, operation)
	}
	rm.mu.Unlock()

	// Attempt recovery
	rm.logger.Infof("Attempting recovery for %s (attempt %d/%d)", operation, attemptCount, maxAttempts)

	start := time.Now()
	recoveryErr := handler(err)
	duration := time.Since(start)

	// Record the attempt
	attempt := RecoveryAttempt{
		Operation:   operation,
		ErrorType:   errorType,
		Attempt:     attemptCount,
		MaxAttempts: maxAttempts,
		Success:     recoveryErr == nil,
		Duration:    duration,
		Error:       recoveryErr,
	}

	rm.mu.Lock()
	rm.attempts[operation] = append(rm.attempts[operation], attempt)
	rm.mu.Unlock()

	if recoveryErr == nil {
		rm.logger.Infof("Recovery successful for %s after %d attempts", operation, attemptCount)
	} else {
		rm.logger.Errorf("Recovery attempt %d failed for %s: %v", attemptCount, operation, recoveryErr)
	}

	return recoveryErr
}

// GetRecoveryAttempts returns all recovery attempts for an operation
func (rm *RecoveryManager) GetRecoveryAttempts(operation string) []RecoveryAttempt {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	attempts := rm.attempts[operation]
	result := make([]RecoveryAttempt, len(attempts))
	copy(result, attempts)
	return result
}

// ClearRecoveryAttempts clears recovery attempts for an operation
func (rm *RecoveryManager) ClearRecoveryAttempts(operation string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.attempts, operation)
}
