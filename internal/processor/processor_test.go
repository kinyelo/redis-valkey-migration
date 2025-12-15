package processor

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test string value processing
func TestProcessString(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		sourceValue string
		sourceTTL   time.Duration
		expectError bool
	}{
		{
			name:        "successful string migration",
			key:         "test:string",
			sourceValue: "hello world",
			sourceTTL:   -1, // No TTL
			expectError: false,
		},
		{
			name:        "string with TTL",
			key:         "test:string:ttl",
			sourceValue: "expiring value",
			sourceTTL:   300 * time.Second,
			expectError: false,
		},
		{
			name:        "empty string",
			key:         "test:empty",
			sourceValue: "",
			sourceTTL:   -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			// Set up expectations
			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[tt.key] = tt.sourceValue
			source.ttls[tt.key] = tt.sourceTTL

			// Execute
			err := processor.ProcessString(tt.key, source, target)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check target has the value
				targetValue, exists := target.data[tt.key]
				assert.True(t, exists)
				assert.Equal(t, tt.sourceValue, targetValue)

				// Check TTL if set
				if tt.sourceTTL > 0 {
					targetTTL, exists := target.ttls[tt.key]
					assert.True(t, exists)
					assert.Equal(t, tt.sourceTTL, targetTTL)
				}
			}
		})
	}
}

// Test string processing with source error
func TestProcessString_SourceError(t *testing.T) {
	source := NewMockDatabaseClient()
	target := NewMockDatabaseClient()
	mockLogger := &MockLogger{}

	mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	processor := NewDataProcessor(mockLogger)

	// Don't set up any data in source, so GetValue will return nil, nil
	err := processor.ProcessString("nonexistent", source, target)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected string value")
}

// Test hash field processing
func TestProcessHash(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		sourceValue map[string]string
		sourceTTL   time.Duration
		expectError bool
	}{
		{
			name: "successful hash migration",
			key:  "test:hash",
			sourceValue: map[string]string{
				"field1": "value1",
				"field2": "value2",
			},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name: "hash with TTL",
			key:  "test:hash:ttl",
			sourceValue: map[string]string{
				"name": "test",
				"type": "hash",
			},
			sourceTTL:   600 * time.Second,
			expectError: false,
		},
		{
			name:        "empty hash",
			key:         "test:empty:hash",
			sourceValue: map[string]string{},
			sourceTTL:   -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[tt.key] = tt.sourceValue
			source.ttls[tt.key] = tt.sourceTTL

			// Execute
			err := processor.ProcessHash(tt.key, source, target)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check target has the value
				targetValue, exists := target.data[tt.key]
				assert.True(t, exists)
				assert.Equal(t, tt.sourceValue, targetValue)

				// Check TTL if set
				if tt.sourceTTL > 0 {
					targetTTL, exists := target.ttls[tt.key]
					assert.True(t, exists)
					assert.Equal(t, tt.sourceTTL, targetTTL)
				}
			}
		})
	}
}

// Test list element processing with order preservation
func TestProcessList(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		sourceValue []string
		sourceTTL   time.Duration
		expectError bool
	}{
		{
			name:        "successful list migration",
			key:         "test:list",
			sourceValue: []string{"item1", "item2", "item3"},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name:        "list with TTL",
			key:         "test:list:ttl",
			sourceValue: []string{"a", "b", "c", "d"},
			sourceTTL:   900 * time.Second,
			expectError: false,
		},
		{
			name:        "single item list",
			key:         "test:single",
			sourceValue: []string{"only"},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name:        "empty list",
			key:         "test:empty:list",
			sourceValue: []string{},
			sourceTTL:   -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[tt.key] = tt.sourceValue
			source.ttls[tt.key] = tt.sourceTTL

			// Execute
			err := processor.ProcessList(tt.key, source, target)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check target has the value
				targetValue, exists := target.data[tt.key]
				assert.True(t, exists)

				// Verify order is preserved
				targetList, ok := targetValue.([]string)
				assert.True(t, ok)
				assert.Equal(t, len(tt.sourceValue), len(targetList))

				for i, item := range tt.sourceValue {
					assert.Equal(t, item, targetList[i], "Order should be preserved at index %d", i)
				}

				// Check TTL if set
				if tt.sourceTTL > 0 {
					targetTTL, exists := target.ttls[tt.key]
					assert.True(t, exists)
					assert.Equal(t, tt.sourceTTL, targetTTL)
				}
			}
		})
	}
}

// Test set member processing
func TestProcessSet(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		sourceValue []string
		sourceTTL   time.Duration
		expectError bool
	}{
		{
			name:        "successful set migration",
			key:         "test:set",
			sourceValue: []string{"member1", "member2", "member3"},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name:        "set with TTL",
			key:         "test:set:ttl",
			sourceValue: []string{"a", "b", "c"},
			sourceTTL:   1200 * time.Second,
			expectError: false,
		},
		{
			name:        "single member set",
			key:         "test:single:set",
			sourceValue: []string{"only"},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name:        "empty set",
			key:         "test:empty:set",
			sourceValue: []string{},
			sourceTTL:   -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[tt.key] = tt.sourceValue
			source.ttls[tt.key] = tt.sourceTTL

			// Execute
			err := processor.ProcessSet(tt.key, source, target)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check target has the value
				targetValue, exists := target.data[tt.key]
				assert.True(t, exists)

				// Verify all members are present (order doesn't matter for sets)
				targetSet, ok := targetValue.([]interface{})
				assert.True(t, ok)
				assert.Equal(t, len(tt.sourceValue), len(targetSet))

				// Convert to map for easier comparison
				sourceMap := make(map[string]bool)
				for _, member := range tt.sourceValue {
					sourceMap[member] = true
				}

				targetMap := make(map[string]bool)
				for _, member := range targetSet {
					str, ok := member.(string)
					assert.True(t, ok)
					targetMap[str] = true
				}

				assert.Equal(t, sourceMap, targetMap)

				// Check TTL if set
				if tt.sourceTTL > 0 {
					targetTTL, exists := target.ttls[tt.key]
					assert.True(t, exists)
					assert.Equal(t, tt.sourceTTL, targetTTL)
				}
			}
		})
	}
}

// Test sorted set processing with scores
func TestProcessSortedSet(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		sourceValue []redis.Z
		sourceTTL   time.Duration
		expectError bool
	}{
		{
			name: "successful sorted set migration",
			key:  "test:zset",
			sourceValue: []redis.Z{
				{Score: 1.0, Member: "member1"},
				{Score: 2.0, Member: "member2"},
				{Score: 3.0, Member: "member3"},
			},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name: "sorted set with TTL",
			key:  "test:zset:ttl",
			sourceValue: []redis.Z{
				{Score: 10.5, Member: "a"},
				{Score: 20.0, Member: "b"},
			},
			sourceTTL:   1800 * time.Second,
			expectError: false,
		},
		{
			name: "single member sorted set",
			key:  "test:single:zset",
			sourceValue: []redis.Z{
				{Score: 100.0, Member: "only"},
			},
			sourceTTL:   -1,
			expectError: false,
		},
		{
			name:        "empty sorted set",
			key:         "test:empty:zset",
			sourceValue: []redis.Z{},
			sourceTTL:   -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewMockDatabaseClient()
			target := NewMockDatabaseClient()
			mockLogger := &MockLogger{}

			mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()

			processor := NewDataProcessor(mockLogger)

			// Set up source data
			source.data[tt.key] = tt.sourceValue
			source.ttls[tt.key] = tt.sourceTTL

			// Execute
			err := processor.ProcessSortedSet(tt.key, source, target)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check target has the value
				targetValue, exists := target.data[tt.key]
				assert.True(t, exists)

				// Verify all members and scores are present
				targetZset, ok := targetValue.([]redis.Z)
				assert.True(t, ok)
				assert.Equal(t, len(tt.sourceValue), len(targetZset))

				// Convert to maps for easier comparison
				sourceMap := make(map[string]float64)
				for _, z := range tt.sourceValue {
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

				assert.Equal(t, sourceMap, targetMap)

				// Check TTL if set
				if tt.sourceTTL > 0 {
					targetTTL, exists := target.ttls[tt.key]
					assert.True(t, exists)
					assert.Equal(t, tt.sourceTTL, targetTTL)
				}
			}
		})
	}
}

// Test error handling when source returns wrong type
func TestProcessWrongType(t *testing.T) {
	source := NewMockDatabaseClient()
	target := NewMockDatabaseClient()
	mockLogger := &MockLogger{}

	mockLogger.On("LogKeyTransfer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	processor := NewDataProcessor(mockLogger)

	// Test ProcessString with wrong type
	source.data["test"] = 123 // int instead of string
	err := processor.ProcessString("test", source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected string value")

	// Test ProcessHash with wrong type
	source.data["test"] = "not a hash"
	err = processor.ProcessHash("test", source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected map[string]string value")

	// Test ProcessList with wrong type
	source.data["test"] = map[string]string{"not": "list"}
	err = processor.ProcessList("test", source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected []string value")

	// Test ProcessSet with wrong type
	source.data["test"] = 42
	err = processor.ProcessSet("test", source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected []string value")

	// Test ProcessSortedSet with wrong type
	source.data["test"] = []string{"not", "zset"}
	err = processor.ProcessSortedSet("test", source, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected []redis.Z value")
}

// Test processing key with "none" type (non-existent key)
func TestProcessKey_NoneType(t *testing.T) {
	source := NewMockDatabaseClient()
	target := NewMockDatabaseClient()
	mockLogger := &MockLogger{}

	// Set up expectation for the warning log
	mockLogger.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return()

	processor := NewDataProcessor(mockLogger)

	// Test processing a key with "none" type (key doesn't exist)
	err := processor.ProcessKey("redisson__execute_task_once_latch:{mfa_sessions_params}", "none", source, target)

	// Should not return an error - key should be skipped
	assert.NoError(t, err)

	// Verify that the warning was logged
	mockLogger.AssertCalled(t, "Warnf", mock.MatchedBy(func(format string) bool {
		return assert.Contains(t, format, "no longer exists") &&
			assert.Contains(t, format, "type: none") &&
			assert.Contains(t, format, "Skipping migration")
	}), mock.Anything)
}
