package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/mock"
)

// **Feature: redis-valkey-migration, Property 11: Graceful Critical Error Handling**
// **Validates: Requirements 6.4**
func TestProperty_GracefulCriticalErrorHandling(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10
	parameters.MaxSize = 5
	properties := gopter.NewProperties(parameters)

	properties.Property("graceful shutdown manager should execute all registered handlers within timeout", prop.ForAll(
		func(handlerCount int, timeoutMs int) bool {
			// Ensure valid test parameters
			if handlerCount < 1 || handlerCount > 10 {
				handlerCount = 5
			}
			if timeoutMs < 100 || timeoutMs > 5000 {
				timeoutMs = 1000
			}

			timeout := time.Duration(timeoutMs) * time.Millisecond

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Info", mock.AnythingOfType("string")).Return()

			// Create graceful shutdown manager
			shutdownManager := NewGracefulShutdownManager(mockLogger, timeout)

			// Track handler executions
			var executedHandlers sync.Map
			var handlerErrors []error

			// Register handlers
			for i := 0; i < handlerCount; i++ {
				handlerIndex := i
				shutdownManager.RegisterShutdownHandler(func() error {
					executedHandlers.Store(handlerIndex, true)

					// Simulate some work (but keep it fast for testing)
					time.Sleep(time.Millisecond * 10)

					// Occasionally return an error to test error handling
					if handlerIndex%3 == 0 {
						err := fmt.Errorf("handler %d error", handlerIndex)
						handlerErrors = append(handlerErrors, err)
						return err
					}
					return nil
				})
			}

			// Execute shutdown
			shutdownManager.InitiateShutdown()

			// Verify all handlers were executed
			for i := 0; i < handlerCount; i++ {
				if _, executed := executedHandlers.Load(i); !executed {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 8),      // handlerCount
		gen.IntRange(500, 2000), // timeoutMs
	))

	properties.Property("critical failure handler should classify errors and initiate shutdown", prop.ForAll(
		func(errorTypeIndex int) bool {
			// Ensure valid test parameters
			if errorTypeIndex < 0 || errorTypeIndex > 5 {
				errorTypeIndex = 0
			}

			// Define test errors for different types
			testErrors := []error{
				errors.New("connection refused"),      // ConnectionError
				errors.New("network timeout"),         // NetworkError
				errors.New("authentication failed"),   // AuthenticationError
				errors.New("invalid data format"),     // DataError
				errors.New("invalid configuration"),   // ConfigurationError
				errors.New("critical system failure"), // CriticalError
			}

			expectedTypes := []ErrorType{
				ConnectionError,
				NetworkError,
				AuthenticationError,
				DataError,
				ConfigurationError,
				CriticalError,
			}

			testError := testErrors[errorTypeIndex]
			expectedType := expectedTypes[errorTypeIndex]

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Info", mock.AnythingOfType("string")).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			// Create shutdown manager and failure handler
			shutdownManager := NewGracefulShutdownManager(mockLogger, time.Second)
			failureHandler := NewCriticalFailureHandler(mockLogger, shutdownManager)

			// Track if shutdown was initiated
			shutdownInitiated := false
			shutdownManager.RegisterShutdownHandler(func() error {
				shutdownInitiated = true
				return nil
			})

			// Handle the critical failure
			err := failureHandler.HandleCriticalFailure("test_operation", testError)

			// Should return an error
			if err == nil {
				return false
			}

			// Should have initiated shutdown
			if !shutdownInitiated {
				return false
			}

			// Verify error classification
			classifiedType := ClassifyError(testError)
			if classifiedType != expectedType {
				return false
			}

			return true
		},
		gen.IntRange(0, 5), // errorTypeIndex
	))

	properties.Property("recovery manager should attempt recovery up to max attempts", prop.ForAll(
		func(maxAttempts int, shouldSucceed bool) bool {
			// Ensure valid test parameters
			if maxAttempts < 1 || maxAttempts > 5 {
				maxAttempts = 3
			}

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()

			// Create recovery manager
			recoveryManager := NewRecoveryManager(mockLogger)

			// Track recovery attempts
			attemptCount := 0

			// Register recovery handler
			recoveryManager.RegisterRecoveryHandler(ConnectionError, maxAttempts, func(err error) error {
				attemptCount++
				if shouldSucceed && attemptCount == maxAttempts {
					return nil // Success on final attempt
				}
				return fmt.Errorf("recovery attempt %d failed", attemptCount)
			})

			// Attempt recovery
			testError := errors.New("connection refused")
			operation := "test_connection"

			var finalErr error
			for i := 0; i < maxAttempts+2; i++ { // Try more than max to test limit
				finalErr = recoveryManager.AttemptRecovery(operation, testError)
				if finalErr == nil {
					break // Success
				}
				if i >= maxAttempts-1 {
					// Should fail after max attempts
					break
				}
			}

			// Verify attempt count doesn't exceed maximum
			if attemptCount > maxAttempts {
				return false
			}

			// Verify success/failure matches expectation
			if shouldSucceed {
				// Should succeed within max attempts
				if finalErr != nil && attemptCount == maxAttempts {
					return false
				}
			} else {
				// Should fail after max attempts
				if finalErr == nil && attemptCount == maxAttempts {
					return false
				}
			}

			// Verify recovery attempts are tracked
			attempts := recoveryManager.GetRecoveryAttempts(operation)
			if len(attempts) == 0 {
				return false
			}

			return true
		},
		gen.IntRange(1, 4), // maxAttempts
		gen.Bool(),         // shouldSucceed
	))

	properties.Property("shutdown manager should handle concurrent shutdown requests gracefully", prop.ForAll(
		func(concurrentRequests int) bool {
			// Ensure valid test parameters
			if concurrentRequests < 1 || concurrentRequests > 10 {
				concurrentRequests = 5
			}

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Info", mock.AnythingOfType("string")).Return()

			// Create shutdown manager
			shutdownManager := NewGracefulShutdownManager(mockLogger, time.Second)

			// Track shutdown handler executions
			executionCount := 0
			var mu sync.Mutex

			shutdownManager.RegisterShutdownHandler(func() error {
				mu.Lock()
				executionCount++
				mu.Unlock()
				return nil
			})

			// Launch concurrent shutdown requests
			var wg sync.WaitGroup
			for i := 0; i < concurrentRequests; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					shutdownManager.InitiateShutdown()
				}()
			}

			// Wait for all requests to complete
			wg.Wait()

			// Give some time for handlers to execute
			time.Sleep(100 * time.Millisecond)

			// Shutdown handler should be executed only once despite multiple requests
			mu.Lock()
			finalCount := executionCount
			mu.Unlock()

			if finalCount != 1 {
				return false
			}

			// Context should be cancelled
			select {
			case <-shutdownManager.Context().Done():
				// Expected - context should be cancelled
			default:
				return false
			}

			return true
		},
		gen.IntRange(2, 8), // concurrentRequests
	))

	properties.Property("error context should preserve all metadata through graceful shutdown", prop.ForAll(
		func(suffix int) bool {
			// Use fixed strings with suffix for variation
			operation := fmt.Sprintf("test_operation_%d", suffix)
			key := fmt.Sprintf("test_key_%d", suffix)
			errorMsg := fmt.Sprintf("test_error_message_%d", suffix)

			// Create error context with metadata
			errorContext := NewErrorContext(operation, key, errorMsg)
			errorContext.WithMetadata("severity", "critical")
			errorContext.WithMetadata("component", "migration_engine")
			errorContext.WithStackTrace()

			// Verify all fields are preserved
			if errorContext.Operation != operation {
				return false
			}
			if errorContext.Key != key {
				return false
			}
			if errorContext.ErrorMessage != errorMsg {
				return false
			}

			// Verify metadata is preserved
			if errorContext.Metadata["severity"] != "critical" {
				return false
			}
			if errorContext.Metadata["component"] != "migration_engine" {
				return false
			}

			// Verify stack trace is present
			if len(errorContext.StackTrace) == 0 {
				return false
			}

			// Verify timestamp is set
			if errorContext.Timestamp.IsZero() {
				return false
			}

			return true
		},
		gen.IntRange(1, 100), // suffix
	))

	properties.TestingRun(t)
}
