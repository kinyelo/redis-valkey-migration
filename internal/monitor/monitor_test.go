package monitor

import (
	"errors"
	"testing"
	"time"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestMonitor() *ProgressMonitor {
	testLogger, _ := logger.NewLogger(logger.Config{Level: "info"})
	return NewProgressMonitor(testLogger)
}

func TestNewProgressMonitor(t *testing.T) {
	monitor := createTestMonitor()

	assert.NotNil(t, monitor)
	assert.Equal(t, 0, monitor.TotalKeys)
	assert.Equal(t, 0, monitor.ProcessedKeys)
	assert.Empty(t, monitor.FailedKeys)
	assert.Equal(t, StatusNotStarted, monitor.Status)
	assert.NotNil(t, monitor.FailedKeys)
	assert.NotNil(t, monitor.Errors)
}

func TestProgressMonitor_Start(t *testing.T) {
	monitor := createTestMonitor()
	totalKeys := 1000

	monitor.Start(totalKeys)

	assert.Equal(t, totalKeys, monitor.TotalKeys)
	assert.Equal(t, 0, monitor.ProcessedKeys)
	assert.Empty(t, monitor.FailedKeys)
	assert.Equal(t, StatusRunning, monitor.Status)
	assert.False(t, monitor.StartTime.IsZero())
	assert.True(t, monitor.EndTime.IsZero())

	stats := monitor.GetStatistics()
	assert.Equal(t, totalKeys, stats.TotalKeys)
	assert.Equal(t, 0, stats.SuccessfulKeys)
	assert.Equal(t, 0, stats.FailedKeys)
	assert.Equal(t, int64(0), stats.BytesTransferred)
}

func TestProgressMonitor_RecordSuccess(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	key := "test:key:1"
	bytesTransferred := int64(256)

	monitor.RecordSuccess(key, bytesTransferred)

	assert.Equal(t, 1, monitor.ProcessedKeys)
	stats := monitor.GetStatistics()
	assert.Equal(t, 1, stats.SuccessfulKeys)
	assert.Equal(t, 0, stats.FailedKeys)
	assert.Equal(t, bytesTransferred, stats.BytesTransferred)

	// Record another success
	monitor.RecordSuccess("test:key:2", 128)
	assert.Equal(t, 2, monitor.ProcessedKeys)
	stats = monitor.GetStatistics()
	assert.Equal(t, 2, stats.SuccessfulKeys)
	assert.Equal(t, bytesTransferred+128, stats.BytesTransferred)
}

func TestProgressMonitor_RecordFailure(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	key := "test:key:1"
	testError := errors.New("connection failed")

	monitor.RecordFailure(key, testError)

	assert.Equal(t, 1, monitor.ProcessedKeys)
	assert.Contains(t, monitor.FailedKeys, key)
	stats := monitor.GetStatistics()
	assert.Equal(t, 0, stats.SuccessfulKeys)
	assert.Equal(t, 1, stats.FailedKeys)

	errors := monitor.GetErrors()
	require.Len(t, errors, 1)
	assert.Equal(t, key, errors[0].Key)
	assert.Equal(t, testError.Error(), errors[0].Message)
	assert.False(t, errors[0].Timestamp.IsZero())
}

func TestProgressMonitor_GetProgress(t *testing.T) {
	monitor := createTestMonitor()
	totalKeys := 100
	monitor.Start(totalKeys)

	// Initial progress
	processed, total, failed, percentage := monitor.GetProgress()
	assert.Equal(t, 0, processed)
	assert.Equal(t, totalKeys, total)
	assert.Equal(t, 0, failed)
	assert.Equal(t, 0.0, percentage)

	// After some successes
	monitor.RecordSuccess("key1", 100)
	monitor.RecordSuccess("key2", 200)
	processed, total, failed, percentage = monitor.GetProgress()
	assert.Equal(t, 2, processed)
	assert.Equal(t, totalKeys, total)
	assert.Equal(t, 0, failed)
	assert.Equal(t, 2.0, percentage)

	// After some failures
	monitor.RecordFailure("key3", errors.New("error"))
	processed, total, failed, percentage = monitor.GetProgress()
	assert.Equal(t, 3, processed)
	assert.Equal(t, totalKeys, total)
	assert.Equal(t, 1, failed)
	assert.Equal(t, 3.0, percentage)
}

func TestProgressMonitor_Complete(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(10)

	// Process some keys
	monitor.RecordSuccess("key1", 100)
	monitor.RecordFailure("key2", errors.New("error"))

	monitor.Complete()

	assert.Equal(t, StatusCompleted, monitor.Status)
	assert.False(t, monitor.EndTime.IsZero())
	assert.True(t, monitor.EndTime.After(monitor.StartTime))

	stats := monitor.GetStatistics()
	assert.True(t, stats.Duration > 0)
}

func TestProgressMonitor_Fail(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(10)

	monitor.Fail()

	assert.Equal(t, StatusFailed, monitor.Status)
	assert.False(t, monitor.EndTime.IsZero())
	assert.True(t, monitor.EndTime.After(monitor.StartTime))

	stats := monitor.GetStatistics()
	assert.True(t, stats.Duration > 0)
}

func TestProgressMonitor_GetStatistics(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	// Test running statistics
	monitor.RecordSuccess("key1", 256)
	monitor.RecordSuccess("key2", 512)
	monitor.RecordFailure("key3", errors.New("error"))

	stats := monitor.GetStatistics()
	assert.Equal(t, 100, stats.TotalKeys)
	assert.Equal(t, 2, stats.SuccessfulKeys)
	assert.Equal(t, 1, stats.FailedKeys)
	assert.Equal(t, int64(768), stats.BytesTransferred)
	assert.True(t, stats.Duration > 0) // Should be running duration

	// Test completed statistics
	monitor.Complete()
	finalStats := monitor.GetStatistics()
	assert.Equal(t, stats.TotalKeys, finalStats.TotalKeys)
	assert.Equal(t, stats.SuccessfulKeys, finalStats.SuccessfulKeys)
	assert.Equal(t, stats.FailedKeys, finalStats.FailedKeys)
	assert.Equal(t, stats.BytesTransferred, finalStats.BytesTransferred)
	assert.True(t, finalStats.Duration > 0)
}

func TestProgressMonitor_GetStatus(t *testing.T) {
	monitor := createTestMonitor()

	assert.Equal(t, StatusNotStarted, monitor.GetStatus())

	monitor.Start(10)
	assert.Equal(t, StatusRunning, monitor.GetStatus())

	monitor.Complete()
	assert.Equal(t, StatusCompleted, monitor.GetStatus())

	// Test failure status
	monitor2 := createTestMonitor()
	monitor2.Start(10)
	monitor2.Fail()
	assert.Equal(t, StatusFailed, monitor2.GetStatus())
}

func TestProgressMonitor_GetErrors(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(10)

	// Initially no errors
	migrationErrors := monitor.GetErrors()
	assert.Empty(t, migrationErrors)

	// Add some errors
	monitor.RecordFailure("key1", errors.New("error 1"))
	monitor.RecordFailure("key2", errors.New("error 2"))

	migrationErrors = monitor.GetErrors()
	require.Len(t, migrationErrors, 2)
	assert.Equal(t, "key1", migrationErrors[0].Key)
	assert.Equal(t, "error 1", migrationErrors[0].Message)
	assert.Equal(t, "key2", migrationErrors[1].Key)
	assert.Equal(t, "error 2", migrationErrors[1].Message)
}

func TestProgressMonitor_ShouldReport(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(10)

	// Wait a bit to ensure time has passed since start
	time.Sleep(10 * time.Millisecond)

	// Should report after some time has passed
	assert.True(t, monitor.ShouldReport(5*time.Millisecond))

	// Mark as reported
	monitor.MarkReported()

	// Should not report immediately after marking
	assert.False(t, monitor.ShouldReport(1*time.Second))

	// Wait a bit and should report again
	time.Sleep(10 * time.Millisecond)
	assert.True(t, monitor.ShouldReport(5*time.Millisecond))
}

func TestProgressMonitor_MarkReported(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(10)

	initialTime := monitor.lastReported
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	monitor.MarkReported()
	assert.True(t, monitor.lastReported.After(initialTime))
}

func TestProgressMonitor_ThreadSafety(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(1000)

	// Test concurrent access
	done := make(chan bool, 3)

	// Goroutine 1: Record successes
	go func() {
		for i := 0; i < 100; i++ {
			monitor.RecordSuccess("success-key", 100)
		}
		done <- true
	}()

	// Goroutine 2: Record failures
	go func() {
		for i := 0; i < 50; i++ {
			monitor.RecordFailure("failure-key", errors.New("test error"))
		}
		done <- true
	}()

	// Goroutine 3: Read progress
	go func() {
		for i := 0; i < 200; i++ {
			monitor.GetProgress()
			monitor.GetStatistics()
			monitor.GetStatus()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify final state
	stats := monitor.GetStatistics()
	assert.Equal(t, 100, stats.SuccessfulKeys)
	assert.Equal(t, 50, stats.FailedKeys)
	assert.Equal(t, int64(10000), stats.BytesTransferred) // 100 * 100 bytes
}

func TestMigrationStatus_String(t *testing.T) {
	testCases := []struct {
		status   MigrationStatus
		expected string
	}{
		{StatusNotStarted, "Not Started"},
		{StatusRunning, "Running"},
		{StatusCompleted, "Completed"},
		{StatusFailed, "Failed"},
		{StatusPaused, "Paused"},
		{MigrationStatus(999), "Unknown"}, // Invalid status
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.status.String())
		})
	}
}

func TestProgressMonitor_CalculateRate(t *testing.T) {
	monitor := createTestMonitor()

	// Test zero duration
	stats := MigrationStats{Duration: 0}
	rate := monitor.calculateRate(stats)
	assert.Equal(t, 0.0, rate)

	// Test normal rate calculation
	stats = MigrationStats{
		SuccessfulKeys: 100,
		FailedKeys:     10,
		Duration:       10 * time.Second,
	}
	rate = monitor.calculateRate(stats)
	assert.Equal(t, 11.0, rate) // (100+10)/10 = 11 keys/second
}

func TestProgressMonitor_CalculateSuccessRate(t *testing.T) {
	monitor := createTestMonitor()

	// Test zero total
	stats := MigrationStats{SuccessfulKeys: 0, FailedKeys: 0}
	rate := monitor.calculateSuccessRate(stats)
	assert.Equal(t, 0.0, rate)

	// Test normal success rate calculation
	stats = MigrationStats{SuccessfulKeys: 80, FailedKeys: 20}
	rate = monitor.calculateSuccessRate(stats)
	assert.Equal(t, 80.0, rate) // 80/100 * 100 = 80%
}

func TestProgressMonitor_FormatBytes(t *testing.T) {
	monitor := createTestMonitor()

	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := monitor.formatBytes(tc.bytes)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProgressMonitor_PrintProgress(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	// Add some data
	monitor.RecordSuccess("key1", 1024)
	monitor.RecordSuccess("key2", 2048)
	monitor.RecordFailure("key3", errors.New("test error"))

	// This test just ensures PrintProgress doesn't panic
	// In a real scenario, you might capture stdout to verify output
	monitor.PrintProgress()
}

func TestProgressMonitor_PrintSummary(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	// Add some data
	for i := 0; i < 80; i++ {
		monitor.RecordSuccess("success-key", 100)
	}
	for i := 0; i < 5; i++ {
		monitor.RecordFailure("failure-key", errors.New("test error"))
	}

	monitor.Complete()

	// This test just ensures PrintSummary doesn't panic
	// In a real scenario, you might capture stdout to verify output format
	monitor.PrintSummary()
}

func TestProgressMonitor_PrintSummary_WithManyFailures(t *testing.T) {
	monitor := createTestMonitor()
	monitor.Start(100)

	// Add many failures to test the truncation logic
	for i := 0; i < 15; i++ {
		key := "failure-key-" + string(rune('0'+i%10))
		monitor.RecordFailure(key, errors.New("test error"))
	}

	monitor.Complete()

	// This test ensures PrintSummary handles many failures correctly
	monitor.PrintSummary()
}

// Test edge cases and error conditions
func TestProgressMonitor_EdgeCases(t *testing.T) {
	t.Run("zero total keys", func(t *testing.T) {
		monitor := createTestMonitor()
		monitor.Start(0)

		processed, total, failed, percentage := monitor.GetProgress()
		assert.Equal(t, 0, processed)
		assert.Equal(t, 0, total)
		assert.Equal(t, 0, failed)
		assert.Equal(t, 0.0, percentage)
	})

	t.Run("multiple starts", func(t *testing.T) {
		monitor := createTestMonitor()
		monitor.Start(100)
		monitor.RecordSuccess("key1", 100)

		// Start again - should reset
		monitor.Start(200)
		processed, total, _, _ := monitor.GetProgress()
		assert.Equal(t, 0, processed)
		assert.Equal(t, 200, total)
	})

	t.Run("operations before start", func(t *testing.T) {
		monitor := createTestMonitor()

		// These should not panic
		monitor.RecordSuccess("key1", 100)
		monitor.RecordFailure("key2", errors.New("error"))
		monitor.Complete()
		monitor.Fail()

		// Should still be in not started state
		assert.Equal(t, StatusNotStarted, monitor.GetStatus())

		// Operations before start should be ignored
		processed, _, _, _ := monitor.GetProgress()
		assert.Equal(t, 0, processed) // No operations should be recorded
	})
}
