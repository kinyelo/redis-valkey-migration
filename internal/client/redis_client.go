package client

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient implements DatabaseClient for Redis
type RedisClient struct {
	client *redis.Client
	config *ClientConfig
}

// NewRedisClient creates a new Redis client
func NewRedisClient(config *ClientConfig) *RedisClient {
	return &RedisClient{
		config: config,
	}
}

// Connect establishes a connection to Redis
func (r *RedisClient) Connect() error {
	r.client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", r.config.Host, r.config.Port),
		Password: r.config.Password,
		DB:       r.config.Database,
	})

	ctx, cancel := r.config.Context()
	defer cancel()

	// Test the connection
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

// Disconnect closes the Redis connection
func (r *RedisClient) Disconnect() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// GetAllKeys retrieves all keys from Redis
func (r *RedisClient) GetAllKeys() ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	var keys []string
	iter := r.client.Scan(ctx, 0, "*", 0).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	return keys, nil
}

// GetKeyType returns the data type of a Redis key
func (r *RedisClient) GetKeyType(key string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	keyType, err := r.client.Type(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get key type for %s: %w", key, err)
	}

	return keyType, nil
}

// GetValue retrieves the value for a key, handling all Redis data types
func (r *RedisClient) GetValue(key string) (interface{}, error) {
	if r.client == nil {
		return nil, fmt.Errorf("Redis client not connected")
	}

	keyType, err := r.GetKeyType(key)
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	switch keyType {
	case "string":
		return r.client.Get(ctx, key).Result()
	case "hash":
		return r.client.HGetAll(ctx, key).Result()
	case "list":
		return r.client.LRange(ctx, key, 0, -1).Result()
	case "set":
		return r.client.SMembers(ctx, key).Result()
	case "zset":
		return r.client.ZRangeWithScores(ctx, key, 0, -1).Result()
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// SetValue stores a value for a key, handling all Redis data types
func (r *RedisClient) SetValue(key string, value interface{}) error {
	if r.client == nil {
		return fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	switch v := value.(type) {
	case string:
		return r.client.Set(ctx, key, v, 0).Err()
	case map[string]string:
		return r.client.HMSet(ctx, key, v).Err()
	case []string:
		// Clear existing list first
		pipe := r.client.Pipeline()
		pipe.Del(ctx, key)

		// Add all elements (if any)
		if len(v) > 0 {
			for _, item := range v {
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
		pipe := r.client.Pipeline()
		pipe.Del(ctx, key)

		if len(v) > 0 {
			pipe.SAdd(ctx, key, v...)
		} else {
			// Create empty set by adding and removing a dummy value
			pipe.SAdd(ctx, key, "dummy")
			pipe.SRem(ctx, key, "dummy")
		}

		_, err := pipe.Exec(ctx)
		return err
	case []redis.Z:
		// For sorted sets - clear existing sorted set first
		pipe := r.client.Pipeline()
		pipe.Del(ctx, key)

		if len(v) > 0 {
			pipe.ZAdd(ctx, key, v...)
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

// Exists checks if a key exists in Redis
func (r *RedisClient) Exists(key string) (bool, error) {
	if r.client == nil {
		return false, fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence for %s: %w", key, err)
	}

	return count > 0, nil
}

// Ping tests the Redis connection
func (r *RedisClient) Ping() error {
	if r.client == nil {
		return fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	return r.client.Ping(ctx).Err()
}

// GetTTL returns the time-to-live for a Redis key
func (r *RedisClient) GetTTL(key string) (time.Duration, error) {
	if r.client == nil {
		return 0, fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for %s: %w", key, err)
	}

	return ttl, nil
}

// SetTTL sets the time-to-live for a Redis key
func (r *RedisClient) SetTTL(key string, ttl time.Duration) error {
	if r.client == nil {
		return fmt.Errorf("Redis client not connected")
	}

	ctx, cancel := r.config.Context()
	defer cancel()

	return r.client.Expire(ctx, key, ttl).Err()
}
