package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// MigrationStatus represents the current state of migration
type MigrationStatus int

const (
	StatusNotStarted MigrationStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusPaused
)

func (s MigrationStatus) String() string {
	switch s {
	case StatusNotStarted:
		return "Not Started"
	case StatusRunning:
		return "Running"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	case StatusPaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// MigrationStats holds detailed statistics about the migration process
type MigrationStats struct {
	TotalKeys        int
	ProcessedKeys    int
	SuccessfulKeys   int
	FailedKeys       int
	BytesTransferred int64
	Duration         time.Duration
	Throughput       float64
}

// MigrationError represents an error that occurred during migration
type MigrationError struct {
	Key       string
	Message   string
	Timestamp time.Time
}

// ProgressMonitor tracks and reports migration progress with detailed statistics
type ProgressMonitor struct {
	mu            sync.RWMutex
	TotalKeys     int
	ProcessedKeys int
	FailedKeys    []string
	StartTime     time.Time
	EndTime       time.Time
	Status        MigrationStatus
	Statistics    MigrationStats
	Errors        []MigrationError
	lastReported  time.Time
	logger        logger.Logger
}

// NewProgressMonitor creates a new progress monitor instance
func NewProgressMonitor(logger logger.Logger) *ProgressMonitor {
	return &ProgressMonitor{
		FailedKeys: make([]string, 0),
		Errors:     make([]MigrationError, 0),
		Status:     StatusNotStarted,
		logger:     logger,
	}
}

// Initialize initializes the progress monitor for a new migration session
func (pm *ProgressMonitor) Initialize(totalKeys int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalKeys = totalKeys
	pm.ProcessedKeys = 0
	pm.FailedKeys = make([]string, 0)
	pm.StartTime = time.Now()
	pm.EndTime = time.Time{}
	pm.Status = StatusRunning
	pm.Statistics = MigrationStats{
		TotalKeys: totalKeys,
	}
	pm.Errors = make([]MigrationError, 0)
	pm.lastReported = time.Now()
}

// IncrementProcessed increments the processed key count
func (pm *ProgressMonitor) IncrementProcessed() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.ProcessedKeys++
	pm.Statistics.ProcessedKeys++
	pm.Statistics.SuccessfulKeys++

	// Update throughput
	if pm.Status == StatusRunning {
		elapsed := time.Since(pm.StartTime).Seconds()
		if elapsed > 0 {
			pm.Statistics.Throughput = float64(pm.ProcessedKeys) / elapsed
		}
	}
}

// IncrementFailed increments the failed key count
func (pm *ProgressMonitor) IncrementFailed() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.ProcessedKeys++
	pm.Statistics.ProcessedKeys++
	pm.Statistics.FailedKeys++
}

// GetStats returns current migration statistics
func (pm *ProgressMonitor) GetStats() MigrationStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := pm.Statistics
	if pm.Status == StatusRunning {
		stats.Duration = time.Since(pm.StartTime)
		elapsed := stats.Duration.Seconds()
		if elapsed > 0 {
			stats.Throughput = float64(pm.ProcessedKeys) / elapsed
		}
	}

	return stats
}

// Start initializes the progress monitor for a new migration session
func (pm *ProgressMonitor) Start(totalKeys int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.TotalKeys = totalKeys
	pm.ProcessedKeys = 0
	pm.FailedKeys = make([]string, 0)
	pm.StartTime = time.Now()
	pm.EndTime = time.Time{}
	pm.Status = StatusRunning
	pm.Statistics = MigrationStats{
		TotalKeys: totalKeys,
	}
	pm.Errors = make([]MigrationError, 0)
	pm.lastReported = time.Now()
}

// RecordSuccess records a successful key migration
func (pm *ProgressMonitor) RecordSuccess(key string, bytesTransferred int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Only record if migration has started
	if pm.Status != StatusRunning {
		return
	}

	pm.ProcessedKeys++
	pm.Statistics.SuccessfulKeys++
	pm.Statistics.BytesTransferred += bytesTransferred
}

// RecordFailure records a failed key migration
func (pm *ProgressMonitor) RecordFailure(key string, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Only record if migration has started
	if pm.Status != StatusRunning {
		return
	}

	pm.ProcessedKeys++
	pm.FailedKeys = append(pm.FailedKeys, key)
	pm.Statistics.FailedKeys++

	migrationErr := MigrationError{
		Key:       key,
		Message:   err.Error(),
		Timestamp: time.Now(),
	}
	pm.Errors = append(pm.Errors, migrationErr)
}

// Complete marks the migration as completed and calculates final statistics
func (pm *ProgressMonitor) Complete() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Only complete if migration is running
	if pm.Status != StatusRunning {
		return
	}

	pm.EndTime = time.Now()
	pm.Status = StatusCompleted
	pm.Statistics.Duration = pm.EndTime.Sub(pm.StartTime)
}

// Fail marks the migration as failed
func (pm *ProgressMonitor) Fail() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Only fail if migration is running
	if pm.Status != StatusRunning {
		return
	}

	pm.EndTime = time.Now()
	pm.Status = StatusFailed
	pm.Statistics.Duration = pm.EndTime.Sub(pm.StartTime)
}

// GetProgress returns current progress information in a thread-safe manner
func (pm *ProgressMonitor) GetProgress() (processed, total, failed int, percentage float64) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	processed = pm.ProcessedKeys
	total = pm.TotalKeys
	failed = len(pm.FailedKeys)

	if total > 0 {
		percentage = float64(processed) / float64(total) * 100
	}

	return
}

// GetStatistics returns a copy of current migration statistics
func (pm *ProgressMonitor) GetStatistics() MigrationStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := pm.Statistics
	if pm.Status == StatusRunning {
		stats.Duration = time.Since(pm.StartTime)
	}

	return stats
}

// GetStatus returns the current migration status
func (pm *ProgressMonitor) GetStatus() MigrationStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.Status
}

// GetErrors returns a copy of all migration errors
func (pm *ProgressMonitor) GetErrors() []MigrationError {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	errors := make([]MigrationError, len(pm.Errors))
	copy(errors, pm.Errors)
	return errors
}

// ShouldReport determines if progress should be reported based on time interval
func (pm *ProgressMonitor) ShouldReport(interval time.Duration) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return time.Since(pm.lastReported) >= interval
}

// MarkReported updates the last reported timestamp
func (pm *ProgressMonitor) MarkReported() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.lastReported = time.Now()
}

// PrintProgress displays current progress to console
func (pm *ProgressMonitor) PrintProgress() {
	processed, total, failed, percentage := pm.GetProgress()
	stats := pm.GetStatistics()

	fmt.Printf("\r[%s] Progress: %d/%d (%.1f%%) | Success: %d | Failed: %d | Duration: %v | Rate: %.1f keys/sec",
		pm.GetStatus(),
		processed,
		total,
		percentage,
		stats.SuccessfulKeys,
		failed,
		stats.Duration.Truncate(time.Second),
		pm.calculateRate(stats),
	)
}

// PrintSummary displays comprehensive migration summary
func (pm *ProgressMonitor) PrintSummary() {
	stats := pm.GetStatistics()

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("MIGRATION SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Status: %s\n", pm.GetStatus())
	fmt.Printf("Total Keys: %d\n", stats.TotalKeys)
	fmt.Printf("Successful: %d\n", stats.SuccessfulKeys)
	fmt.Printf("Failed: %d\n", stats.FailedKeys)
	fmt.Printf("Success Rate: %.2f%%\n", pm.calculateSuccessRate(stats))
	fmt.Printf("Duration: %v\n", stats.Duration.Truncate(time.Millisecond))
	fmt.Printf("Average Rate: %.2f keys/second\n", pm.calculateRate(stats))
	fmt.Printf("Data Transferred: %s\n", pm.formatBytes(stats.BytesTransferred))

	if len(pm.FailedKeys) > 0 {
		fmt.Printf("\nFailed Keys (%d):\n", len(pm.FailedKeys))
		for i, key := range pm.FailedKeys {
			if i >= 10 { // Limit display to first 10 failed keys
				fmt.Printf("... and %d more\n", len(pm.FailedKeys)-10)
				break
			}
			fmt.Printf("  - %s\n", key)
		}
	}

	fmt.Println(strings.Repeat("=", 80))
}

// calculateRate calculates the processing rate in keys per second
func (pm *ProgressMonitor) calculateRate(stats MigrationStats) float64 {
	if stats.Duration.Seconds() == 0 {
		return 0
	}
	return float64(stats.SuccessfulKeys+stats.FailedKeys) / stats.Duration.Seconds()
}

// calculateSuccessRate calculates the success percentage
func (pm *ProgressMonitor) calculateSuccessRate(stats MigrationStats) float64 {
	total := stats.SuccessfulKeys + stats.FailedKeys
	if total == 0 {
		return 0
	}
	return float64(stats.SuccessfulKeys) / float64(total) * 100
}

// formatBytes formats byte count into human-readable format
func (pm *ProgressMonitor) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
