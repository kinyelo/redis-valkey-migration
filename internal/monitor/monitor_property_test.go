package monitor

import (
	"errors"
	"testing"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func createTestLogger() logger.Logger {
	testLogger, _ := logger.NewLogger(logger.Config{Level: "info"})
	return testLogger
}

// **Feature: redis-valkey-migration, Property 6: Progress Tracking Accuracy**
// **Validates: Requirements 3.1, 3.2, 3.5**
// For any migration session, the progress reporting should accurately reflect the total count, processed count, and remaining count throughout the migration process
func TestProperty_ProgressTrackingAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for total keys count
	totalKeysGen := gen.IntRange(1, 10000)
	// Generator for successful operations count
	successCountGen := gen.IntRange(0, 100)
	// Generator for failed operations count
	failureCountGen := gen.IntRange(0, 50)
	// Generator for bytes transferred per operation
	bytesGen := gen.Int64Range(1, 1024*1024) // 1 byte to 1MB per key

	properties.Property("progress tracking maintains accurate counts throughout migration", prop.ForAll(
		func(totalKeys int, successOps int, failureOps int, bytesPerOp int64) bool {
			// Ensure we don't exceed total keys
			if successOps+failureOps > totalKeys {
				return true // Skip invalid combinations
			}

			monitor := NewProgressMonitor(createTestLogger())
			monitor.Start(totalKeys)

			// Verify initial state
			processed, total, failed, percentage := monitor.GetProgress()
			if processed != 0 || total != totalKeys || failed != 0 || percentage != 0 {
				return false
			}

			// Record successful operations
			for i := 0; i < successOps; i++ {
				key := generateTestKey(i)
				monitor.RecordSuccess(key, bytesPerOp)
			}

			// Record failed operations
			for i := 0; i < failureOps; i++ {
				key := generateTestKey(successOps + i)
				monitor.RecordFailure(key, errors.New("test error"))
			}

			// Verify progress tracking accuracy
			processed, total, failed, percentage = monitor.GetProgress()
			expectedProcessed := successOps + failureOps
			expectedPercentage := float64(expectedProcessed) / float64(totalKeys) * 100

			// Check all counts are accurate
			if processed != expectedProcessed {
				return false
			}
			if total != totalKeys {
				return false
			}
			if failed != failureOps {
				return false
			}
			if percentage != expectedPercentage {
				return false
			}

			// Verify statistics accuracy
			stats := monitor.GetStatistics()
			if stats.TotalKeys != totalKeys {
				return false
			}
			if stats.SuccessfulKeys != successOps {
				return false
			}
			if stats.FailedKeys != failureOps {
				return false
			}
			if stats.BytesTransferred != int64(successOps)*bytesPerOp {
				return false
			}

			// Verify remaining count calculation
			remaining := total - processed
			expectedRemaining := totalKeys - expectedProcessed
			if remaining != expectedRemaining {
				return false
			}

			return true
		},
		totalKeysGen, successCountGen, failureCountGen, bytesGen,
	))

	properties.Property("progress percentage calculation is always accurate", prop.ForAll(
		func(totalKeys int, processedKeys int) bool {
			if totalKeys <= 0 || processedKeys < 0 || processedKeys > totalKeys {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())
			monitor.Start(totalKeys)

			// Simulate processing keys
			for i := 0; i < processedKeys; i++ {
				key := generateTestKey(i)
				monitor.RecordSuccess(key, 100) // 100 bytes per key
			}

			_, _, _, percentage := monitor.GetProgress()
			expectedPercentage := float64(processedKeys) / float64(totalKeys) * 100

			// Allow for floating point precision differences
			return abs(percentage-expectedPercentage) < 0.001
		},
		gen.IntRange(1, 1000),
		gen.IntRange(0, 1000),
	))

	properties.Property("statistics remain consistent throughout migration lifecycle", prop.ForAll(
		func(totalKeys int, operations []bool) bool { // true = success, false = failure
			if totalKeys <= 0 || len(operations) > totalKeys {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())

			// Verify initial state
			if monitor.GetStatus() != StatusNotStarted {
				return false
			}

			monitor.Start(totalKeys)

			// Verify running state
			if monitor.GetStatus() != StatusRunning {
				return false
			}

			successCount := 0
			failureCount := 0
			totalBytes := int64(0)

			// Process operations
			for i, isSuccess := range operations {
				key := generateTestKey(i)
				if isSuccess {
					bytesToTransfer := int64(100 + i) // Variable bytes per key
					monitor.RecordSuccess(key, bytesToTransfer)
					successCount++
					totalBytes += bytesToTransfer
				} else {
					monitor.RecordFailure(key, errors.New("test failure"))
					failureCount++
				}

				// Verify statistics consistency at each step
				stats := monitor.GetStatistics()
				if stats.SuccessfulKeys != successCount {
					return false
				}
				if stats.FailedKeys != failureCount {
					return false
				}
				if stats.BytesTransferred != totalBytes {
					return false
				}
				if stats.TotalKeys != totalKeys {
					return false
				}

				// Verify processed count matches sum of success and failure
				processed, _, _, _ := monitor.GetProgress()
				if processed != successCount+failureCount {
					return false
				}
			}

			// Complete the migration
			monitor.Complete()

			// Verify final state
			if monitor.GetStatus() != StatusCompleted {
				return false
			}

			// Verify final statistics
			finalStats := monitor.GetStatistics()
			if finalStats.SuccessfulKeys != successCount {
				return false
			}
			if finalStats.FailedKeys != failureCount {
				return false
			}
			if finalStats.BytesTransferred != totalBytes {
				return false
			}
			if finalStats.Duration <= 0 {
				return false
			}

			return true
		},
		gen.IntRange(1, 100),
		gen.SliceOf(gen.Bool()),
	))

	properties.TestingRun(t)
}

// **Feature: redis-valkey-migration, Property 19: Filtered Progress Reporting**
// **Validates: Requirements 9.5**
func TestProperty_FilteredProgressReporting(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("filtered migration progress is based on filtered key set size", prop.ForAll(
		func(totalKeysInDB int, filteredKeyCount int) bool {
			if totalKeysInDB <= 0 || filteredKeyCount < 0 || filteredKeyCount > totalKeysInDB {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())

			// Initialize with filtered key count (not total DB size)
			monitor.Initialize(filteredKeyCount)

			// Verify initial state uses filtered count
			processed, total, failed, percentage := monitor.GetProgress()
			if total != filteredKeyCount {
				return false
			}
			if processed != 0 || failed != 0 || percentage != 0 {
				return false
			}

			// Verify statistics use filtered count
			stats := monitor.GetStats()
			if stats.TotalKeys != filteredKeyCount {
				return false
			}

			return true
		},
		gen.IntRange(1, 10000), // Total keys in database
		gen.IntRange(0, 1000),  // Filtered key count
	))

	properties.Property("progress percentage calculation uses filtered key set", prop.ForAll(
		func(filteredKeyCount int, processedKeys int) bool {
			if filteredKeyCount <= 0 || processedKeys < 0 || processedKeys > filteredKeyCount {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())
			monitor.Initialize(filteredKeyCount)

			// Process some keys from the filtered set
			for i := 0; i < processedKeys; i++ {
				monitor.IncrementProcessed()
			}

			_, total, _, percentage := monitor.GetProgress()
			expectedPercentage := float64(processedKeys) / float64(filteredKeyCount) * 100

			// Progress should be based on filtered set, not entire database
			if total != filteredKeyCount {
				return false
			}

			// Percentage should be calculated against filtered set
			if abs(percentage-expectedPercentage) > 0.001 {
				return false
			}

			return true
		},
		gen.IntRange(1, 1000), // Filtered key count
		gen.IntRange(0, 1000), // Processed keys
	))

	properties.Property("filtered migration completion is accurate", prop.ForAll(
		func(filteredKeyCount int) bool {
			if filteredKeyCount <= 0 {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())
			monitor.Initialize(filteredKeyCount)

			// Process all filtered keys
			for i := 0; i < filteredKeyCount; i++ {
				monitor.IncrementProcessed()
			}

			processed, total, _, percentage := monitor.GetProgress()

			// Should show 100% completion when all filtered keys are processed
			if processed != filteredKeyCount {
				return false
			}
			if total != filteredKeyCount {
				return false
			}
			if abs(percentage-100.0) > 0.001 {
				return false
			}

			return true
		},
		gen.IntRange(1, 1000), // Filtered key count
	))

	properties.Property("filtered migration statistics are independent of total database size", prop.ForAll(
		func(totalDBKeys int, filteredKeys int, processedKeys int) bool {
			if totalDBKeys <= 0 || filteredKeys <= 0 || filteredKeys > totalDBKeys ||
				processedKeys < 0 || processedKeys > filteredKeys {
				return true // Skip invalid inputs
			}

			monitor := NewProgressMonitor(createTestLogger())
			monitor.Initialize(filteredKeys) // Only filtered keys matter

			// Process some keys
			successCount := processedKeys / 2
			failureCount := processedKeys - successCount

			for i := 0; i < successCount; i++ {
				monitor.IncrementProcessed()
			}
			for i := 0; i < failureCount; i++ {
				monitor.IncrementFailed()
			}

			stats := monitor.GetStats()

			// Statistics should be based only on filtered set
			if stats.TotalKeys != filteredKeys {
				return false
			}
			if stats.ProcessedKeys != processedKeys {
				return false
			}
			if stats.SuccessfulKeys != successCount {
				return false
			}
			if stats.FailedKeys != failureCount {
				return false
			}

			// Progress percentage should be based on filtered set
			_, _, _, percentage := monitor.GetProgress()
			expectedPercentage := float64(processedKeys) / float64(filteredKeys) * 100
			if abs(percentage-expectedPercentage) > 0.001 {
				return false
			}

			return true
		},
		gen.IntRange(100, 10000), // Total DB keys
		gen.IntRange(1, 100),     // Filtered keys
		gen.IntRange(0, 100),     // Processed keys
	))

	properties.TestingRun(t)
}

// Helper function to generate test keys
func generateTestKey(index int) string {
	return "test:key:" + string(rune('0'+index%10)) + string(rune('a'+index%26))
}

// Helper function to calculate absolute difference for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
