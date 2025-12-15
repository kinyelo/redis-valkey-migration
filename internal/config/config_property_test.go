package config

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// createValidConfigForProperty creates a valid configuration for property testing
func createValidConfigForProperty() *Config {
	return &Config{
		Redis: DatabaseConfig{
			Host:              "localhost",
			Port:              6379,
			Password:          "test",
			Database:          0,
			ConnectionTimeout: 30 * time.Second,
			OperationTimeout:  10 * time.Second,
			LargeDataTimeout:  60 * time.Second,
		},
		Valkey: DatabaseConfig{
			Host:              "localhost",
			Port:              6380,
			Password:          "",
			Database:          0,
			ConnectionTimeout: 30 * time.Second,
			OperationTimeout:  10 * time.Second,
			LargeDataTimeout:  60 * time.Second,
		},
		Migration: MigrationConfig{
			BatchSize:     1000,
			RetryAttempts: 3,
			LogLevel:      "info",
			TimeoutConfig: TimeoutConfig{
				ConnectionTimeout:   30 * time.Second,
				DefaultOperation:    10 * time.Second,
				StringOperation:     5 * time.Second,
				HashOperation:       15 * time.Second,
				ListOperation:       15 * time.Second,
				SetOperation:        15 * time.Second,
				SortedSetOperation:  20 * time.Second,
				LargeDataThreshold:  10000,
				LargeDataMultiplier: 2.0,
			},
		},
	}
}

// **Feature: redis-valkey-migration, Property 5: Input Validation and Error Reporting**
// **Validates: Requirements 2.1, 2.4**
func TestProperty_InputValidationAndErrorReporting(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: For any invalid port, validation should fail with specific error details
	properties.Property("Invalid ports should be rejected with specific errors", prop.ForAll(
		func(port int) bool {
			config := createValidConfigForProperty()
			config.Redis.Port = port

			err := ValidateConfig(config)

			// If port is invalid, validation should fail
			isInvalid := port <= 0 || port > 65535

			if isInvalid {
				// Should have an error
				return err != nil
			} else {
				// Should not have an error for valid port
				return err == nil
			}
		},
		gen.IntRange(-100, 70000), // port
	))

	// Property: For any invalid database number, validation should fail
	properties.Property("Invalid database numbers should be rejected", prop.ForAll(
		func(database int) bool {
			config := createValidConfigForProperty()
			config.Redis.Database = database

			err := ValidateConfig(config)

			// Check if database number is invalid
			isInvalid := database < 0 || database > 15

			if isInvalid {
				// Should have an error
				return err != nil
			} else {
				// Should not have an error for valid database
				return err == nil
			}
		},
		gen.IntRange(-10, 20), // database
	))

	// Property: For any invalid batch size, validation should fail
	properties.Property("Invalid batch sizes should be rejected", prop.ForAll(
		func(batchSize int) bool {
			config := createValidConfigForProperty()
			config.Migration.BatchSize = batchSize

			err := ValidateConfig(config)

			// Check if batch size is invalid
			isInvalid := batchSize <= 0

			if isInvalid {
				// Should have an error
				return err != nil
			} else {
				// Should not have an error for valid batch size
				return err == nil
			}
		},
		gen.IntRange(-100, 10000), // batchSize
	))

	// Property: Empty host should be rejected
	properties.Property("Empty hosts should be rejected", prop.ForAll(
		func() bool {
			config := createValidConfigForProperty()
			config.Redis.Host = "" // Empty host

			err := ValidateConfig(config)
			// Should have an error for empty host
			return err != nil
		},
	))

	// Property: Valid configurations should always pass validation
	properties.Property("Valid configurations should always pass validation", prop.ForAll(
		func(port1, port2 int, db1, db2 int, batchSize, retryAttempts int) bool {
			config := createValidConfigForProperty()
			config.Redis.Port = port1
			config.Redis.Database = db1
			config.Valkey.Port = port2
			config.Valkey.Database = db2
			config.Migration.BatchSize = batchSize
			config.Migration.RetryAttempts = retryAttempts

			err := ValidateConfig(config)
			return err == nil
		},
		gen.IntRange(1, 65535), // port1
		gen.IntRange(1, 65535), // port2
		gen.IntRange(0, 15),    // db1
		gen.IntRange(0, 15),    // db2
		gen.IntRange(1, 10000), // batchSize
		gen.IntRange(0, 100),   // retryAttempts
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: redis-valkey-migration, Property 13: Timeout Configuration Validation**
// **Validates: Requirements 8.5**
func TestProperty_TimeoutConfigurationValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: For any invalid timeout values, validation should fail with specific error details
	properties.Property("Invalid timeout values should be rejected", prop.ForAll(
		func(connTimeoutNanos, defaultOpNanos, stringOpNanos, hashOpNanos, listOpNanos, setOpNanos, sortedSetOpNanos int64, threshold int64, multiplier float64) bool {
			config := createValidConfigForProperty()
			config.Migration.TimeoutConfig = TimeoutConfig{
				ConnectionTimeout:   time.Duration(connTimeoutNanos),
				DefaultOperation:    time.Duration(defaultOpNanos),
				StringOperation:     time.Duration(stringOpNanos),
				HashOperation:       time.Duration(hashOpNanos),
				ListOperation:       time.Duration(listOpNanos),
				SetOperation:        time.Duration(setOpNanos),
				SortedSetOperation:  time.Duration(sortedSetOpNanos),
				LargeDataThreshold:  threshold,
				LargeDataMultiplier: multiplier,
			}

			err := ValidateConfig(config)

			// Check if any timeout values are invalid
			maxTimeout := 10 * time.Minute
			connectionTimeout := time.Duration(connTimeoutNanos)
			defaultOp := time.Duration(defaultOpNanos)
			stringOp := time.Duration(stringOpNanos)
			hashOp := time.Duration(hashOpNanos)
			listOp := time.Duration(listOpNanos)
			setOp := time.Duration(setOpNanos)
			sortedSetOp := time.Duration(sortedSetOpNanos)

			isInvalid := connectionTimeout <= 0 || connectionTimeout > maxTimeout ||
				defaultOp <= 0 || defaultOp > maxTimeout ||
				stringOp <= 0 || stringOp > maxTimeout ||
				hashOp <= 0 || hashOp > maxTimeout ||
				listOp <= 0 || listOp > maxTimeout ||
				setOp <= 0 || setOp > maxTimeout ||
				sortedSetOp <= 0 || sortedSetOp > maxTimeout ||
				threshold <= 0 ||
				multiplier <= 1.0 || multiplier > 10.0

			if isInvalid {
				// Should have an error
				return err != nil
			} else {
				// Should not have an error for valid timeouts
				return err == nil
			}
		},
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // connTimeoutNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // defaultOpNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // stringOpNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // hashOpNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // listOpNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // setOpNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // sortedSetOpNanos
		gen.Int64Range(-1000, 100000),                            // threshold
		gen.Float64Range(-1.0, 15.0),                             // multiplier
	))

	// Property: Valid timeout configurations should always pass validation
	properties.Property("Valid timeout configurations should pass validation", prop.ForAll(
		func(connTimeoutNanos, defaultOpNanos, stringOpNanos, hashOpNanos, listOpNanos, setOpNanos, sortedSetOpNanos int64, threshold int64, multiplier float64) bool {
			config := createValidConfigForProperty()
			config.Migration.TimeoutConfig = TimeoutConfig{
				ConnectionTimeout:   time.Duration(connTimeoutNanos),
				DefaultOperation:    time.Duration(defaultOpNanos),
				StringOperation:     time.Duration(stringOpNanos),
				HashOperation:       time.Duration(hashOpNanos),
				ListOperation:       time.Duration(listOpNanos),
				SetOperation:        time.Duration(setOpNanos),
				SortedSetOperation:  time.Duration(sortedSetOpNanos),
				LargeDataThreshold:  threshold,
				LargeDataMultiplier: multiplier,
			}

			err := ValidateConfig(config)
			return err == nil
		},
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // connTimeoutNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // defaultOpNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // stringOpNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // hashOpNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // listOpNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // setOpNanos
		gen.Int64Range(int64(1*time.Second), int64(10*time.Minute)), // sortedSetOpNanos
		gen.Int64Range(1, 100000),                                   // threshold
		gen.Float64Range(1.1, 10.0),                                 // multiplier
	))

	// Property: Database timeout validation should work correctly
	properties.Property("Database timeout validation should work correctly", prop.ForAll(
		func(connTimeoutNanos, opTimeoutNanos, largeTimeoutNanos int64) bool {
			config := createValidConfigForProperty()
			config.Redis.ConnectionTimeout = time.Duration(connTimeoutNanos)
			config.Redis.OperationTimeout = time.Duration(opTimeoutNanos)
			config.Redis.LargeDataTimeout = time.Duration(largeTimeoutNanos)

			err := ValidateConfig(config)

			// Check if any timeout values are invalid
			connTimeout := time.Duration(connTimeoutNanos)
			opTimeout := time.Duration(opTimeoutNanos)
			largeTimeout := time.Duration(largeTimeoutNanos)
			isInvalid := connTimeout <= 0 || opTimeout <= 0 || largeTimeout <= 0

			if isInvalid {
				// Should have an error
				return err != nil
			} else {
				// Should not have an error for valid timeouts
				return err == nil
			}
		},
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // connTimeoutNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // opTimeoutNanos
		gen.Int64Range(-int64(time.Hour), int64(15*time.Minute)), // largeTimeoutNanos
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
