package engine

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// Simple test to verify basic recovery functionality
func TestSimpleConnectionRecovery(t *testing.T) {
	// Create a simple logger that doesn't do anything
	mockLogger := &MockLogger{}
	mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("LogError", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return()

	// Create retry configuration with short delays for testing
	retryConfig := RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond, // Very short for testing
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"connection refused",
			"timeout",
		},
	}

	recovery := NewConnectionRecovery(retryConfig, mockLogger)

	// Test 1: Operation that succeeds after 2 retries
	attemptCount := 0
	err := recovery.WithRetry("test operation", func() error {
		attemptCount++
		if attemptCount < 3 {
			return errors.New("connection refused")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	// Test 2: Non-retryable error should fail immediately
	attemptCount = 0
	err = recovery.WithRetry("test operation", func() error {
		attemptCount++
		return errors.New("authentication failed")
	})

	if err == nil {
		t.Error("Expected error for non-retryable failure")
	}

	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attemptCount)
	}
}

func TestResumeState(t *testing.T) {
	resumeState := NewResumeState()
	resumeState.TotalKeys = 10

	// Mark some keys as processed
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key:%d", i)
		resumeState.MarkProcessed(key)
	}

	// Verify processed count
	if resumeState.GetProcessedCount() != 5 {
		t.Errorf("Expected 5 processed keys, got %d", resumeState.GetProcessedCount())
	}

	// Verify processed keys are marked correctly
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key:%d", i)
		if !resumeState.IsProcessed(key) {
			t.Errorf("Key %s should be marked as processed", key)
		}
	}

	// Verify unprocessed keys are not marked
	for i := 5; i < 10; i++ {
		key := fmt.Sprintf("key:%d", i)
		if resumeState.IsProcessed(key) {
			t.Errorf("Key %s should not be marked as processed", key)
		}
	}
}
