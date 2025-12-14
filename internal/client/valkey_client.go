package client

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ValkeyClient implements DatabaseClient for Valkey
// Since Valkey is Redis-compatible, we use the same Redis client library
type ValkeyClient struct {
	client *redis.Client
	config *ClientConfig
}

// NewValkeyClient creates a new Valkey client
func NewValkeyClient(config *ClientConfig) *ValkeyClient {
	return &ValkeyClient{
		config: config,
	}
}

// Connect establishes a connection to Valkey
func (v *ValkeyClient) Connect() error {
	v.client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", v.config.Host, v.config.Port),
		Password: v.config.Password,
		DB:       v.config.Database,
	})

	ctx, cancel := v.config.Context()
	defer cancel()

	// Test the connection
	if err := v.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return nil
}

// Disconnect closes the Valkey connection
func (v *ValkeyClient) Disconnect() error {
	if v.client != nil {
		return v.client.Close()
	}
	return nil
}

// GetAllKeys retrieves all keys from Valkey
func (v *ValkeyClient) GetAllKeys() ([]string, error) {
	if v.client == nil {
		return nil, fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	var keys []string
	iter := v.client.Scan(ctx, 0, "*", 0).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	return keys, nil
}

// GetKeyType returns the data type of a Valkey key
func (v *ValkeyClient) GetKeyType(key string) (string, error) {
	if v.client == nil {
		return "", fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	keyType, err := v.client.Type(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get key type for %s: %w", key, err)
	}

	return keyType, nil
}

// GetValue retrieves the value for a key, handling all Valkey data types
func (v *ValkeyClient) GetValue(key string) (interface{}, error) {
	if v.client == nil {
		return nil, fmt.Errorf("Valkey client not connected")
	}

	keyType, err := v.GetKeyType(key)
	if err != nil {
		return nil, err
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	switch keyType {
	case "string":
		return v.client.Get(ctx, key).Result()
	case "hash":
		return v.client.HGetAll(ctx, key).Result()
	case "list":
		return v.client.LRange(ctx, key, 0, -1).Result()
	case "set":
		return v.client.SMembers(ctx, key).Result()
	case "zset":
		return v.client.ZRangeWithScores(ctx, key, 0, -1).Result()
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// SetValue stores a value for a key, handling all Valkey data types
func (v *ValkeyClient) SetValue(key string, value interface{}) error {
	if v.client == nil {
		return fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	switch val := value.(type) {
	case string:
		return v.client.Set(ctx, key, val, 0).Err()
	case map[string]string:
		return v.client.HMSet(ctx, key, val).Err()
	case []string:
		// Clear existing list first
		pipe := v.client.Pipeline()
		pipe.Del(ctx, key)

		// Add all elements (if any)
		if len(val) > 0 {
			for _, item := range val {
				pipe.RPush(ctx, key, item)
			}
		} else {
			// Create empty list by pushing and popping a dummy value
			pipe.LPush(ctx, key, "dummy")
			pipe.LPop(ctx, key)
		}

		_, err := pipe.Exec(ctx)
		return err
	case []interface{}:
		// For sets - clear existing set first
		pipe := v.client.Pipeline()
		pipe.Del(ctx, key)

		if len(val) > 0 {
			pipe.SAdd(ctx, key, val...)
		} else {
			// Create empty set by adding and removing a dummy value
			pipe.SAdd(ctx, key, "dummy")
			pipe.SRem(ctx, key, "dummy")
		}

		_, err := pipe.Exec(ctx)
		return err
	case []redis.Z:
		// For sorted sets - clear existing sorted set first
		pipe := v.client.Pipeline()
		pipe.Del(ctx, key)

		if len(val) > 0 {
			pipe.ZAdd(ctx, key, val...)
		} else {
			// Create empty sorted set by adding and removing a dummy value
			pipe.ZAdd(ctx, key, redis.Z{Score: 0, Member: "dummy"})
			pipe.ZRem(ctx, key, "dummy")
		}

		_, err := pipe.Exec(ctx)
		return err
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
}

// Exists checks if a key exists in Valkey
func (v *ValkeyClient) Exists(key string) (bool, error) {
	if v.client == nil {
		return false, fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	count, err := v.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence for %s: %w", key, err)
	}

	return count > 0, nil
}

// Ping tests the Valkey connection
func (v *ValkeyClient) Ping() error {
	if v.client == nil {
		return fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	return v.client.Ping(ctx).Err()
}

// GetTTL returns the time-to-live for a Valkey key
func (v *ValkeyClient) GetTTL(key string) (time.Duration, error) {
	if v.client == nil {
		return 0, fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	ttl, err := v.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for %s: %w", key, err)
	}

	return ttl, nil
}

// SetTTL sets the time-to-live for a Valkey key
func (v *ValkeyClient) SetTTL(key string, ttl time.Duration) error {
	if v.client == nil {
		return fmt.Errorf("Valkey client not connected")
	}

	ctx, cancel := v.config.Context()
	defer cancel()

	return v.client.Expire(ctx, key, ttl).Err()
}
