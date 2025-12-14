package verifier

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// VerificationResult represents the result of a verification operation
type VerificationResult struct {
	Key        string
	DataType   string
	Success    bool
	ErrorMsg   string
	Mismatches []string
	Duration   time.Duration
}

// VerificationSummary contains overall verification statistics
type VerificationSummary struct {
	TotalKeys      int
	VerifiedKeys   int
	FailedKeys     int
	MismatchedKeys int
	Duration       time.Duration
	Results        []VerificationResult
}

// DataVerifier defines the interface for data verification operations
type DataVerifier interface {
	// VerifyKey verifies a single key between source and target databases
	VerifyKey(key string, source, target client.DatabaseClient) VerificationResult

	// VerifyAllKeys verifies all keys in the provided list
	VerifyAllKeys(keys []string, source, target client.DatabaseClient) VerificationSummary

	// VerifyKeyExists checks if a key exists in the target database
	VerifyKeyExists(key string, target client.DatabaseClient) bool

	// CompareKeyContent compares the content of a key between source and target
	CompareKeyContent(key string, source, target client.DatabaseClient) (bool, []string, error)
}

// migrationVerifier implements DataVerifier interface
type migrationVerifier struct {
	logger logger.Logger
}

// NewDataVerifier creates a new DataVerifier instance
func NewDataVerifier(logger logger.Logger) DataVerifier {
	return &migrationVerifier{
		logger: logger,
	}
}

// VerifyKey verifies a single key between source and target databases
func (v *migrationVerifier) VerifyKey(key string, source, target client.DatabaseClient) VerificationResult {
	startTime := time.Now()
	result := VerificationResult{
		Key:        key,
		Success:    false,
		Mismatches: []string{},
	}

	// Check if key exists in target
	exists, err := target.Exists(key)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("failed to check key existence in target: %v", err)
		result.Duration = time.Since(startTime)
		v.logVerificationResult(result)
		return result
	}

	if !exists {
		result.ErrorMsg = "key does not exist in target database"
		result.Duration = time.Since(startTime)
		v.logVerificationResult(result)
		return result
	}

	// Get key type from source
	sourceType, err := source.GetKeyType(key)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("failed to get key type from source: %v", err)
		result.Duration = time.Since(startTime)
		v.logVerificationResult(result)
		return result
	}

	// Get key type from target
	targetType, err := target.GetKeyType(key)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("failed to get key type from target: %v", err)
		result.Duration = time.Since(startTime)
		v.logVerificationResult(result)
		return result
	}

	result.DataType = sourceType

	// Compare key types
	if sourceType != targetType {
		result.Mismatches = append(result.Mismatches, fmt.Sprintf("type mismatch: source=%s, target=%s", sourceType, targetType))
	}

	// Only compare content if types match
	if sourceType == targetType {
		_, mismatches, err := v.CompareKeyContent(key, source, target)
		if err != nil {
			result.ErrorMsg = fmt.Sprintf("failed to compare key content: %v", err)
			result.Duration = time.Since(startTime)
			v.logVerificationResult(result)
			return result
		}
		result.Mismatches = append(result.Mismatches, mismatches...)
	}

	// Verification succeeds if types match and no mismatches
	result.Success = sourceType == targetType && len(result.Mismatches) == 0
	result.Duration = time.Since(startTime)

	v.logVerificationResult(result)
	return result
}

// VerifyAllKeys verifies all keys in the provided list
func (v *migrationVerifier) VerifyAllKeys(keys []string, source, target client.DatabaseClient) VerificationSummary {
	startTime := time.Now()
	summary := VerificationSummary{
		TotalKeys: len(keys),
		Results:   make([]VerificationResult, 0, len(keys)),
	}

	v.logger.Infof("Starting verification of %d keys", len(keys))

	for i, key := range keys {
		result := v.VerifyKey(key, source, target)
		summary.Results = append(summary.Results, result)

		if result.Success {
			summary.VerifiedKeys++
		} else {
			summary.FailedKeys++
			if len(result.Mismatches) > 0 {
				summary.MismatchedKeys++
			}
		}

		// Log progress every 1000 keys
		if (i+1)%1000 == 0 || i == len(keys)-1 {
			v.logger.Infof("Verification progress: %d/%d keys processed", i+1, len(keys))
		}
	}

	summary.Duration = time.Since(startTime)

	// Log summary
	v.logger.WithFields(map[string]interface{}{
		"total_keys":      summary.TotalKeys,
		"verified_keys":   summary.VerifiedKeys,
		"failed_keys":     summary.FailedKeys,
		"mismatched_keys": summary.MismatchedKeys,
		"duration":        summary.Duration.String(),
		"success_rate":    fmt.Sprintf("%.2f%%", float64(summary.VerifiedKeys)/float64(summary.TotalKeys)*100),
	}).Info("Verification completed")

	return summary
}

// VerifyKeyExists checks if a key exists in the target database
func (v *migrationVerifier) VerifyKeyExists(key string, target client.DatabaseClient) bool {
	exists, err := target.Exists(key)
	if err != nil {
		v.logger.Errorf("Failed to check existence of key %s: %v", key, err)
		return false
	}
	return exists
}

// CompareKeyContent compares the content of a key between source and target
func (v *migrationVerifier) CompareKeyContent(key string, source, target client.DatabaseClient) (bool, []string, error) {
	// Get values from both databases
	sourceValue, err := source.GetValue(key)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get source value: %w", err)
	}

	targetValue, err := target.GetValue(key)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get target value: %w", err)
	}

	// Get key type to determine comparison method
	keyType, err := source.GetKeyType(key)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get key type: %w", err)
	}

	// Compare based on data type
	switch keyType {
	case "string":
		return v.compareStringValues(sourceValue, targetValue)
	case "hash":
		return v.compareHashValues(sourceValue, targetValue)
	case "list":
		return v.compareListValues(sourceValue, targetValue)
	case "set":
		return v.compareSetValues(sourceValue, targetValue)
	case "zset":
		return v.compareSortedSetValues(sourceValue, targetValue)
	default:
		return false, []string{fmt.Sprintf("unsupported data type: %s", keyType)}, nil
	}
}

// compareStringValues compares string values
func (v *migrationVerifier) compareStringValues(source, target interface{}) (bool, []string, error) {
	sourceStr, ok := source.(string)
	if !ok {
		return false, []string{"source value is not a string"}, nil
	}

	targetStr, ok := target.(string)
	if !ok {
		return false, []string{"target value is not a string"}, nil
	}

	if sourceStr != targetStr {
		return false, []string{fmt.Sprintf("string content mismatch: source length=%d, target length=%d", len(sourceStr), len(targetStr))}, nil
	}

	return true, nil, nil
}

// compareHashValues compares hash values
func (v *migrationVerifier) compareHashValues(source, target interface{}) (bool, []string, error) {
	sourceHash, ok := source.(map[string]string)
	if !ok {
		return false, []string{"source value is not a hash"}, nil
	}

	targetHash, ok := target.(map[string]string)
	if !ok {
		return false, []string{"target value is not a hash"}, nil
	}

	var mismatches []string

	// Check if all source fields exist in target with same values
	for field, sourceVal := range sourceHash {
		if targetVal, exists := targetHash[field]; !exists {
			mismatches = append(mismatches, fmt.Sprintf("field '%s' missing in target", field))
		} else if sourceVal != targetVal {
			mismatches = append(mismatches, fmt.Sprintf("field '%s' value mismatch", field))
		}
	}

	// Check if target has extra fields
	for field := range targetHash {
		if _, exists := sourceHash[field]; !exists {
			mismatches = append(mismatches, fmt.Sprintf("extra field '%s' in target", field))
		}
	}

	return len(mismatches) == 0, mismatches, nil
}

// compareListValues compares list values (order matters)
func (v *migrationVerifier) compareListValues(source, target interface{}) (bool, []string, error) {
	sourceList, ok := source.([]string)
	if !ok {
		return false, []string{"source value is not a list"}, nil
	}

	targetList, ok := target.([]string)
	if !ok {
		return false, []string{"target value is not a list"}, nil
	}

	var mismatches []string

	if len(sourceList) != len(targetList) {
		mismatches = append(mismatches, fmt.Sprintf("list length mismatch: source=%d, target=%d", len(sourceList), len(targetList)))
	}

	// Compare elements up to the shorter length
	minLen := len(sourceList)
	if len(targetList) < minLen {
		minLen = len(targetList)
	}

	for i := 0; i < minLen; i++ {
		if sourceList[i] != targetList[i] {
			mismatches = append(mismatches, fmt.Sprintf("element at index %d mismatch", i))
		}
	}

	return len(mismatches) == 0, mismatches, nil
}

// compareSetValues compares set values (order doesn't matter)
func (v *migrationVerifier) compareSetValues(source, target interface{}) (bool, []string, error) {
	sourceSet, ok := source.([]string)
	if !ok {
		return false, []string{"source value is not a set"}, nil
	}

	targetSet, ok := target.([]string)
	if !ok {
		return false, []string{"target value is not a set"}, nil
	}

	// Convert to maps for easier comparison
	sourceMap := make(map[string]bool)
	for _, member := range sourceSet {
		sourceMap[member] = true
	}

	targetMap := make(map[string]bool)
	for _, member := range targetSet {
		targetMap[member] = true
	}

	var mismatches []string

	// Check for missing members in target
	for member := range sourceMap {
		if !targetMap[member] {
			mismatches = append(mismatches, fmt.Sprintf("member '%s' missing in target", member))
		}
	}

	// Check for extra members in target
	for member := range targetMap {
		if !sourceMap[member] {
			mismatches = append(mismatches, fmt.Sprintf("extra member '%s' in target", member))
		}
	}

	return len(mismatches) == 0, mismatches, nil
}

// compareSortedSetValues compares sorted set values (order and scores matter)
func (v *migrationVerifier) compareSortedSetValues(source, target interface{}) (bool, []string, error) {
	sourceZSet, ok := source.([]redis.Z)
	if !ok {
		return false, []string{"source value is not a sorted set"}, nil
	}

	targetZSet, ok := target.([]redis.Z)
	if !ok {
		return false, []string{"target value is not a sorted set"}, nil
	}

	// Convert to maps for easier comparison (member -> score)
	sourceMap := make(map[string]float64)
	for _, z := range sourceZSet {
		if member, ok := z.Member.(string); ok {
			sourceMap[member] = z.Score
		}
	}

	targetMap := make(map[string]float64)
	for _, z := range targetZSet {
		if member, ok := z.Member.(string); ok {
			targetMap[member] = z.Score
		}
	}

	var mismatches []string

	// Check for missing or mismatched members in target
	for member, sourceScore := range sourceMap {
		if targetScore, exists := targetMap[member]; !exists {
			mismatches = append(mismatches, fmt.Sprintf("member '%s' missing in target", member))
		} else if sourceScore != targetScore {
			mismatches = append(mismatches, fmt.Sprintf("member '%s' score mismatch: source=%.2f, target=%.2f", member, sourceScore, targetScore))
		}
	}

	// Check for extra members in target
	for member := range targetMap {
		if _, exists := sourceMap[member]; !exists {
			mismatches = append(mismatches, fmt.Sprintf("extra member '%s' in target", member))
		}
	}

	return len(mismatches) == 0, mismatches, nil
}

// logVerificationResult logs the result of a verification operation
func (v *migrationVerifier) logVerificationResult(result VerificationResult) {
	fields := map[string]interface{}{
		"key":       result.Key,
		"data_type": result.DataType,
		"success":   result.Success,
		"duration":  result.Duration.String(),
	}

	if result.ErrorMsg != "" {
		fields["error"] = result.ErrorMsg
	}

	if len(result.Mismatches) > 0 {
		fields["mismatches"] = result.Mismatches
	}

	if result.Success {
		v.logger.WithFields(fields).Debug("Key verification successful")
	} else {
		v.logger.WithFields(fields).Error("Key verification failed")
	}
}
