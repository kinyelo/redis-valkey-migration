package engine

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/mock"

	"redis-valkey-migration/pkg/logger"
)

// **Feature: redis-valkey-migration, Property 9: Connection Recovery and Resume**
// **Validates: Requirements 6.1, 6.2, 6.5**
func TestProperty_ConnectionRecoveryAndResume(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10
	parameters.MaxSize = 5
	properties := gopter.NewProperties(parameters)

	properties.Property("connection recovery should retry retryable errors and eventually succeed", prop.ForAll(
		func(maxAttempts int) bool {
			// Ensure valid test parameters
			if maxAttempts < 1 || maxAttempts > 5 {
				maxAttempts = 3
			}

			// Create retry configuration with short delays for testing
			retryConfig := RetryConfig{
				MaxAttempts:   maxAttempts,
				InitialDelay:  time.Millisecond, // Very short for testing
				MaxDelay:      10 * time.Millisecond,
				BackoffFactor: 2.0,
				RetryableErrors: []string{
					"connection refused",
					"timeout",
					"network",
				},
			}

			// Create mock logger
			mockLogger := &MockLogger{}
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return()
			mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

			// Create connection recovery
			recovery := NewConnectionRecovery(retryConfig, mockLogger)

			// Test case: Retryable error that eventually succeeds
			attemptCount := 0
			err := recovery.WithRetry("test operation", func() error {
				attemptCount++
				if attemptCount < maxAttempts {
					return errors.New("connection refused") // Retryable error
				}
				return nil // Success on final attempt
			})

			// Should succeed after retries
			if err != nil {
				return false
			}

			// Should have made the expected number of attempts
			if attemptCount != maxAttempts {
				return false
			}

			return true
		},
		gen.IntRange(1, 3), // maxAttempts
	))

	properties.Property("resume state should track processed keys and avoid duplicates", prop.ForAll(
		func(totalKeys int, processedRatio float64) bool {
			// Ensure valid test parameters
			if totalKeys < 1 || totalKeys > 20 {
				totalKeys = 10
			}
			if processedRatio < 0.0 || processedRatio > 1.0 {
				processedRatio = 0.5
			}

			// Create resume state
			resumeState := NewResumeState()
			resumeState.TotalKeys = totalKeys

			// Process some keys
			processedCount := int(float64(totalKeys) * processedRatio)
			for i := 0; i < processedCount; i++ {
				key := fmt.Sprintf("key:%d", i)
				resumeState.MarkProcessed(key)
			}

			// Verify processed count
			if resumeState.GetProcessedCount() != processedCount {
				return false
			}

			// Verify processed keys are marked correctly
			for i := 0; i < processedCount; i++ {
				key := fmt.Sprintf("key:%d", i)
				if !resumeState.IsProcessed(key) {
					return false
				}
			}

			// Verify unprocessed keys are not marked
			for i := processedCount; i < totalKeys; i++ {
				key := fmt.Sprintf("key:%d", i)
				if resumeState.IsProcessed(key) {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),        // totalKeys
		gen.Float64Range(0.0, 1.0), // processedRatio
	))

	properties.TestingRun(t)
}

// MockDatabaseClient is a mock implementation of DatabaseClient for testing
type MockDatabaseClient struct {
	mock.Mock
}

func (m *MockDatabaseClient) Connect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabaseClient) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabaseClient) GetAllKeys() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDatabaseClient) GetKeyType(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *MockDatabaseClient) GetValue(key string) (interface{}, error) {
	args := m.Called(key)
	return args.Get(0), args.Error(1)
}

func (m *MockDatabaseClient) SetValue(key string, value interface{}) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockDatabaseClient) Exists(key string) (bool, error) {
	args := m.Called(key)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabaseClient) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabaseClient) GetTTL(key string) (time.Duration, error) {
	args := m.Called(key)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *MockDatabaseClient) SetTTL(key string, ttl time.Duration) error {
	args := m.Called(key, ttl)
	return args.Error(0)
}

// MockLogger is a mock implementation of Logger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Info(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Warn(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Fatal(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) WithField(key string, value interface{}) logger.Logger {
	args := m.Called(key, value)
	return args.Get(0).(logger.Logger)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) logger.Logger {
	args := m.Called(fields)
	return args.Get(0).(logger.Logger)
}

func (m *MockLogger) LogConnection(operation, host string, port int, database int, success bool, duration time.Duration) {
	m.Called(operation, host, port, database, success, duration)
}

func (m *MockLogger) LogKeyTransfer(key, dataType string, size int64, success bool, duration time.Duration, errorMsg string) {
	m.Called(key, dataType, size, success, duration, errorMsg)
}

func (m *MockLogger) LogProgress(totalKeys, processedKeys, failedKeys int, throughput float64) {
	m.Called(totalKeys, processedKeys, failedKeys, throughput)
}

func (m *MockLogger) LogError(operation, key, errorMsg, stackTrace string, retryAttempt int) {
	m.Called(operation, key, errorMsg, stackTrace, retryAttempt)
}

func (m *MockLogger) LogSummary(stats logger.MigrationStats) {
	m.Called(stats)
}
