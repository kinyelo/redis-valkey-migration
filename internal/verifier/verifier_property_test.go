package verifier

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// **Feature: redis-valkey-migration, Property 4: Data Integrity Verification**
// **Validates: Requirements 1.5, 4.1, 4.2, 4.4**
func TestProperty_DataIntegrityVerification(t *testing.T) {
	// Run property test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Create mock clients
			sourceClient := &mockDatabaseClient{
				data:     make(map[string]interface{}),
				keyTypes: make(map[string]string),
			}
			targetClient := &mockDatabaseClient{
				data:     make(map[string]interface{}),
				keyTypes: make(map[string]string),
			}

			// Create logger
			loggerConfig := logger.Config{
				Level:  "error", // Reduce noise in tests
				Format: "text",
			}
			testLogger, err := logger.NewLogger(loggerConfig)
			require.NoError(t, err)

			// Create verifier
			verifier := NewDataVerifier(testLogger)

			// Generate random test data
			testData := generateRandomTestData()

			// Set up source client with test data
			for key, data := range testData {
				sourceClient.data[key] = data.value
				sourceClient.keyTypes[key] = data.dataType
			}

			// Simulate successful migration by copying data to target
			for key, data := range testData {
				targetClient.data[key] = data.value
				targetClient.keyTypes[key] = data.dataType
			}

			// Property: For any migrated object, verification should confirm
			// that the object exists in target with matching content
			keys := make([]string, 0, len(testData))
			for key := range testData {
				keys = append(keys, key)
			}

			summary := verifier.VerifyAllKeys(keys, sourceClient, targetClient)

			// All keys should verify successfully
			assert.Equal(t, len(testData), summary.TotalKeys, "Total keys should match test data size")
			assert.Equal(t, len(testData), summary.VerifiedKeys, "All keys should be verified successfully")
			assert.Equal(t, 0, summary.FailedKeys, "No keys should fail verification")
			assert.Equal(t, 0, summary.MismatchedKeys, "No keys should have mismatches")

			// Verify individual key verification
			for key := range testData {
				result := verifier.VerifyKey(key, sourceClient, targetClient)
				assert.True(t, result.Success, "Key %s should verify successfully", key)
				assert.Empty(t, result.Mismatches, "Key %s should have no mismatches", key)
				assert.Empty(t, result.ErrorMsg, "Key %s should have no error", key)
			}
		})
	}
}

// **Feature: redis-valkey-migration, Property 12: Verification Failure Reporting**
// **Validates: Requirements 4.3**
func TestProperty_VerificationFailureReporting(t *testing.T) {
	// Run property test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Create mock clients
			sourceClient := &mockDatabaseClient{
				data:     make(map[string]interface{}),
				keyTypes: make(map[string]string),
			}
			targetClient := &mockDatabaseClient{
				data:     make(map[string]interface{}),
				keyTypes: make(map[string]string),
			}

			// Create logger
			loggerConfig := logger.Config{
				Level:  "error", // Reduce noise in tests
				Format: "text",
			}
			testLogger, err := logger.NewLogger(loggerConfig)
			require.NoError(t, err)

			// Create verifier
			verifier := NewDataVerifier(testLogger)

			// Generate random test data
			testData := generateRandomTestData()

			// Set up source client with test data
			for key, data := range testData {
				sourceClient.data[key] = data.value
				sourceClient.keyTypes[key] = data.dataType
			}

			// Introduce random mismatches in target
			mismatchedKeys := make(map[string]bool)
			for key, data := range testData {
				if rand.Float32() < 0.3 { // 30% chance of mismatch
					mismatchedKeys[key] = true
					// Introduce different types of mismatches
					switch rand.Intn(4) {
					case 0:
						// Missing key - don't add to target at all
						// Don't set data or keyTypes
					case 1:
						// Type mismatch - force different type
						targetClient.data[key] = data.value
						if data.dataType == "string" {
							targetClient.keyTypes[key] = "hash"
						} else {
							targetClient.keyTypes[key] = "string"
						}
					case 2:
						// Content mismatch - different value, same type
						targetClient.data[key] = generateMismatchedValue(data.value, data.dataType)
						targetClient.keyTypes[key] = data.dataType
					case 3:
						// Partial content (for complex types)
						targetClient.data[key] = generatePartialValue(data.value, data.dataType)
						targetClient.keyTypes[key] = data.dataType
					}
				} else {
					// Correct data - exact copy
					targetClient.data[key] = data.value
					targetClient.keyTypes[key] = data.dataType
				}
			}

			// Property: For any data verification failure, the tool should report
			// the specific key name and detailed nature of the mismatch
			keys := make([]string, 0, len(testData))
			for key := range testData {
				keys = append(keys, key)
			}

			summary := verifier.VerifyAllKeys(keys, sourceClient, targetClient)

			// Check that failures are properly reported
			for _, result := range summary.Results {
				if mismatchedKeys[result.Key] {
					// This key should fail verification
					assert.False(t, result.Success, "Mismatched key %s should fail verification", result.Key)

					// Should have specific error information
					hasSpecificInfo := result.ErrorMsg != "" || len(result.Mismatches) > 0
					assert.True(t, hasSpecificInfo, "Failed key %s should have specific error information", result.Key)

					// Error messages should be descriptive
					if result.ErrorMsg != "" {
						assert.NotEmpty(t, result.ErrorMsg, "Error message should not be empty for key %s", result.Key)
					}

					// Mismatches should be detailed
					for _, mismatch := range result.Mismatches {
						assert.NotEmpty(t, mismatch, "Mismatch description should not be empty for key %s", result.Key)
					}
				} else {
					// This key should pass verification
					assert.True(t, result.Success, "Correct key %s should pass verification", result.Key)
					assert.Empty(t, result.ErrorMsg, "Correct key %s should have no error", result.Key)
					assert.Empty(t, result.Mismatches, "Correct key %s should have no mismatches", result.Key)
				}
			}

			// Summary should reflect the failures
			expectedFailed := len(mismatchedKeys)
			expectedVerified := len(testData) - expectedFailed

			assert.Equal(t, len(testData), summary.TotalKeys, "Total keys should match")
			assert.Equal(t, expectedVerified, summary.VerifiedKeys, "Verified keys count should be correct")
			assert.Equal(t, expectedFailed, summary.FailedKeys, "Failed keys count should be correct")
		})
	}
}

// testData represents a piece of test data with its type
type testData struct {
	value    interface{}
	dataType string
}

// generateRandomTestData creates random test data for different Redis types
func generateRandomTestData() map[string]testData {
	data := make(map[string]testData)

	// Generate 5-15 random keys
	numKeys := rand.Intn(11) + 5

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("test_key_%d_%d", i, rand.Intn(1000))

		// Randomly choose data type
		switch rand.Intn(5) {
		case 0: // String
			data[key] = testData{
				value:    generateRandomString(),
				dataType: "string",
			}
		case 1: // Hash
			data[key] = testData{
				value:    generateRandomHash(),
				dataType: "hash",
			}
		case 2: // List
			data[key] = testData{
				value:    generateRandomList(),
				dataType: "list",
			}
		case 3: // Set
			data[key] = testData{
				value:    generateRandomSet(),
				dataType: "set",
			}
		case 4: // Sorted Set
			data[key] = testData{
				value:    generateRandomSortedSet(),
				dataType: "zset",
			}
		}
	}

	return data
}

// generateRandomString creates a random string value
func generateRandomString() string {
	length := rand.Intn(100) + 1
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// generateRandomHash creates a random hash value
func generateRandomHash() map[string]string {
	hash := make(map[string]string)
	numFields := rand.Intn(10) + 1

	for i := 0; i < numFields; i++ {
		field := fmt.Sprintf("field_%d", i)
		value := generateRandomString()
		hash[field] = value
	}

	return hash
}

// generateRandomList creates a random list value
func generateRandomList() []string {
	length := rand.Intn(20) + 1
	list := make([]string, length)

	for i := range list {
		list[i] = generateRandomString()
	}

	return list
}

// generateRandomSet creates a random set value
func generateRandomSet() []string {
	length := rand.Intn(15) + 1
	set := make([]string, length)

	for i := range set {
		set[i] = fmt.Sprintf("member_%d_%s", i, generateRandomString())
	}

	return set
}

// generateRandomSortedSet creates a random sorted set value
func generateRandomSortedSet() []redis.Z {
	length := rand.Intn(15) + 1
	zset := make([]redis.Z, length)

	for i := range zset {
		zset[i] = redis.Z{
			Score:  rand.Float64() * 100,
			Member: fmt.Sprintf("member_%d_%s", i, generateRandomString()),
		}
	}

	return zset
}

// generateMismatchedValue creates a value that doesn't match the original
func generateMismatchedValue(original interface{}, dataType string) interface{} {
	switch dataType {
	case "string":
		return generateRandomString() // Different string
	case "hash":
		// Return hash with different content
		hash := make(map[string]string)
		hash["different_field"] = "different_value"
		return hash
	case "list":
		// Return list with different content
		return []string{"different", "list", "content"}
	case "set":
		// Return set with different content
		return []string{"different", "set", "members"}
	case "zset":
		// Return sorted set with different content
		return []redis.Z{
			{Score: 1.0, Member: "different_member"},
		}
	default:
		return "mismatched_value"
	}
}

// generatePartialValue creates a value that's partially different (for complex types)
func generatePartialValue(original interface{}, dataType string) interface{} {
	switch dataType {
	case "hash":
		if originalHash, ok := original.(map[string]string); ok {
			// Remove one field
			partial := make(map[string]string)
			count := 0
			for k, v := range originalHash {
				if count > 0 { // Skip first field
					partial[k] = v
				}
				count++
			}
			return partial
		}
	case "list":
		if originalList, ok := original.([]string); ok && len(originalList) > 1 {
			// Return list with one less element
			return originalList[:len(originalList)-1]
		}
	case "set":
		if originalSet, ok := original.([]string); ok && len(originalSet) > 1 {
			// Return set with one less member
			return originalSet[:len(originalSet)-1]
		}
	case "zset":
		if originalZSet, ok := original.([]redis.Z); ok && len(originalZSet) > 1 {
			// Return sorted set with one less member
			return originalZSet[:len(originalZSet)-1]
		}
	}

	// Fallback to completely different value
	return generateMismatchedValue(original, dataType)
}

// mockDatabaseClient implements client.DatabaseClient for testing
type mockDatabaseClient struct {
	data     map[string]interface{}
	keyTypes map[string]string
}

func (m *mockDatabaseClient) Connect() error {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	if m.keyTypes == nil {
		m.keyTypes = make(map[string]string)
	}
	return nil
}

func (m *mockDatabaseClient) Disconnect() error {
	return nil
}

func (m *mockDatabaseClient) GetAllKeys() ([]string, error) {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *mockDatabaseClient) GetKeysByPattern(pattern string) ([]string, error) {
	// Simple pattern matching for testing - just return all keys for now
	return m.GetAllKeys()
}

func (m *mockDatabaseClient) GetKeyType(key string) (string, error) {
	if keyType, exists := m.keyTypes[key]; exists {
		return keyType, nil
	}
	return "", fmt.Errorf("key not found: %s", key)
}

func (m *mockDatabaseClient) GetValue(key string) (interface{}, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *mockDatabaseClient) SetValue(key string, value interface{}) error {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
	return nil
}

func (m *mockDatabaseClient) Exists(key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *mockDatabaseClient) Ping() error {
	return nil
}

func (m *mockDatabaseClient) GetTTL(key string) (time.Duration, error) {
	return -1, nil // No TTL
}

func (m *mockDatabaseClient) SetTTL(key string, ttl time.Duration) error {
	return nil
}
