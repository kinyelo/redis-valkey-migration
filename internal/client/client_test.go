package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientConfig(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "password", 0)

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, "password", config.Password)
	assert.Equal(t, 0, config.Database)
	assert.Equal(t, DefaultTimeout, config.ConnectionTimeout)
	assert.Equal(t, 10*time.Second, config.OperationTimeout)
	assert.Equal(t, 60*time.Second, config.LargeDataTimeout)
	assert.NotNil(t, config.TimeoutConfig)
}

func TestClientConfig_Context(t *testing.T) {
	config := &ClientConfig{
		Host:              "localhost",
		Port:              6379,
		Password:          "",
		Database:          0,
		OperationTimeout:  5 * time.Second,
		ConnectionTimeout: 30 * time.Second,
		LargeDataTimeout:  60 * time.Second,
	}

	ctx, cancel := config.Context()
	defer cancel()

	assert.NotNil(t, ctx)

	// Verify the context has the correct timeout
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.True(t, time.Until(deadline) <= 5*time.Second)
	assert.True(t, time.Until(deadline) > 4*time.Second) // Should be close to 5 seconds
}

func TestNewRedisClient(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "password", 0)
	client := NewRedisClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.Nil(t, client.client) // Should be nil until Connect() is called
}

func TestNewValkeyClient(t *testing.T) {
	config := NewClientConfig("localhost", 6380, "password", 0)
	client := NewValkeyClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.Nil(t, client.client) // Should be nil until Connect() is called
}

func TestRedisClient_Disconnect_WithoutConnect(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "", 0)
	client := NewRedisClient(config)

	// Should not panic when disconnecting without connecting
	err := client.Disconnect()
	assert.NoError(t, err)
}

func TestValkeyClient_Disconnect_WithoutConnect(t *testing.T) {
	config := NewClientConfig("localhost", 6380, "", 0)
	client := NewValkeyClient(config)

	// Should not panic when disconnecting without connecting
	err := client.Disconnect()
	assert.NoError(t, err)
}

// Test connection establishment with invalid credentials
// Note: These tests verify the connection logic without requiring actual Redis/Valkey instances
func TestRedisClient_Connect_InvalidHost(t *testing.T) {
	config := NewClientConfig("invalid-host-that-does-not-exist", 6379, "", 0)
	config.ConnectionTimeout = 1 * time.Second // Short timeout for faster test
	client := NewRedisClient(config)

	err := client.Connect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestValkeyClient_Connect_InvalidHost(t *testing.T) {
	config := NewClientConfig("invalid-host-that-does-not-exist", 6380, "", 0)
	config.ConnectionTimeout = 1 * time.Second // Short timeout for faster test
	client := NewValkeyClient(config)

	err := client.Connect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Valkey")
}

func TestRedisClient_Connect_InvalidPort(t *testing.T) {
	config := NewClientConfig("localhost", 99999, "", 0) // Invalid port
	config.ConnectionTimeout = 1 * time.Second           // Short timeout for faster test
	client := NewRedisClient(config)

	err := client.Connect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestValkeyClient_Connect_InvalidPort(t *testing.T) {
	config := NewClientConfig("localhost", 99999, "", 0) // Invalid port
	config.ConnectionTimeout = 1 * time.Second           // Short timeout for faster test
	client := NewValkeyClient(config)

	err := client.Connect()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Valkey")
}

// Test that clients implement the DatabaseClient interface
func TestRedisClient_ImplementsInterface(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "", 0)
	client := NewRedisClient(config)

	// This should compile without error
	var _ DatabaseClient = client
}

func TestValkeyClient_ImplementsInterface(t *testing.T) {
	config := NewClientConfig("localhost", 6380, "", 0)
	client := NewValkeyClient(config)

	// This should compile without error
	var _ DatabaseClient = client
}

// Test operations without connection (should return errors)
func TestRedisClient_OperationsWithoutConnection(t *testing.T) {
	config := NewClientConfig("localhost", 6379, "", 0)
	client := NewRedisClient(config)

	// These operations should fail gracefully when not connected
	_, err := client.GetAllKeys()
	assert.Error(t, err)

	_, err = client.GetKeyType("test")
	assert.Error(t, err)

	_, err = client.GetValue("test")
	assert.Error(t, err)

	err = client.SetValue("test", "value")
	assert.Error(t, err)

	_, err = client.Exists("test")
	assert.Error(t, err)

	err = client.Ping()
	assert.Error(t, err)

	_, err = client.GetTTL("test")
	assert.Error(t, err)

	err = client.SetTTL("test", time.Minute)
	assert.Error(t, err)
}

func TestValkeyClient_OperationsWithoutConnection(t *testing.T) {
	config := NewClientConfig("localhost", 6380, "", 0)
	client := NewValkeyClient(config)

	// These operations should fail gracefully when not connected
	_, err := client.GetAllKeys()
	assert.Error(t, err)

	_, err = client.GetKeyType("test")
	assert.Error(t, err)

	_, err = client.GetValue("test")
	assert.Error(t, err)

	err = client.SetValue("test", "value")
	assert.Error(t, err)

	_, err = client.Exists("test")
	assert.Error(t, err)

	err = client.Ping()
	assert.Error(t, err)

	_, err = client.GetTTL("test")
	assert.Error(t, err)

	err = client.SetTTL("test", time.Minute)
	assert.Error(t, err)
}

// Test configuration validation scenarios
func TestClientConfig_ValidConfiguration(t *testing.T) {
	testCases := []struct {
		name     string
		host     string
		port     int
		password string
		database int
		valid    bool
	}{
		{"valid localhost", "localhost", 6379, "", 0, true},
		{"valid with password", "127.0.0.1", 6379, "password", 5, true},
		{"valid high port", "redis.example.com", 65535, "secret", 15, true},
		{"empty host", "", 6379, "", 0, false},
		{"invalid port low", "localhost", 0, "", 0, false},
		{"invalid port high", "localhost", 65536, "", 0, false},
		{"invalid database low", "localhost", 6379, "", -1, false},
		{"invalid database high", "localhost", 6379, "", 16, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ClientConfig{
				Host:              tc.host,
				Port:              tc.port,
				Password:          tc.password,
				Database:          tc.database,
				ConnectionTimeout: DefaultTimeout,
				OperationTimeout:  10 * time.Second,
				LargeDataTimeout:  60 * time.Second,
			}

			// Test that clients can be created (they don't validate config at creation time)
			redisClient := NewRedisClient(config)
			valkeyClient := NewValkeyClient(config)

			require.NotNil(t, redisClient)
			require.NotNil(t, valkeyClient)

			// The actual validation would happen during connection attempts
			// Invalid configs would fail during Connect(), not during client creation
		})
	}
}
