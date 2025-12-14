package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/mock"
)

// **Feature: redis-valkey-migration, Property 8: Error Context Logging**
// **Validates: Requirements 3.4, 7.4**
func TestProperty_ErrorContextLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10
	parameters.MaxSize = 5
	properties := gopter.NewProperties(parameters)

	properties.Property("error context should include operation, key, error message, and retry attempt", prop.ForAll(
		func(operation string, key string, errorMsg string, retryAttempt int) bool {
			// Ensure valid test parameters
			if len(operation) == 0 {
				operation = "test_operation"
			}
			if len(key) == 0 {
				key = "test_key"
			}
			if len(errorMsg) == 0 {
				errorMsg = "test_error"
			}
			if retryAttempt < 0 || retryAttempt > 10 {
				retryAttempt = 1
			}

			// Create mock logger to capture log calls
			mockLogger := &MockLogger{}

			// Set up expectation to capture the LogError call
			var capturedOperation, capturedKey, capturedErrorMsg, capturedStackTrace string
			var capturedRetryAttempt int

			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).
				Run(func(args mock.Arguments) {
					capturedOperation = args.String(0)
					capturedKey = args.String(1)
					capturedErrorMsg = args.String(2)
					capturedStackTrace = args.String(3)
					capturedRetryAttempt = args.Int(4)
				}).Return()

			// Create error context and log it
			errorContext := NewErrorContext(operation, key, errorMsg)
			errorContext.WithStackTrace()
			errorContext.LogWithContext(mockLogger, retryAttempt)

			// Verify all expected fields were logged
			if capturedOperation != operation {
				return false
			}
			if capturedKey != key {
				return false
			}
			if capturedErrorMsg != errorMsg {
				return false
			}
			if capturedRetryAttempt != retryAttempt {
				return false
			}

			// Stack trace should be present and non-empty
			if len(capturedStackTrace) == 0 {
				return false
			}

			return true
		},
		gen.Const("test_operation"),     // operation
		gen.Const("test_key"),           // key
		gen.Const("test_error_message"), // errorMsg
		gen.IntRange(0, 5),              // retryAttempt
	))

	properties.Property("migration errors should be properly classified and logged", prop.ForAll(
		func(errorTypeIndex int, operation string) bool {
			// Ensure valid test parameters
			if errorTypeIndex < 0 || errorTypeIndex > 5 {
				errorTypeIndex = 0
			}
			if len(operation) == 0 {
				operation = "test_operation"
			}

			// Define test error messages for different types
			errorMessages := []string{
				"connection refused",      // ConnectionError
				"network timeout",         // NetworkError
				"authentication failed",   // AuthenticationError
				"invalid data format",     // DataError
				"invalid configuration",   // ConfigurationError
				"critical system failure", // CriticalError
			}

			expectedTypes := []ErrorType{
				ConnectionError,
				NetworkError,
				AuthenticationError,
				DataError,
				ConfigurationError,
				CriticalError,
			}

			errorMsg := errorMessages[errorTypeIndex]
			expectedType := expectedTypes[errorTypeIndex]

			// Create error and wrap it
			originalErr := errors.New(errorMsg)
			migrationErr := WrapError(originalErr, operation)

			// Verify error classification
			if migrationErr.Type != expectedType {
				return false
			}

			// Verify error contains operation context
			if migrationErr.Operation != operation {
				return false
			}

			// Verify error message is preserved
			if migrationErr.Message != errorMsg {
				return false
			}

			// Verify retryable classification is correct
			expectedRetryable := isRetryableByType(expectedType)
			if migrationErr.Retryable != expectedRetryable {
				return false
			}

			return true
		},
		gen.IntRange(0, 5),          // errorTypeIndex
		gen.Const("test_operation"), // operation
	))

	properties.Property("error aggregator should collect and report multiple errors with context", prop.ForAll(
		func(errorCount int) bool {
			// Ensure valid test parameters
			if errorCount < 1 || errorCount > 10 {
				errorCount = 5
			}

			aggregator := NewErrorAggregator()

			// Add multiple errors with different types
			for i := 0; i < errorCount; i++ {
				operation := fmt.Sprintf("operation_%d", i)
				key := fmt.Sprintf("key_%d", i)
				errorMsg := fmt.Sprintf("error_%d", i)

				err := NewMigrationError(ConnectionError, operation, errorMsg).WithKey(key)
				aggregator.Add(err)
			}

			// Verify error count
			if aggregator.Count() != errorCount {
				return false
			}

			// Verify aggregator has errors
			if !aggregator.HasErrors() {
				return false
			}

			// Verify error message contains information about all errors
			combinedMsg := aggregator.Error()
			if len(combinedMsg) == 0 {
				return false
			}

			// For multiple errors, should contain "multiple errors occurred"
			if errorCount > 1 {
				if !strings.Contains(combinedMsg, "multiple errors occurred") {
					return false
				}
			}

			// Verify all errors are retrievable
			errors := aggregator.Errors()
			if len(errors) != errorCount {
				return false
			}

			return true
		},
		gen.IntRange(1, 8), // errorCount
	))

	properties.Property("connection recovery should log comprehensive error context during failures", prop.ForAll(
		func(maxAttempts int) bool {
			// Ensure valid test parameters
			if maxAttempts < 1 || maxAttempts > 5 {
				maxAttempts = 3
			}

			// Create mock logger to capture all log calls
			mockLogger := &MockLogger{}

			var loggedErrors []struct {
				operation    string
				key          string
				errorMsg     string
				stackTrace   string
				retryAttempt int
			}

			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).
				Run(func(args mock.Arguments) {
					loggedErrors = append(loggedErrors, struct {
						operation    string
						key          string
						errorMsg     string
						stackTrace   string
						retryAttempt int
					}{
						operation:    args.String(0),
						key:          args.String(1),
						errorMsg:     args.String(2),
						stackTrace:   args.String(3),
						retryAttempt: args.Int(4),
					})
				}).Return()

			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()

			// Create retry configuration
			retryConfig := RetryConfig{
				MaxAttempts:     maxAttempts,
				InitialDelay:    1, // Very short for testing
				MaxDelay:        10,
				BackoffFactor:   2.0,
				RetryableErrors: []string{"connection refused"},
			}

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			// Execute operation that always fails with retryable error
			err := recovery.WithRetry("test_connection", func() error {
				return errors.New("connection refused")
			})

			// Should fail after exhausting retries
			if err == nil {
				return false
			}

			// Should have logged errors for each attempt (except the final success log)
			expectedLogCount := maxAttempts + 1 // Initial attempt + retries
			if len(loggedErrors) != expectedLogCount {
				return false
			}

			// Verify each logged error has proper context
			for i, loggedError := range loggedErrors {
				// Operation should be consistent
				if loggedError.operation != "test_connection" {
					return false
				}

				// Error message should be consistent
				if loggedError.errorMsg != "connection refused" {
					return false
				}

				// Retry attempt should match the attempt number
				if loggedError.retryAttempt != i {
					return false
				}

				// Stack trace should be empty for this test (not added by WithRetry)
				// This is expected behavior as WithRetry doesn't add stack traces
			}

			return true
		},
		gen.IntRange(1, 3), // maxAttempts
	))

	properties.TestingRun(t)
}
