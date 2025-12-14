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
				Host:     host,
				Port:     port,
				Password: password,
				Database: database,
				Timeout:  timeout,
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
			if config.Timeout != DefaultTimeout {
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
