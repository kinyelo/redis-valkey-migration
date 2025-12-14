package verifier

import (
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

func TestVerifyKey_SuccessfulVerification(t *testing.T) {
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

	// Test data
	key := "test_string_key"
	value := "test_string_value"

	// Set up identical data in both clients
	sourceClient.data[key] = value
	sourceClient.keyTypes[key] = "string"
	targetClient.data[key] = value
	targetClient.keyTypes[key] = "string"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.True(t, result.Success, "Verification should succeed for identical data")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "string", result.DataType)
	assert.Empty(t, result.ErrorMsg, "Should have no error message")
	assert.Empty(t, result.Mismatches, "Should have no mismatches")
	assert.Greater(t, result.Duration, time.Duration(0), "Duration should be recorded")
}

func TestVerifyKey_KeyNotExistsInTarget(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data - only in source
	key := "missing_key"
	value := "test_value"

	sourceClient.data[key] = value
	sourceClient.keyTypes[key] = "string"
	// Don't add to target

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for missing key")
	assert.Equal(t, key, result.Key)
	assert.Contains(t, result.ErrorMsg, "key does not exist in target database")
	assert.Empty(t, result.Mismatches, "Should have no mismatches when key is missing")
}

func TestVerifyKey_TypeMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with type mismatch
	key := "type_mismatch_key"

	sourceClient.data[key] = "string_value"
	sourceClient.keyTypes[key] = "string"
	targetClient.data[key] = map[string]string{"field": "value"}
	targetClient.keyTypes[key] = "hash"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for type mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "string", result.DataType)
	assert.Len(t, result.Mismatches, 1, "Should have one mismatch")
	assert.Contains(t, result.Mismatches[0], "type mismatch")
}

func TestVerifyKey_StringContentMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with content mismatch
	key := "content_mismatch_key"

	sourceClient.data[key] = "source_value"
	sourceClient.keyTypes[key] = "string"
	targetClient.data[key] = "target_value"
	targetClient.keyTypes[key] = "string"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for content mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "string", result.DataType)
	assert.Len(t, result.Mismatches, 1, "Should have one mismatch")
	assert.Contains(t, result.Mismatches[0], "string content mismatch")
}

func TestVerifyKey_HashFieldMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with hash field mismatch
	key := "hash_mismatch_key"

	sourceHash := map[string]string{
		"field1": "value1",
		"field2": "value2",
	}
	targetHash := map[string]string{
		"field1": "value1",
		"field3": "value3", // Different field
	}

	sourceClient.data[key] = sourceHash
	sourceClient.keyTypes[key] = "hash"
	targetClient.data[key] = targetHash
	targetClient.keyTypes[key] = "hash"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for hash field mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "hash", result.DataType)
	assert.Greater(t, len(result.Mismatches), 0, "Should have mismatches")

	// Check for specific mismatch messages
	mismatchStr := fmt.Sprintf("%v", result.Mismatches)
	assert.Contains(t, mismatchStr, "field2")
	assert.Contains(t, mismatchStr, "field3")
}

func TestVerifyKey_ListOrderMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with list order mismatch
	key := "list_mismatch_key"

	sourceList := []string{"item1", "item2", "item3"}
	targetList := []string{"item1", "item3", "item2"} // Different order

	sourceClient.data[key] = sourceList
	sourceClient.keyTypes[key] = "list"
	targetClient.data[key] = targetList
	targetClient.keyTypes[key] = "list"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for list order mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "list", result.DataType)
	assert.Greater(t, len(result.Mismatches), 0, "Should have mismatches")

	// Check for mismatch at specific indices
	mismatchStr := fmt.Sprintf("%v", result.Mismatches)
	assert.Contains(t, mismatchStr, "index")
}

func TestVerifyKey_SetMemberMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with set member mismatch
	key := "set_mismatch_key"

	sourceSet := []string{"member1", "member2", "member3"}
	targetSet := []string{"member1", "member2", "member4"} // Different member

	sourceClient.data[key] = sourceSet
	sourceClient.keyTypes[key] = "set"
	targetClient.data[key] = targetSet
	targetClient.keyTypes[key] = "set"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for set member mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "set", result.DataType)
	assert.Greater(t, len(result.Mismatches), 0, "Should have mismatches")

	// Check for specific member mismatches
	mismatchStr := fmt.Sprintf("%v", result.Mismatches)
	assert.Contains(t, mismatchStr, "member3")
	assert.Contains(t, mismatchStr, "member4")
}

func TestVerifyKey_SortedSetScoreMismatch(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test data with sorted set score mismatch
	key := "zset_mismatch_key"

	sourceZSet := []redis.Z{
		{Score: 1.0, Member: "member1"},
		{Score: 2.0, Member: "member2"},
	}
	targetZSet := []redis.Z{
		{Score: 1.0, Member: "member1"},
		{Score: 3.0, Member: "member2"}, // Different score
	}

	sourceClient.data[key] = sourceZSet
	sourceClient.keyTypes[key] = "zset"
	targetClient.data[key] = targetZSet
	targetClient.keyTypes[key] = "zset"

	// Verify the key
	result := verifier.VerifyKey(key, sourceClient, targetClient)

	// Assertions
	assert.False(t, result.Success, "Verification should fail for sorted set score mismatch")
	assert.Equal(t, key, result.Key)
	assert.Equal(t, "zset", result.DataType)
	assert.Greater(t, len(result.Mismatches), 0, "Should have mismatches")

	// Check for score mismatch
	mismatchStr := fmt.Sprintf("%v", result.Mismatches)
	assert.Contains(t, mismatchStr, "score mismatch")
	assert.Contains(t, mismatchStr, "member2")
}

func TestVerifyAllKeys_MixedResults(t *testing.T) {
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
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Set up test data - mix of successful and failed verifications
	keys := []string{"success_key", "missing_key", "mismatch_key"}

	// Successful key
	sourceClient.data["success_key"] = "value"
	sourceClient.keyTypes["success_key"] = "string"
	targetClient.data["success_key"] = "value"
	targetClient.keyTypes["success_key"] = "string"

	// Missing key (only in source)
	sourceClient.data["missing_key"] = "value"
	sourceClient.keyTypes["missing_key"] = "string"

	// Mismatch key
	sourceClient.data["mismatch_key"] = "source_value"
	sourceClient.keyTypes["mismatch_key"] = "string"
	targetClient.data["mismatch_key"] = "target_value"
	targetClient.keyTypes["mismatch_key"] = "string"

	// Verify all keys
	summary := verifier.VerifyAllKeys(keys, sourceClient, targetClient)

	// Assertions
	assert.Equal(t, 3, summary.TotalKeys, "Should have 3 total keys")
	assert.Equal(t, 1, summary.VerifiedKeys, "Should have 1 verified key")
	assert.Equal(t, 2, summary.FailedKeys, "Should have 2 failed keys")
	assert.Equal(t, 1, summary.MismatchedKeys, "Should have 1 mismatched key")
	assert.Len(t, summary.Results, 3, "Should have 3 results")
	assert.Greater(t, summary.Duration, time.Duration(0), "Duration should be recorded")

	// Check individual results
	successResult := findResultByKey(summary.Results, "success_key")
	assert.NotNil(t, successResult, "Should find success result")
	assert.True(t, successResult.Success, "Success key should pass")

	missingResult := findResultByKey(summary.Results, "missing_key")
	assert.NotNil(t, missingResult, "Should find missing result")
	assert.False(t, missingResult.Success, "Missing key should fail")
	assert.Contains(t, missingResult.ErrorMsg, "does not exist")

	mismatchResult := findResultByKey(summary.Results, "mismatch_key")
	assert.NotNil(t, mismatchResult, "Should find mismatch result")
	assert.False(t, mismatchResult.Success, "Mismatch key should fail")
	assert.Greater(t, len(mismatchResult.Mismatches), 0, "Should have mismatches")
}

func TestVerifyKeyExists_Success(t *testing.T) {
	// Create mock client
	targetClient := &mockDatabaseClient{
		data:     make(map[string]interface{}),
		keyTypes: make(map[string]string),
	}

	// Create logger
	loggerConfig := logger.Config{
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Set up test data
	key := "existing_key"
	targetClient.data[key] = "value"

	// Test key existence
	exists := verifier.VerifyKeyExists(key, targetClient)

	// Assertions
	assert.True(t, exists, "Key should exist in target")
}

func TestVerifyKeyExists_Missing(t *testing.T) {
	// Create mock client
	targetClient := &mockDatabaseClient{
		data:     make(map[string]interface{}),
		keyTypes: make(map[string]string),
	}

	// Create logger
	loggerConfig := logger.Config{
		Level:  "error",
		Format: "text",
	}
	testLogger, err := logger.NewLogger(loggerConfig)
	require.NoError(t, err)

	// Create verifier
	verifier := NewDataVerifier(testLogger)

	// Test non-existing key
	exists := verifier.VerifyKeyExists("non_existing_key", targetClient)

	// Assertions
	assert.False(t, exists, "Key should not exist in target")
}

// Helper function to find a result by key
func findResultByKey(results []VerificationResult, key string) *VerificationResult {
	for _, result := range results {
		if result.Key == key {
			return &result
		}
	}
	return nil
}
