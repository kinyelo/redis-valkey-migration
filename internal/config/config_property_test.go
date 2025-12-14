package config

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: redis-valkey-migration, Property 5: Input Validation and Error Reporting**
// **Validates: Requirements 2.1, 2.4**
func TestProperty_InputValidationAndErrorReporting(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: For any invalid port, validation should fail with specific error details
	properties.Property("Invalid ports should be rejected with specific errors", prop.ForAll(
		func(port int) bool {
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     port,
					Password: "test",
					Database: 0,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     1000,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

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
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     6379,
					Password: "",
					Database: database,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     1000,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

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
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     6379,
					Password: "",
					Database: 0,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     batchSize,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

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
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "", // Empty host
					Port:     6379,
					Password: "",
					Database: 0,
				},
				Valkey: DatabaseConfig{
					Host:     "localhost",
					Port:     6380,
					Password: "",
					Database: 0,
				},
				Migration: MigrationConfig{
					BatchSize:     1000,
					RetryAttempts: 3,
					LogLevel:      "info",
				},
			}

			err := ValidateConfig(config)
			// Should have an error for empty host
			return err != nil
		},
	))

	// Property: Valid configurations should always pass validation
	properties.Property("Valid configurations should always pass validation", prop.ForAll(
		func(port1, port2 int, db1, db2 int, batchSize, retryAttempts int) bool {
			config := &Config{
				Redis: DatabaseConfig{
					Host:     "localhost",
					Port:     port1,
					Password: "test-password",
					Database: db1,
				},
				Valkey: DatabaseConfig{
					Host:     "valkey-host",
					Port:     port2,
					Password: "test-password",
					Database: db2,
				},
				Migration: MigrationConfig{
					BatchSize:     batchSize,
					RetryAttempts: retryAttempts,
					LogLevel:      "info",
				},
			}

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
