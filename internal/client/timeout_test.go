package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kinyelo/redis-valkey-migration/internal/config"
)

func TestClientConfig_OperationTimeout(t *testing.T) {
	testCases := []struct {
		name         string
		dataType     string
		dataSize     int64
		expectedType string // which timeout should be used
	}{
		{"string operation", "string", 100, "string"},
		{"hash operation", "hash", 500, "hash"},
		{"list operation", "list", 1000, "list"},
		{"set operation", "set", 200, "set"},
		{"sorted set operation", "zset", 300, "sorted_set"},
		{"unknown type uses default", "unknown", 100, "default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := NewClientConfig("localhost", 6379, "", 0)

			timeout := config.getOperationTimeout(tc.dataType, tc.dataSize)

			var expectedTimeout time.Duration
			switch tc.expectedType {
			case "string":
				expectedTimeout = config.TimeoutConfig.StringOperation
			case "hash":
				expectedTimeout = config.TimeoutConfig.HashOperation
			case "list":
				expectedTimeout = config.TimeoutConfig.ListOperation
			case "set":
				expectedTimeout = config.TimeoutConfig.SetOperation
			case "sorted_set":
				expectedTimeout = config.TimeoutConfig.SortedSetOperation
			case "default":
				expectedTimeout = config.TimeoutConfig.DefaultOperation
			}

			assert.Equal(t, expectedTimeout, timeout)
		})
	}
}

func TestClientConfig_LargeDataTimeoutScaling(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "", 0)

	// Set a custom threshold and multiplier for testing
	config.TimeoutConfig.LargeDataThreshold = 1000
	config.TimeoutConfig.LargeDataMultiplier = 2.5

	testCases := []struct {
		name        string
		dataType    string
		dataSize    int64
		shouldScale bool
	}{
		{"small data - no scaling", "hash", 500, false},
		{"at threshold - no scaling", "hash", 1000, false},
		{"large data - should scale", "hash", 1500, true},
		{"very large data - should scale", "list", 5000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := config.getOperationTimeout(tc.dataType, tc.dataSize)
			baseTimeout := config.TimeoutConfig.HashOperation // Using hash as example
			if tc.dataType == "list" {
				baseTimeout = config.TimeoutConfig.ListOperation
			}

			if tc.shouldScale {
				expectedTimeout := time.Duration(float64(baseTimeout) * config.TimeoutConfig.LargeDataMultiplier)
				assert.Equal(t, expectedTimeout, timeout)
			} else {
				assert.Equal(t, baseTimeout, timeout)
			}
		})
	}
}

func TestClientConfig_IsLargeData(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "", 0)
	config.TimeoutConfig.LargeDataThreshold = 1000

	testCases := []struct {
		name     string
		dataSize int64
		expected bool
	}{
		{"small data", 500, false},
		{"at threshold", 1000, false},
		{"just above threshold", 1001, true},
		{"large data", 5000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := config.IsLargeData(tc.dataSize)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestClientConfig_ContextTimeouts(t *testing.T) {
	config := &ClientConfig{
		Host:              "localhost",
		Port:              6379,
		Password:          "",
		Database:          0,
		ConnectionTimeout: 30 * time.Second,
		OperationTimeout:  10 * time.Second,
		LargeDataTimeout:  60 * time.Second,
		TimeoutConfig: &config.TimeoutConfig{
			ConnectionTimeout:   30 * time.Second,
			DefaultOperation:    10 * time.Second,
			StringOperation:     5 * time.Second,
			HashOperation:       15 * time.Second,
			ListOperation:       15 * time.Second,
			SetOperation:        15 * time.Second,
			SortedSetOperation:  20 * time.Second,
			LargeDataThreshold:  1000,
			LargeDataMultiplier: 2.0,
		},
	}

	t.Run("connection context uses connection timeout", func(t *testing.T) {
		ctx, cancel := config.ConnectionContext()
		defer cancel()

		deadline, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)

		actualTimeout := time.Until(deadline)
		margin := 100 * time.Millisecond

		assert.True(t, actualTimeout >= config.ConnectionTimeout-margin)
		assert.True(t, actualTimeout <= config.ConnectionTimeout+margin)
	})

	t.Run("operation context uses operation timeout", func(t *testing.T) {
		ctx, cancel := config.Context()
		defer cancel()

		deadline, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)

		actualTimeout := time.Until(deadline)
		margin := 100 * time.Millisecond

		assert.True(t, actualTimeout >= config.OperationTimeout-margin)
		assert.True(t, actualTimeout <= config.OperationTimeout+margin)
	})

	t.Run("operation context with data type uses specific timeout", func(t *testing.T) {
		ctx, cancel := config.OperationContext("hash", 500)
		defer cancel()

		deadline, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)

		actualTimeout := time.Until(deadline)
		margin := 100 * time.Millisecond
		expectedTimeout := config.TimeoutConfig.HashOperation

		assert.True(t, actualTimeout >= expectedTimeout-margin)
		assert.True(t, actualTimeout <= expectedTimeout+margin)
	})

	t.Run("operation context with large data uses scaled timeout", func(t *testing.T) {
		ctx, cancel := config.OperationContext("hash", 2000) // Above threshold
		defer cancel()

		deadline, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)

		actualTimeout := time.Until(deadline)
		margin := 100 * time.Millisecond
		expectedTimeout := time.Duration(float64(config.TimeoutConfig.HashOperation) * config.TimeoutConfig.LargeDataMultiplier)

		assert.True(t, actualTimeout >= expectedTimeout-margin)
		assert.True(t, actualTimeout <= expectedTimeout+margin)
	})
}

func TestNewClientConfigFromDatabaseConfig(t *testing.T) {
	dbConfig := &config.DatabaseConfig{
		Host:              "redis-host",
		Port:              6380,
		Password:          "secret",
		Database:          2,
		ConnectionTimeout: 45 * time.Second,
		OperationTimeout:  20 * time.Second,
		LargeDataTimeout:  90 * time.Second,
	}

	timeoutConfig := &config.TimeoutConfig{
		ConnectionTimeout:   45 * time.Second,
		DefaultOperation:    20 * time.Second,
		StringOperation:     8 * time.Second,
		HashOperation:       25 * time.Second,
		ListOperation:       25 * time.Second,
		SetOperation:        25 * time.Second,
		SortedSetOperation:  30 * time.Second,
		LargeDataThreshold:  5000,
		LargeDataMultiplier: 3.0,
	}

	clientConfig := NewClientConfigFromDatabaseConfig(dbConfig, timeoutConfig)

	assert.Equal(t, "redis-host", clientConfig.Host)
	assert.Equal(t, 6380, clientConfig.Port)
	assert.Equal(t, "secret", clientConfig.Password)
	assert.Equal(t, 2, clientConfig.Database)
	assert.Equal(t, 45*time.Second, clientConfig.ConnectionTimeout)
	assert.Equal(t, 20*time.Second, clientConfig.OperationTimeout)
	assert.Equal(t, 90*time.Second, clientConfig.LargeDataTimeout)
	assert.Equal(t, timeoutConfig, clientConfig.TimeoutConfig)
}

func TestClientConfig_TimeoutErrorHandling(t *testing.T) {
	t.Run("nil timeout config should not panic", func(t *testing.T) {
		config := &ClientConfig{
			Host:              "localhost",
			Port:              6379,
			Password:          "",
			Database:          0,
			ConnectionTimeout: 30 * time.Second,
			OperationTimeout:  10 * time.Second,
			LargeDataTimeout:  60 * time.Second,
			TimeoutConfig:     nil, // nil config
		}

		// Should not panic and should use operation timeout as fallback
		timeout := config.getOperationTimeout("hash", 1000)
		assert.Equal(t, config.OperationTimeout, timeout)

		// IsLargeData should return false for nil config
		assert.False(t, config.IsLargeData(1000))
	})
}
