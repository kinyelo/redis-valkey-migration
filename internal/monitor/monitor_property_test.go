package monitor

import (
	"errors"
	"redis-valkey-migration/pkg/logger"
	"testing"

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
