package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/mock"
)

// **Feature: redis-valkey-migration, Property 10: Retry Logic for Network Errors**
// **Validates: Requirements 6.3**
func TestProperty_RetryLogicForNetworkErrors(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10
	parameters.MaxSize = 5
	properties := gopter.NewProperties(parameters)

	properties.Property("retry logic should attempt exactly MaxAttempts+1 times for retryable errors", prop.ForAll(
		func(maxAttempts int) bool {
			// Ensure valid test parameters
			if maxAttempts < 1 || maxAttempts > 5 {
				maxAttempts = 3
			}

			// Create retry configuration
			retryConfig := RetryConfig{
				MaxAttempts:   maxAttempts,
				InitialDelay:  time.Millisecond, // Very short for testing
				MaxDelay:      10 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{
					"network",
					"timeout",
					"connection refused",
					"i/o timeout",
					"broken pipe",
				},
			}

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			// Test with network error that always fails
			attemptCount := 0
			err := recovery.WithRetry("network operation", func() error {
				attemptCount++
				return errors.New("network timeout") // Always fails with retryable error
			})

			// Should fail after exhausting all retries
			if err == nil {
				return false
			}

			// Should have made exactly MaxAttempts + 1 attempts (initial + retries)
			expectedAttempts := maxAttempts + 1
			if attemptCount != expectedAttempts {
				return false
			}

			return true
		},
		gen.IntRange(1, 3), // maxAttempts
	))

	properties.Property("retry logic should use exponential backoff delays", prop.ForAll(
		func(initialDelay int, backoffFactor float64) bool {
			// Ensure valid test parameters
			if initialDelay < 1 || initialDelay > 100 {
				initialDelay = 10 // milliseconds
			}
			if backoffFactor < 1.5 || backoffFactor > 5.0 {
				backoffFactor = 2.0
			}

			initialDelayDuration := time.Duration(initialDelay) * time.Millisecond
			maxDelay := 1000 * time.Millisecond

			retryConfig := RetryConfig{
				MaxAttempts:     3,
				InitialDelay:    initialDelayDuration,
				MaxDelay:        maxDelay,
				BackoffFactor:   backoffFactor,
				RetryableErrors: []string{"network error"},
			}

			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			// Test delay calculation
			delay1 := recovery.calculateDelay(1)
			delay2 := recovery.calculateDelay(2)
			delay3 := recovery.calculateDelay(3)

			// First delay should be initial delay
			if delay1 != initialDelayDuration {
				return false
			}

			// Second delay should be initial * backoffFactor
			expectedDelay2 := time.Duration(float64(initialDelayDuration) * backoffFactor)
			if expectedDelay2 > maxDelay {
				expectedDelay2 = maxDelay
			}
			if delay2 != expectedDelay2 {
				return false
			}

			// Third delay should be initial * backoffFactor^2
			expectedDelay3 := time.Duration(float64(initialDelayDuration) * backoffFactor * backoffFactor)
			if expectedDelay3 > maxDelay {
				expectedDelay3 = maxDelay
			}
			if delay3 != expectedDelay3 {
				return false
			}

			// Delays should be non-decreasing (due to exponential backoff)
			if delay1 > delay2 || delay2 > delay3 {
				// Only fail if we haven't hit the max delay cap
				if delay2 < maxDelay && delay3 < maxDelay {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),        // initialDelay in milliseconds
		gen.Float64Range(1.5, 4.0), // backoffFactor
	))

	properties.Property("retry logic should distinguish between retryable and non-retryable errors", prop.ForAll(
		func(errorType int) bool {
			// Define error types: 0 = retryable, 1 = non-retryable
			if errorType < 0 || errorType > 1 {
				errorType = 0
			}

			retryConfig := RetryConfig{
				MaxAttempts:   3,
				InitialDelay:  time.Millisecond,
				MaxDelay:      10 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{
					"network",
					"timeout",
					"connection refused",
				},
			}

			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			var testError error
			var expectedAttempts int

			if errorType == 0 {
				// Retryable error
				testError = errors.New("network timeout")
				expectedAttempts = retryConfig.MaxAttempts + 1 // Initial + retries
			} else {
				// Non-retryable error
				testError = errors.New("authentication failed")
				expectedAttempts = 1 // Should fail immediately
			}

			attemptCount := 0
			err := recovery.WithRetry("test operation", func() error {
				attemptCount++
				return testError
			})

			// Should always fail (since we never return nil)
			if err == nil {
				return false
			}

			// Should have made the expected number of attempts
			if attemptCount != expectedAttempts {
				return false
			}

			return true
		},
		gen.IntRange(0, 1), // errorType
	))

	properties.Property("retry logic should succeed if operation succeeds within retry limit", prop.ForAll(
		func(successAttempt int) bool {
			// Ensure valid test parameters
			if successAttempt < 1 || successAttempt > 4 {
				successAttempt = 2
			}

			retryConfig := RetryConfig{
				MaxAttempts:     3,
				InitialDelay:    time.Millisecond,
				MaxDelay:        10 * time.Millisecond,
				BackoffFactor:   2.0,
				RetryableErrors: []string{"network error"},
			}

			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			attemptCount := 0
			err := recovery.WithRetry("test operation", func() error {
				attemptCount++
				if attemptCount < successAttempt {
					return errors.New("network error") // Retryable error
				}
				return nil // Success
			})

			// If success attempt is within retry limit, should succeed
			if successAttempt <= retryConfig.MaxAttempts+1 {
				if err != nil {
					return false
				}
				if attemptCount != successAttempt {
					return false
				}
			} else {
				// If success attempt is beyond retry limit, should fail
				if err == nil {
					return false
				}
				if attemptCount != retryConfig.MaxAttempts+1 {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 5), // successAttempt
	))

	properties.TestingRun(t)
}
