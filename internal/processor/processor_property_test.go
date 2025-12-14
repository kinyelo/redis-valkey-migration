package processor

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"

	"redis-valkey-migration/pkg/logger"
)

// MockDatabaseClient is a mock implementation of DatabaseClient for testing
type MockDatabaseClient struct {
	mock.Mock
	data map[string]interface{}
	ttls map[string]time.Duration
}

func NewMockDatabaseClient() *MockDatabaseClient {
	return &MockDatabaseClient{
		data: make(map[string]interface{}),
		ttls: make(map[string]time.Duration),
	}
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
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, nil
}

func (m *MockDatabaseClient) SetValue(key string, value interface{}) error {
	m.data[key] = value
	return nil
}

func (m *MockDatabaseClient) Exists(key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockDatabaseClient) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabaseClient) GetTTL(key string) (time.Duration, error) {
	if ttl, exists := m.ttls[key]; exists {
		return ttl, nil
	}
	return -1, nil // No TTL
}

func (m *MockDatabaseClient) SetTTL(key string, ttl time.Duration) error {
	m.ttls[key] = ttl
	return nil
}

// MockLogger is a mock implementation of Logger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(args ...interface{})                              { m.Called(args...) }
func (m *MockLogger) Info(args ...interface{})                               { m.Called(args...) }
func (m *MockLogger) Warn(args ...interface{})                               { m.Called(args...) }
func (m *MockLogger) Error(args ...interface{})                              { m.Called(args...) }
func (m *MockLogger) Fatal(args ...interface{})                              { m.Called(args...) }
func (m *MockLogger) Debugf(format string, args ...interface{})              { m.Called(format, args) }
func (m *MockLogger) Infof(format string, args ...interface{})               { m.Called(format, args) }
func (m *MockLogger) Warnf(format string, args ...interface{})               { m.Called(format, args) }
func (m *MockLogger) Errorf(format string, args ...interface{})              { m.Called(format, args) }
func (m *MockLogger) Fatalf(format string, args ...interface{})              { m.Called(format, args) }
func (m *MockLogger) WithField(key string, value interface{}) logger.Logger  { return m }
func (m *MockLogger) WithFields(fields map[string]interface{}) logger.Logger { return m }

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

// **Feature: redis-valkey-migration, Property 2: Complete Data Migration Round Trip**
// **Validates: Requirements 1.4, 5.1, 5.2, 5.3, 5.4, 5.5**
// For any Redis database with data objects, after migration to Valkey, every object should exist in Valkey with identical key names, data types, and content
func TestProperty_CompleteDataMigrationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for key names
	keyGen := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 50
	})

	// Generator for string values
	stringValueGen := gen.AlphaString()

	// Generator for hash values - simpler approach
	hashValueGen := gen.OneConstOf(
		map[string]string{"key1": "value1"},
		map[string]string{"key1": "value1", "key2": "value2"},
		map[string]string{"field": "data", "name": "test"},
	)

	// Generator for list values - simpler approach
	listValueGen := gen.OneConstOf(
		[]string{"item1"},
		[]string{"item1", "item2"},
		[]string{"a", "b", "c"},
	)

	// Generator for set values - simpler approach
	setValueGen := gen.OneConstOf(
		[]string{"member1"},
		[]string{"member1", "member2"},
		[]string{"a", "b", "c"},
	)

	// Generator for sorted set values - simpler approach
	zsetValueGen := gen.OneConstOf(
		[]redis.Z{{Score: 1.0, Member: "member1"}},
		[]redis.Z{{Score: 1.0, Member: "member1"}, {Score: 2.0, Member: "member2"}},
		[]redis.Z{{Score: 10.5, Member: "a"}, {Score: 20.0, Member: "b"}},
	)

	// Test string migration round trip
	properties.Property("string migration preserves data", prop.ForAll(
		func(key string, value string) bool {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Allow any logging calls
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[key] = value

			// Process the string
			err := processor.ProcessString(key, source, target)
			if err != nil {
				return false
			}

			// Verify target has the same data
			targetValue, exists := target.data[key]
			if !exists {
				return false
			}

			targetString, ok := targetValue.(string)
			if !ok {
				return false
			}

			return targetString == value
		},
		keyGen, stringValueGen,
	))

	// Test hash migration round trip
	properties.Property("hash migration preserves data", prop.ForAll(
		func(key string, value map[string]string) bool {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Allow any logging calls
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[key] = value

			// Process the hash
			err := processor.ProcessHash(key, source, target)
			if err != nil {
				return false
			}

			// Verify target has the same data
			targetValue, exists := target.data[key]
			if !exists {
				return false
			}

			targetHash, ok := targetValue.(map[string]string)
			if !ok {
				return false
			}

			// Compare maps
			if len(targetHash) != len(value) {
				return false
			}

			for k, v := range value {
				if targetHash[k] != v {
					return false
				}
			}

			return true
		},
		keyGen, hashValueGen,
	))

	// Test list migration round trip
	properties.Property("list migration preserves data and order", prop.ForAll(
		func(key string, value []string) bool {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Allow any logging calls
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[key] = value

			// Process the list
			err := processor.ProcessList(key, source, target)
			if err != nil {
				return false
			}

			// Verify target has the same data
			targetValue, exists := target.data[key]
			if !exists {
				return false
			}

			targetList, ok := targetValue.([]string)
			if !ok {
				return false
			}

			// Compare slices (order matters for lists)
			if len(targetList) != len(value) {
				return false
			}

			for i, v := range value {
				if targetList[i] != v {
					return false
				}
			}

			return true
		},
		keyGen, listValueGen,
	))

	// Test set migration round trip
	properties.Property("set migration preserves data", prop.ForAll(
		func(key string, value []string) bool {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Allow any logging calls
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[key] = value

			// Process the set
			err := processor.ProcessSet(key, source, target)
			if err != nil {
				return false
			}

			// Verify target has the same data
			targetValue, exists := target.data[key]
			if !exists {
				return false
			}

			targetSet, ok := targetValue.([]interface{})
			if !ok {
				return false
			}

			// Convert target back to []string for comparison
			targetStrings := make([]string, len(targetSet))
			for i, v := range targetSet {
				str, ok := v.(string)
				if !ok {
					return false
				}
				targetStrings[i] = str
			}

			// Compare sets (order doesn't matter, but all elements should be present)
			if len(targetStrings) != len(value) {
				return false
			}

			// Create maps for easier comparison
			sourceMap := make(map[string]bool)
			for _, v := range value {
				sourceMap[v] = true
			}

			targetMap := make(map[string]bool)
			for _, v := range targetStrings {
				targetMap[v] = true
			}

			// Check if all elements match
			for k := range sourceMap {
				if !targetMap[k] {
					return false
				}
			}

			return true
		},
		keyGen, setValueGen,
	))

	// Test sorted set migration round trip
	properties.Property("sorted set migration preserves data and scores", prop.ForAll(
		func(key string, value []redis.Z) bool {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Allow any logging calls
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[key] = value

			// Process the sorted set
			err := processor.ProcessSortedSet(key, source, target)
			if err != nil {
				return false
			}

			// Verify target has the same data
			targetValue, exists := target.data[key]
			if !exists {
				return false
			}

			targetZset, ok := targetValue.([]redis.Z)
			if !ok {
				return false
			}

			// Compare sorted sets
			if len(targetZset) != len(value) {
				return false
			}

			// Create maps for easier comparison (member -> score)
			sourceMap := make(map[string]float64)
			for _, z := range value {
				if member, ok := z.Member.(string); ok {
					sourceMap[member] = z.Score
				}
			}

			targetMap := make(map[string]float64)
			for _, z := range targetZset {
				if member, ok := z.Member.(string); ok {
					targetMap[member] = z.Score
				}
			}

			// Check if all elements and scores match
			for member, score := range sourceMap {
				if targetScore, exists := targetMap[member]; !exists || targetScore != score {
					return false
				}
			}

			return true
		},
		keyGen, zsetValueGen,
	))

	properties.TestingRun(t)
}
