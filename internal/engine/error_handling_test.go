package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestConnectionLossScenarios tests various connection loss scenarios
func TestConnectionLossScenarios(t *testing.T) {
	tests := []struct {
		name           string
		errorSequence  []error
		expectedResult error
		expectedCalls  int
	}{
		{
			name: "connection recovers after 2 attempts",
			errorSequence: []error{
				errors.New("connection refused"),
				errors.New("connection refused"),
				nil, // Success on 3rd attempt
			},
			expectedResult: nil,
			expectedCalls:  3,
		},
		{
			name: "connection fails with non-retryable error",
			errorSequence: []error{
				errors.New("authentication failed"),
			},
			expectedResult: errors.New("non-retryable error in test operation: authentication failed"),
			expectedCalls:  1,
		},
		{
			name: "connection exhausts all retry attempts",
			errorSequence: []error{
				errors.New("network timeout"),
				errors.New("network timeout"),
				errors.New("network timeout"),
				errors.New("network timeout"),
			},
			expectedResult: errors.New("operation test operation failed after 3 attempts: network timeout"),
			expectedCalls:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			// Create retry configuration
			retryConfig := RetryConfig{
				MaxAttempts:   3,
				InitialDelay:  time.Millisecond,
				MaxDelay:      10 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{
					"connection refused",
					"network timeout",
					"timeout",
				},
			}

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			// Track function calls
			callCount := 0
			err := recovery.WithRetry("test operation", func() error {
				if callCount < len(tt.errorSequence) {
					result := tt.errorSequence[callCount]
					callCount++
					return result
				}
				callCount++
				return nil
			})

			// Verify results
			assert.Equal(t, tt.expectedCalls, callCount)
			if tt.expectedResult == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "test operation")
			}
		})
	}
}

// TestRetryMechanismBehavior tests the retry mechanism behavior
func TestRetryMechanismBehavior(t *testing.T) {
	t.Run("exponential backoff calculation", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

		retryConfig := RetryConfig{
			MaxAttempts:     3,
			InitialDelay:    100 * time.Millisecond,
			MaxDelay:        1 * time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []string{"network error"},
		}

		recovery := NewConnectionRecovery(retryConfig, mockLogger)

		// Test delay calculations
		delay1 := recovery.calculateDelay(1)
		delay2 := recovery.calculateDelay(2)
		delay3 := recovery.calculateDelay(3)

		assert.Equal(t, 100*time.Millisecond, delay1)
		assert.Equal(t, 200*time.Millisecond, delay2)
		assert.Equal(t, 400*time.Millisecond, delay3)
	})

	t.Run("max delay cap", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

		retryConfig := RetryConfig{
			MaxAttempts:     5,
			InitialDelay:    100 * time.Millisecond,
			MaxDelay:        300 * time.Millisecond, // Cap at 300ms
			BackoffFactor:   2.0,
			RetryableErrors: []string{"network error"},
		}

		recovery := NewConnectionRecovery(retryConfig, mockLogger)

		// Test that delay is capped at MaxDelay
		delay4 := recovery.calculateDelay(4) // Would be 800ms without cap
		delay5 := recovery.calculateDelay(5) // Would be 1600ms without cap

		assert.Equal(t, 300*time.Millisecond, delay4)
		assert.Equal(t, 300*time.Millisecond, delay5)
	})

	t.Run("error classification", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

		retryConfig := DefaultRetryConfig()
		recovery := NewConnectionRecovery(retryConfig, mockLogger)

		// Test retryable errors
		retryableErrors := []string{
			"connection refused",
			"network timeout",
			"i/o timeout",
			"broken pipe",
		}

		for _, errMsg := range retryableErrors {
			err := errors.New(errMsg)
			assert.True(t, recovery.isRetryableError(err), "Error should be retryable: %s", errMsg)
		}

		// Test non-retryable errors
		nonRetryableErrors := []string{
			"authentication failed",
			"invalid credentials",
			"permission denied",
		}

		for _, errMsg := range nonRetryableErrors {
			err := errors.New(errMsg)
			assert.False(t, recovery.isRetryableError(err), "Error should not be retryable: %s", errMsg)
		}
	})
}

// TestGracefulShutdownProcedures tests graceful shutdown procedures
func TestGracefulShutdownProcedures(t *testing.T) {
	t.Run("shutdown handlers execution", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Info", mock.AnythingOfType("string")).Return()

		shutdownManager := NewGracefulShutdownManager(mockLogger, 5*time.Second)

		// Track handler executions
		var executedHandlers []int

		// Register multiple handlers
		for i := 0; i < 3; i++ {
			handlerIndex := i
			shutdownManager.RegisterShutdownHandler(func() error {
				executedHandlers = append(executedHandlers, handlerIndex)
				return nil
			})
		}

		// Execute shutdown
		shutdownManager.InitiateShutdown()

		// Verify all handlers were executed
		assert.Len(t, executedHandlers, 3)
		assert.Contains(t, executedHandlers, 0)
		assert.Contains(t, executedHandlers, 1)
		assert.Contains(t, executedHandlers, 2)
	})

	t.Run("shutdown handler errors", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Info", mock.AnythingOfType("string")).Return()

		shutdownManager := NewGracefulShutdownManager(mockLogger, 5*time.Second)

		// Register handlers that return errors
		shutdownManager.RegisterShutdownHandler(func() error {
			return errors.New("handler 1 error")
		})
		shutdownManager.RegisterShutdownHandler(func() error {
			return nil // Success
		})
		shutdownManager.RegisterShutdownHandler(func() error {
			return errors.New("handler 3 error")
		})

		// Execute shutdown - should not panic despite errors
		require.NotPanics(t, func() {
			shutdownManager.InitiateShutdown()
		})
	})

	t.Run("context cancellation", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Info", mock.AnythingOfType("string")).Return()

		shutdownManager := NewGracefulShutdownManager(mockLogger, 5*time.Second)

		// Context should not be cancelled initially
		assert.False(t, shutdownManager.IsShuttingDown())

		// Execute shutdown
		shutdownManager.InitiateShutdown()

		// Context should be cancelled after shutdown
		assert.True(t, shutdownManager.IsShuttingDown())
	})
}

// TestErrorLoggingCompleteness tests error logging completeness
func TestErrorLoggingCompleteness(t *testing.T) {
	t.Run("error context creation", func(t *testing.T) {
		operation := "test_operation"
		key := "test_key"
		errorMsg := "test error message"

		errorContext := NewErrorContext(operation, key, errorMsg)

		assert.Equal(t, operation, errorContext.Operation)
		assert.Equal(t, key, errorContext.Key)
		assert.Equal(t, errorMsg, errorContext.ErrorMessage)
		assert.NotZero(t, errorContext.Timestamp)
		assert.NotNil(t, errorContext.Metadata)
	})

	t.Run("error context with metadata", func(t *testing.T) {
		errorContext := NewErrorContext("op", "key", "msg")
		errorContext.WithMetadata("severity", "high")
		errorContext.WithMetadata("component", "engine")

		assert.Equal(t, "high", errorContext.Metadata["severity"])
		assert.Equal(t, "engine", errorContext.Metadata["component"])
	})

	t.Run("error context with stack trace", func(t *testing.T) {
		errorContext := NewErrorContext("op", "key", "msg")
		errorContext.WithStackTrace()

		assert.NotEmpty(t, errorContext.StackTrace)
		assert.Contains(t, errorContext.StackTrace, "TestErrorLoggingCompleteness")
	})

	t.Run("migration error creation and wrapping", func(t *testing.T) {
		originalErr := errors.New("original error")
		operation := "test_operation"

		migrationErr := WrapError(originalErr, operation)

		assert.NotNil(t, migrationErr)
		assert.Equal(t, operation, migrationErr.Operation)
		assert.Equal(t, originalErr.Error(), migrationErr.Message)
		assert.Equal(t, originalErr, migrationErr.Cause)
	})

	t.Run("error aggregation", func(t *testing.T) {
		aggregator := NewErrorAggregator()

		// Initially no errors
		assert.False(t, aggregator.HasErrors())
		assert.Equal(t, 0, aggregator.Count())

		// Add errors
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		aggregator.Add(err1)
		aggregator.Add(err2)

		assert.True(t, aggregator.HasErrors())
		assert.Equal(t, 2, aggregator.Count())

		// Test combined error message
		combinedMsg := aggregator.Error()
		assert.Contains(t, combinedMsg, "multiple errors occurred")
		assert.Contains(t, combinedMsg, "error 1")
		assert.Contains(t, combinedMsg, "error 2")

		// Test error retrieval
		errors := aggregator.Errors()
		assert.Len(t, errors, 2)
		assert.Equal(t, err1, errors[0])
		assert.Equal(t, err2, errors[1])
	})
}

// TestCriticalErrorHandling tests critical error handling scenarios
func TestCriticalErrorHandling(t *testing.T) {
	t.Run("critical failure handler", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Info", mock.AnythingOfType("string")).Return()
		mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

		shutdownManager := NewGracefulShutdownManager(mockLogger, 5*time.Second)
		failureHandler := NewCriticalFailureHandler(mockLogger, shutdownManager)

		// Track shutdown initiation
		shutdownInitiated := false
		shutdownManager.RegisterShutdownHandler(func() error {
			shutdownInitiated = true
			return nil
		})

		// Handle critical failure
		testError := errors.New("critical system failure")
		err := failureHandler.HandleCriticalFailure("test_operation", testError)

		// Should return an error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "critical failure")
		assert.Contains(t, err.Error(), "test_operation")

		// Should have initiated shutdown
		assert.True(t, shutdownInitiated)
	})

	t.Run("recovery manager", func(t *testing.T) {
		mockLogger := &MockLogger{}
		mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
		mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()

		recoveryManager := NewRecoveryManager(mockLogger)

		// Register recovery handler
		maxAttempts := 3
		attemptCount := 0
		recoveryManager.RegisterRecoveryHandler(ConnectionError, maxAttempts, func(err error) error {
			attemptCount++
			if attemptCount < maxAttempts {
				return errors.New("recovery failed")
			}
			return nil // Success on final attempt
		})

		// Attempt recovery
		testError := errors.New("connection refused")
		operation := "test_connection"

		// First attempts should fail
		err1 := recoveryManager.AttemptRecovery(operation, testError)
		assert.Error(t, err1)

		err2 := recoveryManager.AttemptRecovery(operation, testError)
		assert.Error(t, err2)

		// Final attempt should succeed
		err3 := recoveryManager.AttemptRecovery(operation, testError)
		assert.NoError(t, err3)

		// Verify attempts are tracked
		attempts := recoveryManager.GetRecoveryAttempts(operation)
		assert.Len(t, attempts, 3)
		assert.False(t, attempts[0].Success)
		assert.False(t, attempts[1].Success)
		assert.True(t, attempts[2].Success)
	})

	t.Run("error classification", func(t *testing.T) {
		testCases := []struct {
			errorMsg     string
			expectedType ErrorType
		}{
			{"connection refused", ConnectionError},
			{"network timeout", NetworkError},
			{"authentication failed", AuthenticationError},
			{"invalid data format", DataError},
			{"invalid configuration", ConfigurationError},
			{"unknown error", CriticalError},
		}

		for _, tc := range testCases {
			t.Run(tc.errorMsg, func(t *testing.T) {
				err := errors.New(tc.errorMsg)
				errorType := ClassifyError(err)
				assert.Equal(t, tc.expectedType, errorType)
			})
		}
	})
}
