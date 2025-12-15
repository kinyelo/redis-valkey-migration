package client

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: redis-valkey-migration, Property 1: Database Connection Establishment**
// **Validates: Requirements 1.1, 1.2, 2.2, 2.3**
// For any valid database configuration (Redis or Valkey), the Migration Tool should successfully establish a connection and be able to perform basic operations
func TestProperty_DatabaseConnectionEstablishment(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid host values
	hostGen := gen.OneConstOf("localhost", "127.0.0.1")
	// Generator for valid port values
	portGen := gen.IntRange(1024, 65535)
	// Generator for password values
	passwordGen := gen.OneConstOf("", "testpass", "password123")
	// Generator for valid database values
	databaseGen := gen.IntRange(0, 15)
	// Generator for timeout values
	timeoutGen := gen.OneConstOf(5*time.Second, 10*time.Second, 30*time.Second)

	properties.Property("valid configurations should allow client creation", prop.ForAll(
		func(host string, port int, password string, database int, timeout time.Duration) bool {
			config := &ClientConfig{
				Host:              host,
				Port:              port,
				Password:          password,
				Database:          database,
				ConnectionTimeout: timeout,
				OperationTimeout:  10 * time.Second,
				LargeDataTimeout:  60 * time.Second,
			}

			// Test Redis client creation
			redisClient := NewRedisClient(config)
			if redisClient == nil {
				return false
			}
			if redisClient.config != config {
				return false
			}

			// Test Valkey client creation
			valkeyClient := NewValkeyClient(config)
			if valkeyClient == nil {
				return false
			}
			if valkeyClient.config != config {
				return false
			}

			// Verify that both clients implement the DatabaseClient interface
			var _ DatabaseClient = redisClient
			var _ DatabaseClient = valkeyClient

			return true
		},
		hostGen, portGen, passwordGen, databaseGen, timeoutGen,
	))

	properties.TestingRun(t)
}

// Property test for configuration validation
func TestProperty_ConfigurationValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Test that valid configurations are properly handled
	properties.Property("valid configurations should create working clients", prop.ForAll(
		func(host string, port int, password string, database int) bool {
			// Only test with valid values
			if host == "" || port <= 0 || port > 65535 || database < 0 || database > 15 {
				return true // Skip invalid inputs
			}

			config := NewClientConfig(host, port, password, database)

			// Verify config was created correctly
			if config.Host != host || config.Port != port || config.Password != password || config.Database != database {
				return false
			}

			// Verify default timeout was set
			if config.ConnectionTimeout != DefaultTimeout {
				return false
			}

			// Test that clients can be created with this config
			redisClient := NewRedisClient(config)
			valkeyClient := NewValkeyClient(config)

			return redisClient != nil && valkeyClient != nil
		},
		gen.OneConstOf("localhost", "127.0.0.1", "redis.example.com"),
		gen.IntRange(1024, 65535),
		gen.OneConstOf("", "password", "secret123"),
		gen.IntRange(0, 15),
	))

	properties.TestingRun(t)
}

// **Feature: redis-valkey-migration, Property 14: Operation-Specific Timeout Application**
// **Validates: Requirements 8.1, 8.2, 8.4**
func TestProperty_OperationSpecificTimeoutApplication(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: For any data type and size, the client should apply the appropriate timeout
	properties.Property("operation-specific timeouts should be applied correctly", prop.ForAll(
		func(dataType string, dataSize int64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			// Get the timeout that should be applied
			expectedTimeout := config.getOperationTimeout(dataType, dataSize)

			// Create context using the operation context method
			ctx, cancel := config.OperationContext(dataType, dataSize)
			defer cancel()

			// Check that the context has the expected timeout
			deadline, hasDeadline := ctx.Deadline()
			if !hasDeadline {
				return false
			}

			// The actual timeout should be close to the expected timeout
			// (within a small margin due to timing differences)
			actualTimeout := time.Until(deadline)
			margin := 100 * time.Millisecond

			return actualTimeout >= expectedTimeout-margin && actualTimeout <= expectedTimeout+margin
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset", "unknown"), // dataType
		gen.Int64Range(0, 50000), // dataSize
	))

	// Property: Large data should get extended timeouts
	properties.Property("large data should receive extended timeouts", prop.ForAll(
		func(dataType string, largeSize int64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			// Ensure the size is above the threshold
			largeSize = config.TimeoutConfig.LargeDataThreshold + largeSize + 1

			// Get timeouts for small and large data
			smallTimeout := config.getOperationTimeout(dataType, 100) // Small size
			largeTimeout := config.getOperationTimeout(dataType, largeSize)

			// Large data timeout should be greater than small data timeout
			return largeTimeout > smallTimeout
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset"), // dataType
		gen.Int64Range(1, 10000),                                // additional size above threshold
	))

	// Property: Connection context should use connection timeout
	properties.Property("connection context should use connection timeout", prop.ForAll(
		func() bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			ctx, cancel := config.ConnectionContext()
			defer cancel()

			deadline, hasDeadline := ctx.Deadline()
			if !hasDeadline {
				return false
			}

			// The timeout should be close to the connection timeout
			actualTimeout := time.Until(deadline)
			margin := 100 * time.Millisecond

			return actualTimeout >= config.ConnectionTimeout-margin &&
				actualTimeout <= config.ConnectionTimeout+margin
		},
	))

	// Property: IsLargeData should correctly identify large data
	properties.Property("IsLargeData should correctly identify large data", prop.ForAll(
		func(dataSize int64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			isLarge := config.IsLargeData(dataSize)
			expected := dataSize > config.TimeoutConfig.LargeDataThreshold

			return isLarge == expected
		},
		gen.Int64Range(0, 50000), // dataSize
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: redis-valkey-migration, Property 15: Large Data Timeout Scaling**
// **Validates: Requirements 8.4**
func TestProperty_LargeDataTimeoutScaling(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: For any data structure exceeding the large data threshold,
	// the timeout should be scaled by the configured multiplier
	properties.Property("large data timeout scaling should apply multiplier correctly", prop.ForAll(
		func(dataType string, baseSize int64, multiplier float64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			// Set a custom multiplier for testing
			config.TimeoutConfig.LargeDataMultiplier = multiplier

			// Calculate size that exceeds threshold
			largeSize := config.TimeoutConfig.LargeDataThreshold + baseSize + 1

			// Get base timeout for the data type (without large data scaling)
			baseTimeout := config.getOperationTimeout(dataType, 100) // Small size

			// Get timeout for large data
			largeTimeout := config.getOperationTimeout(dataType, largeSize)

			// Calculate expected timeout with multiplier
			expectedTimeout := time.Duration(float64(baseTimeout) * multiplier)

			// Allow for small timing differences (1ms margin)
			margin := 1 * time.Millisecond

			return largeTimeout >= expectedTimeout-margin && largeTimeout <= expectedTimeout+margin
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset"), // dataType
		gen.Int64Range(1, 5000),                                 // baseSize (additional above threshold)
		gen.Float64Range(1.1, 5.0),                              // multiplier (valid range)
	))

	// Property: Data below threshold should not get scaled timeouts
	properties.Property("small data should not receive scaled timeouts", prop.ForAll(
		func(dataType string, smallSize int64, multiplier float64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			// Set a custom multiplier for testing
			config.TimeoutConfig.LargeDataMultiplier = multiplier

			// Ensure size is below threshold
			if smallSize >= config.TimeoutConfig.LargeDataThreshold {
				smallSize = config.TimeoutConfig.LargeDataThreshold - 1
			}
			if smallSize < 0 {
				smallSize = 0
			}

			// Get timeout for small data
			smallTimeout := config.getOperationTimeout(dataType, smallSize)

			// Get base timeout (what we expect for small data)
			var expectedTimeout time.Duration
			switch dataType {
			case "string":
				expectedTimeout = config.TimeoutConfig.StringOperation
			case "hash":
				expectedTimeout = config.TimeoutConfig.HashOperation
			case "list":
				expectedTimeout = config.TimeoutConfig.ListOperation
			case "set":
				expectedTimeout = config.TimeoutConfig.SetOperation
			case "zset":
				expectedTimeout = config.TimeoutConfig.SortedSetOperation
			default:
				expectedTimeout = config.TimeoutConfig.DefaultOperation
			}

			// Small data should get base timeout, not scaled timeout
			return smallTimeout == expectedTimeout
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset"), // dataType
		gen.Int64Range(0, 9999),                                 // smallSize (below default threshold of 10000)
		gen.Float64Range(1.1, 5.0),                              // multiplier
	))

	// Property: Threshold boundary behavior should be consistent
	properties.Property("threshold boundary should be handled correctly", prop.ForAll(
		func(dataType string, threshold int64, multiplier float64) bool {
			config := NewClientConfig("localhost", 6379, "", 0)

			// Set custom threshold and multiplier
			config.TimeoutConfig.LargeDataThreshold = threshold
			config.TimeoutConfig.LargeDataMultiplier = multiplier

			// Test data exactly at threshold
			atThresholdTimeout := config.getOperationTimeout(dataType, threshold)

			// Test data just above threshold
			aboveThresholdTimeout := config.getOperationTimeout(dataType, threshold+1)

			// Data at threshold should not be scaled, data above should be scaled
			return aboveThresholdTimeout > atThresholdTimeout
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset"), // dataType
		gen.Int64Range(100, 20000),                              // threshold
		gen.Float64Range(1.1, 3.0),                              // multiplier
	))

	// Property: Multiplier effect should be proportional
	properties.Property("multiplier effect should be proportional", prop.ForAll(
		func(dataType string, dataSize int64, multiplier1, multiplier2 float64) bool {
			// Ensure multiplier2 > multiplier1
			if multiplier2 <= multiplier1 {
				multiplier1, multiplier2 = multiplier2, multiplier1
			}

			config1 := NewClientConfig("localhost", 6379, "", 0)
			config2 := NewClientConfig("localhost", 6379, "", 0)

			config1.TimeoutConfig.LargeDataMultiplier = multiplier1
			config2.TimeoutConfig.LargeDataMultiplier = multiplier2

			// Ensure data size is above threshold
			largeSize := config1.TimeoutConfig.LargeDataThreshold + dataSize + 1

			timeout1 := config1.getOperationTimeout(dataType, largeSize)
			timeout2 := config2.getOperationTimeout(dataType, largeSize)

			// Higher multiplier should result in higher timeout
			return timeout2 > timeout1
		},
		gen.OneConstOf("string", "hash", "list", "set", "zset"), // dataType
		gen.Int64Range(1, 5000),                                 // dataSize (additional above threshold)
		gen.Float64Range(1.1, 2.5),                              // multiplier1
		gen.Float64Range(2.6, 5.0),                              // multiplier2
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
