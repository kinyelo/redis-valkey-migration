package scanner

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabaseClient is a mock implementation of client.DatabaseClient
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

func TestNewScanner(t *testing.T) {
	mockClient := &MockDatabaseClient{}
	scanner := NewScanner(mockClient)

	assert.NotNil(t, scanner)
	assert.Equal(t, mockClient, scanner.client)
	assert.NotNil(t, scanner.progress)
}

func TestScanAllKeys_EmptyDatabase(t *testing.T) {
	mockClient := &MockDatabaseClient{}
	mockClient.On("GetAllKeys").Return([]string{}, nil)

	scanner := NewScanner(mockClient)
	keyInfos, err := scanner.ScanAllKeys()

	assert.NoError(t, err)
	assert.Empty(t, keyInfos)
	assert.Equal(t, 0, scanner.GetProgress().TotalKeys)
	mockClient.AssertExpectations(t)
}

func TestScanAllKeys_WithKeys(t *testing.T) {
	mockClient := &MockDatabaseClient{}
	keys := []string{"key1", "key2", "key3"}

	mockClient.On("GetAllKeys").Return(keys, nil)
	mockClient.On("GetKeyType", "key1").Return("string", nil)
	mockClient.On("GetKeyType", "key2").Return("hash", nil)
	mockClient.On("GetKeyType", "key3").Return("list", nil)
	mockClient.On("GetTTL", "key1").Return(time.Duration(-1), nil)
	mockClient.On("GetTTL", "key2").Return(time.Duration(-1), nil)
	mockClient.On("GetTTL", "key3").Return(time.Duration(-1), nil)

	scanner := NewScanner(mockClient)
	keyInfos, err := scanner.ScanAllKeys()

	assert.NoError(t, err)
	assert.Len(t, keyInfos, 3)
	assert.Equal(t, 3, scanner.GetProgress().TotalKeys)
	assert.Equal(t, 3, scanner.GetProgress().ScannedKeys)
	assert.True(t, scanner.IsComplete())

	// Check key info details
	assert.Equal(t, "key1", keyInfos[0].Name)
	assert.Equal(t, "string", keyInfos[0].Type)
	assert.Equal(t, "key2", keyInfos[1].Name)
	assert.Equal(t, "hash", keyInfos[1].Type)
	assert.Equal(t, "key3", keyInfos[2].Name)
	assert.Equal(t, "list", keyInfos[2].Type)

	mockClient.AssertExpectations(t)
}

func TestGetKeysByType(t *testing.T) {
	scanner := NewScanner(nil)
	keyInfos := []KeyInfo{
		{Name: "str1", Type: "string"},
		{Name: "str2", Type: "string"},
		{Name: "hash1", Type: "hash"},
		{Name: "list1", Type: "list"},
	}

	keysByType := scanner.GetKeysByType(keyInfos)

	assert.Len(t, keysByType, 3)
	assert.Len(t, keysByType["string"], 2)
	assert.Len(t, keysByType["hash"], 1)
	assert.Len(t, keysByType["list"], 1)
}

func TestGetScanSummary(t *testing.T) {
	scanner := NewScanner(nil)
	keyInfos := []KeyInfo{
		{Name: "str1", Type: "string", Size: 100},
		{Name: "str2", Type: "string", Size: 150},
		{Name: "hash1", Type: "hash", Size: 200},
	}

	summary := scanner.GetScanSummary(keyInfos)

	assert.Equal(t, 3, summary.TotalKeys)
	assert.Equal(t, 2, summary.TypeCounts["string"])
	assert.Equal(t, 1, summary.TypeCounts["hash"])
	assert.Equal(t, int64(450), summary.TotalSize)
	assert.True(t, summary.ScanDuration > 0)
}

func TestGetProgressPercent(t *testing.T) {
	mockClient := &MockDatabaseClient{}
	scanner := NewScanner(mockClient)

	// Initially 0%
	assert.Equal(t, 0.0, scanner.GetProgressPercent())

	// Simulate progress
	scanner.progress.TotalKeys = 100
	scanner.progress.ScannedKeys = 25
	assert.Equal(t, 25.0, scanner.GetProgressPercent())

	scanner.progress.ScannedKeys = 100
	assert.Equal(t, 100.0, scanner.GetProgressPercent())
}

func TestEstimateKeySize(t *testing.T) {
	scanner := NewScanner(nil)

	tests := []struct {
		key      string
		keyType  string
		expected int64
	}{
		{"test", "string", 54},     // 4 (key) + 50 (string estimate)
		{"hash_key", "hash", 208},  // 8 (key) + 200 (hash estimate)
		{"list_key", "list", 108},  // 8 (key) + 100 (list estimate)
		{"set_key", "set", 107},    // 7 (key) + 100 (set estimate)
		{"zset_key", "zset", 158},  // 8 (key) + 150 (zset estimate)
		{"unknown", "unknown", 57}, // 7 (key) + 50 (default estimate)
	}

	for _, test := range tests {
		size := scanner.estimateKeySize(test.key, test.keyType)
		assert.Equal(t, test.expected, size, "Key: %s, Type: %s", test.key, test.keyType)
	}
}

// Test key enumeration with various data sets
func TestScanAllKeys_VariousDataSets(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		keyTypes map[string]string
		ttls     map[string]time.Duration
	}{
		{
			name: "mixed_data_types",
			keys: []string{"user:1", "session:abc", "cache:data", "counter:views", "sorted:scores"},
			keyTypes: map[string]string{
				"user:1":        "hash",
				"session:abc":   "string",
				"cache:data":    "string",
				"counter:views": "string",
				"sorted:scores": "zset",
			},
			ttls: map[string]time.Duration{
				"user:1":        -1,
				"session:abc":   300 * time.Second,
				"cache:data":    600 * time.Second,
				"counter:views": -1,
				"sorted:scores": -1,
			},
		},
		{
			name:     "large_dataset",
			keys:     generateTestKeys(100), // Reduced from 1000 for faster tests
			keyTypes: generateTestKeyTypes(100),
			ttls:     generateTestTTLs(100),
		},
		{
			name:     "single_key",
			keys:     []string{"single:key"},
			keyTypes: map[string]string{"single:key": "string"},
			ttls:     map[string]time.Duration{"single:key": -1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(test.keys, nil)

			for _, key := range test.keys {
				mockClient.On("GetKeyType", key).Return(test.keyTypes[key], nil)
				mockClient.On("GetTTL", key).Return(test.ttls[key], nil)
			}

			scanner := NewScanner(mockClient)
			keyInfos, err := scanner.ScanAllKeys()

			assert.NoError(t, err)
			assert.Len(t, keyInfos, len(test.keys))

			// Verify all keys are discovered with correct metadata
			keyInfoMap := make(map[string]KeyInfo)
			for _, ki := range keyInfos {
				keyInfoMap[ki.Name] = ki
			}

			for _, expectedKey := range test.keys {
				keyInfo, exists := keyInfoMap[expectedKey]
				assert.True(t, exists, "Key %s should be discovered", expectedKey)
				assert.Equal(t, test.keyTypes[expectedKey], keyInfo.Type)
				assert.Equal(t, test.ttls[expectedKey], keyInfo.TTL)
				assert.True(t, keyInfo.Size > 0, "Key size should be estimated")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// Test type detection for all supported types
func TestTypeDetection_AllSupportedTypes(t *testing.T) {
	supportedTypes := []string{"string", "hash", "list", "set", "zset"}

	for _, keyType := range supportedTypes {
		t.Run(fmt.Sprintf("type_%s", keyType), func(t *testing.T) {
			mockClient := &MockDatabaseClient{}
			key := fmt.Sprintf("test:%s", keyType)

			mockClient.On("GetAllKeys").Return([]string{key}, nil)
			mockClient.On("GetKeyType", key).Return(keyType, nil)
			mockClient.On("GetTTL", key).Return(time.Duration(-1), nil)

			scanner := NewScanner(mockClient)
			keyInfos, err := scanner.ScanAllKeys()

			assert.NoError(t, err)
			assert.Len(t, keyInfos, 1)
			assert.Equal(t, key, keyInfos[0].Name)
			assert.Equal(t, keyType, keyInfos[0].Type)

			mockClient.AssertExpectations(t)
		})
	}
}

// Test progress tracking during scanning
func TestProgressTracking_DuringScanning(t *testing.T) {
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	mockClient := &MockDatabaseClient{}

	mockClient.On("GetAllKeys").Return(keys, nil)
	for _, key := range keys {
		mockClient.On("GetKeyType", key).Return("string", nil)
		mockClient.On("GetTTL", key).Return(time.Duration(-1), nil)
	}

	scanner := NewScanner(mockClient)

	// Initial progress should be zero
	initialProgress := scanner.GetProgress()
	assert.Equal(t, 0, initialProgress.TotalKeys)
	assert.Equal(t, 0, initialProgress.ScannedKeys)
	assert.Equal(t, 0.0, scanner.GetProgressPercent())
	assert.False(t, scanner.IsComplete())

	// Scan all keys
	keyInfos, err := scanner.ScanAllKeys()
	assert.NoError(t, err)
	assert.Len(t, keyInfos, 5)

	// Final progress should show completion
	finalProgress := scanner.GetProgress()
	assert.Equal(t, 5, finalProgress.TotalKeys)
	assert.Equal(t, 5, finalProgress.ScannedKeys)
	assert.Equal(t, 100.0, scanner.GetProgressPercent())
	assert.True(t, scanner.IsComplete())
	assert.True(t, finalProgress.KeysPerSecond > 0)

	mockClient.AssertExpectations(t)
}

// Test error handling during scanning
func TestScanAllKeys_ErrorHandling(t *testing.T) {
	t.Run("get_all_keys_error", func(t *testing.T) {
		mockClient := &MockDatabaseClient{}
		mockClient.On("GetAllKeys").Return([]string{}, fmt.Errorf("connection error"))

		scanner := NewScanner(mockClient)
		keyInfos, err := scanner.ScanAllKeys()

		assert.Error(t, err)
		assert.Nil(t, keyInfos)
		assert.Contains(t, err.Error(), "failed to get all keys")
		mockClient.AssertExpectations(t)
	})

	t.Run("get_key_type_error", func(t *testing.T) {
		mockClient := &MockDatabaseClient{}
		keys := []string{"key1", "key2", "key3"}

		mockClient.On("GetAllKeys").Return(keys, nil)
		mockClient.On("GetKeyType", "key1").Return("string", nil)
		mockClient.On("GetKeyType", "key2").Return("", fmt.Errorf("type error"))
		mockClient.On("GetKeyType", "key3").Return("hash", nil)
		mockClient.On("GetTTL", "key1").Return(time.Duration(-1), nil)
		mockClient.On("GetTTL", "key3").Return(time.Duration(-1), nil)

		scanner := NewScanner(mockClient)
		keyInfos, err := scanner.ScanAllKeys()

		// Should continue despite individual key errors
		assert.NoError(t, err)
		assert.Len(t, keyInfos, 2) // Only successful keys
		mockClient.AssertExpectations(t)
	})

	t.Run("nil_client", func(t *testing.T) {
		scanner := NewScanner(nil)
		keyInfos, err := scanner.ScanAllKeys()

		assert.Error(t, err)
		assert.Nil(t, keyInfos)
		assert.Contains(t, err.Error(), "database client is nil")
	})
}

// Test scan summary generation
func TestGetScanSummary_Comprehensive(t *testing.T) {
	keyInfos := []KeyInfo{
		{Name: "str1", Type: "string", Size: 100, TTL: -1},
		{Name: "str2", Type: "string", Size: 150, TTL: 300 * time.Second},
		{Name: "hash1", Type: "hash", Size: 200, TTL: -1},
		{Name: "hash2", Type: "hash", Size: 250, TTL: -1},
		{Name: "list1", Type: "list", Size: 120, TTL: 600 * time.Second},
		{Name: "set1", Type: "set", Size: 80, TTL: -1},
		{Name: "zset1", Type: "zset", Size: 180, TTL: -1},
	}

	scanner := NewScanner(nil)
	summary := scanner.GetScanSummary(keyInfos)

	assert.Equal(t, 7, summary.TotalKeys)
	assert.Equal(t, 2, summary.TypeCounts["string"])
	assert.Equal(t, 2, summary.TypeCounts["hash"])
	assert.Equal(t, 1, summary.TypeCounts["list"])
	assert.Equal(t, 1, summary.TypeCounts["set"])
	assert.Equal(t, 1, summary.TypeCounts["zset"])
	assert.Equal(t, int64(1080), summary.TotalSize)
	assert.True(t, summary.ScanDuration > 0)
}

// Helper functions for generating test data
func generateTestKeys(count int) []string {
	keys := make([]string, count)
	for i := 0; i < count; i++ {
		keys[i] = fmt.Sprintf("test:key:%d", i)
	}
	return keys
}

func generateTestKeyTypes(count int) map[string]string {
	types := []string{"string", "hash", "list", "set", "zset"}
	keyTypes := make(map[string]string)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("test:key:%d", i)
		keyTypes[key] = types[i%len(types)]
	}
	return keyTypes
}

func generateTestTTLs(count int) map[string]time.Duration {
	ttls := make(map[string]time.Duration)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("test:key:%d", i)
		if i%3 == 0 {
			ttls[key] = -1 // No TTL
		} else {
			ttls[key] = time.Duration(i*60) * time.Second
		}
	}
	return ttls
}
