package client

import (
	"context"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/config"
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

	// GetKeysByPattern retrieves keys matching a specific pattern
	GetKeysByPattern(pattern string) ([]string, error)

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
	Host              string
	Port              int
	Password          string
	Database          int
	ConnectionTimeout time.Duration
	OperationTimeout  time.Duration
	LargeDataTimeout  time.Duration
	TimeoutConfig     *config.TimeoutConfig
}

// DefaultTimeout is the default connection timeout
const DefaultTimeout = 30 * time.Second

// NewClientConfig creates a new ClientConfig with default timeout
func NewClientConfig(host string, port int, password string, database int) *ClientConfig {
	return &ClientConfig{
		Host:              host,
		Port:              port,
		Password:          password,
		Database:          database,
		ConnectionTimeout: DefaultTimeout,
		OperationTimeout:  10 * time.Second,
		LargeDataTimeout:  60 * time.Second,
		TimeoutConfig: &config.TimeoutConfig{
			ConnectionTimeout:   DefaultTimeout,
			DefaultOperation:    10 * time.Second,
			StringOperation:     5 * time.Second,
			HashOperation:       15 * time.Second,
			ListOperation:       15 * time.Second,
			SetOperation:        15 * time.Second,
			SortedSetOperation:  20 * time.Second,
			LargeDataThreshold:  10000,
			LargeDataMultiplier: 2.0,
		},
	}
}

// NewClientConfigFromDatabaseConfig creates a ClientConfig from a DatabaseConfig
func NewClientConfigFromDatabaseConfig(dbConfig *config.DatabaseConfig, timeoutConfig *config.TimeoutConfig) *ClientConfig {
	return &ClientConfig{
		Host:              dbConfig.Host,
		Port:              dbConfig.Port,
		Password:          dbConfig.Password,
		Database:          dbConfig.Database,
		ConnectionTimeout: dbConfig.ConnectionTimeout,
		OperationTimeout:  dbConfig.OperationTimeout,
		LargeDataTimeout:  dbConfig.LargeDataTimeout,
		TimeoutConfig:     timeoutConfig,
	}
}

// Context returns a context with timeout for database operations
func (c *ClientConfig) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.OperationTimeout)
}

// ConnectionContext returns a context with connection timeout
func (c *ClientConfig) ConnectionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.ConnectionTimeout)
}

// OperationContext returns a context with operation-specific timeout based on data type and size
func (c *ClientConfig) OperationContext(dataType string, dataSize int64) (context.Context, context.CancelFunc) {
	timeout := c.getOperationTimeout(dataType, dataSize)
	return context.WithTimeout(context.Background(), timeout)
}

// getOperationTimeout returns the appropriate timeout for an operation based on data type and size
func (c *ClientConfig) getOperationTimeout(dataType string, dataSize int64) time.Duration {
	if c.TimeoutConfig == nil {
		return c.OperationTimeout
	}

	var baseTimeout time.Duration
	switch dataType {
	case "string":
		baseTimeout = c.TimeoutConfig.StringOperation
	case "hash":
		baseTimeout = c.TimeoutConfig.HashOperation
	case "list":
		baseTimeout = c.TimeoutConfig.ListOperation
	case "set":
		baseTimeout = c.TimeoutConfig.SetOperation
	case "zset", "sorted_set":
		baseTimeout = c.TimeoutConfig.SortedSetOperation
	default:
		baseTimeout = c.TimeoutConfig.DefaultOperation
	}

	// Apply large data scaling if needed
	if dataSize > c.TimeoutConfig.LargeDataThreshold {
		scaledTimeout := time.Duration(float64(baseTimeout) * c.TimeoutConfig.LargeDataMultiplier)
		return scaledTimeout
	}

	return baseTimeout
}

// IsLargeData checks if the data size exceeds the large data threshold
func (c *ClientConfig) IsLargeData(dataSize int64) bool {
	if c.TimeoutConfig == nil {
		return false
	}
	return dataSize > c.TimeoutConfig.LargeDataThreshold
}
