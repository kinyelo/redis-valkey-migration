package client

import (
	"context"
	"time"
)

// DatabaseClient defines the interface for database operations
// This interface abstracts Redis and Valkey operations for the migration tool
type DatabaseClient interface {
	// Connect establishes a connection to the database
	Connect() error

	// Disconnect closes the connection to the database
	Disconnect() error

	// GetAllKeys retrieves all keys from the database
	GetAllKeys() ([]string, error)

	// GetKeyType returns the data type of a key (string, hash, list, set, zset)
	GetKeyType(key string) (string, error)

	// GetValue retrieves the value for a key, handling all Redis data types
	GetValue(key string) (interface{}, error)

	// SetValue stores a value for a key, handling all Redis data types
	SetValue(key string, value interface{}) error

	// Exists checks if a key exists in the database
	Exists(key string) (bool, error)

	// Ping tests the connection to the database
	Ping() error

	// GetTTL returns the time-to-live for a key (-1 if no TTL, -2 if key doesn't exist)
	GetTTL(key string) (time.Duration, error)

	// SetTTL sets the time-to-live for a key
	SetTTL(key string, ttl time.Duration) error
}

// ClientConfig holds configuration for database clients
type ClientConfig struct {
	Host     string
	Port     int
	Password string
	Database int
	Timeout  time.Duration
}

// DefaultTimeout is the default connection timeout
const DefaultTimeout = 30 * time.Second

// NewClientConfig creates a new ClientConfig with default timeout
func NewClientConfig(host string, port int, password string, database int) *ClientConfig {
	return &ClientConfig{
		Host:     host,
		Port:     port,
		Password: password,
		Database: database,
		Timeout:  DefaultTimeout,
	}
}

// Context returns a context with timeout for database operations
func (c *ClientConfig) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.Timeout)
}
