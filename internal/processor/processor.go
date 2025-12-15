package processor

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/internal/config"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// DataProcessor defines the interface for processing different Redis data types
type DataProcessor interface {
	// ProcessKey handles key migration based on its type
	ProcessKey(key, keyType string, source, target client.DatabaseClient) error

	// ProcessString handles string value migration
	ProcessString(key string, source, target client.DatabaseClient) error

	// ProcessHash handles hash field migration
	ProcessHash(key string, source, target client.DatabaseClient) error

	// ProcessList handles list element migration with order preservation
	ProcessList(key string, source, target client.DatabaseClient) error

	// ProcessSet handles set member migration
	ProcessSet(key string, source, target client.DatabaseClient) error

	// ProcessSortedSet handles sorted set migration with scores
	ProcessSortedSet(key string, source, target client.DatabaseClient) error
}

// migrationProcessor implements DataProcessor interface
type migrationProcessor struct {
	logger        logger.Logger
	timeoutConfig *config.TimeoutConfig
}

// NewDataProcessor creates a new DataProcessor instance
func NewDataProcessor(logger logger.Logger) DataProcessor {
	return &migrationProcessor{
		logger: logger,
	}
}

// NewDataProcessorWithTimeout creates a new DataProcessor instance with timeout configuration
func NewDataProcessorWithTimeout(logger logger.Logger, timeoutConfig *config.TimeoutConfig) DataProcessor {
	return &migrationProcessor{
		logger:        logger,
		timeoutConfig: timeoutConfig,
	}
}

// ProcessKey handles key migration based on its type
func (p *migrationProcessor) ProcessKey(key, keyType string, source, target client.DatabaseClient) error {
	switch keyType {
	case "string":
		return p.ProcessString(key, source, target)
	case "hash":
		return p.ProcessHash(key, source, target)
	case "list":
		return p.ProcessList(key, source, target)
	case "set":
		return p.ProcessSet(key, source, target)
	case "zset":
		return p.ProcessSortedSet(key, source, target)
	default:
		return fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// logLargeDataDetection logs when large data is detected and timeout adjustments are made
func (p *migrationProcessor) logLargeDataDetection(key, keyType string, dataSize int64) {
	if p.timeoutConfig != nil && dataSize > p.timeoutConfig.LargeDataThreshold {
		p.logger.Infof("Large data detected for key %s (type: %s, size: %d elements). Extended timeout will be applied (multiplier: %.1fx)",
			key, keyType, dataSize, p.timeoutConfig.LargeDataMultiplier)
	}
}

// ProcessString handles string value migration
func (p *migrationProcessor) ProcessString(key string, source, target client.DatabaseClient) error {
	startTime := time.Now()

	// Get the string value from source
	value, err := source.GetValue(key)
	if err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "string", 0, false, duration, err.Error())
		return fmt.Errorf("failed to get string value for key %s: %w", key, err)
	}

	stringValue, ok := value.(string)
	if !ok {
		duration := time.Since(startTime)
		errMsg := fmt.Sprintf("expected string value, got %T", value)
		p.logger.LogKeyTransfer(key, "string", 0, false, duration, errMsg)
		return fmt.Errorf("expected string value for key %s, got %T", key, value)
	}

	// Log large data detection for strings (based on byte length)
	p.logLargeDataDetection(key, "string", int64(len(stringValue)))

	// Get TTL from source
	ttl, err := source.GetTTL(key)
	if err != nil {
		p.logger.Warnf("Failed to get TTL for key %s: %v", key, err)
		ttl = -1 // No TTL
	}

	// Set the string value in target
	if err := target.SetValue(key, stringValue); err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "string", int64(len(stringValue)), false, duration, err.Error())
		return fmt.Errorf("failed to set string value for key %s: %w", key, err)
	}

	// Set TTL if it exists
	if ttl > 0 {
		if err := target.SetTTL(key, ttl); err != nil {
			p.logger.Warnf("Failed to set TTL for key %s: %v", key, err)
		}
	}

	duration := time.Since(startTime)
	p.logger.LogKeyTransfer(key, "string", int64(len(stringValue)), true, duration, "")
	return nil
}

// ProcessHash handles hash field migration
func (p *migrationProcessor) ProcessHash(key string, source, target client.DatabaseClient) error {
	startTime := time.Now()

	// Get the hash value from source
	value, err := source.GetValue(key)
	if err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "hash", 0, false, duration, err.Error())
		return fmt.Errorf("failed to get hash value for key %s: %w", key, err)
	}

	hashValue, ok := value.(map[string]string)
	if !ok {
		duration := time.Since(startTime)
		errMsg := fmt.Sprintf("expected map[string]string value, got %T", value)
		p.logger.LogKeyTransfer(key, "hash", 0, false, duration, errMsg)
		return fmt.Errorf("expected map[string]string value for key %s, got %T", key, value)
	}

	// Calculate size (number of fields for hash)
	size := int64(len(hashValue))

	// Log large data detection
	p.logLargeDataDetection(key, "hash", size)

	// Get TTL from source
	ttl, err := source.GetTTL(key)
	if err != nil {
		p.logger.Warnf("Failed to get TTL for key %s: %v", key, err)
		ttl = -1 // No TTL
	}

	// Set the hash value in target
	if err := target.SetValue(key, hashValue); err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "hash", size, false, duration, err.Error())
		return fmt.Errorf("failed to set hash value for key %s: %w", key, err)
	}

	// Set TTL if it exists
	if ttl > 0 {
		if err := target.SetTTL(key, ttl); err != nil {
			p.logger.Warnf("Failed to set TTL for key %s: %v", key, err)
		}
	}

	duration := time.Since(startTime)
	p.logger.LogKeyTransfer(key, "hash", size, true, duration, "")
	return nil
}

// ProcessList handles list element migration with order preservation
func (p *migrationProcessor) ProcessList(key string, source, target client.DatabaseClient) error {
	startTime := time.Now()

	// Get the list value from source
	value, err := source.GetValue(key)
	if err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "list", 0, false, duration, err.Error())
		return fmt.Errorf("failed to get list value for key %s: %w", key, err)
	}

	listValue, ok := value.([]string)
	if !ok {
		duration := time.Since(startTime)
		errMsg := fmt.Sprintf("expected []string value, got %T", value)
		p.logger.LogKeyTransfer(key, "list", 0, false, duration, errMsg)
		return fmt.Errorf("expected []string value for key %s, got %T", key, value)
	}

	// Calculate size (number of elements for list)
	size := int64(len(listValue))

	// Log large data detection
	p.logLargeDataDetection(key, "list", size)

	// Get TTL from source
	ttl, err := source.GetTTL(key)
	if err != nil {
		p.logger.Warnf("Failed to get TTL for key %s: %v", key, err)
		ttl = -1 // No TTL
	}

	// Set the list value in target
	if err := target.SetValue(key, listValue); err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "list", size, false, duration, err.Error())
		return fmt.Errorf("failed to set list value for key %s: %w", key, err)
	}

	// Set TTL if it exists
	if ttl > 0 {
		if err := target.SetTTL(key, ttl); err != nil {
			p.logger.Warnf("Failed to set TTL for key %s: %v", key, err)
		}
	}

	duration := time.Since(startTime)
	p.logger.LogKeyTransfer(key, "list", size, true, duration, "")
	return nil
}

// ProcessSet handles set member migration
func (p *migrationProcessor) ProcessSet(key string, source, target client.DatabaseClient) error {
	startTime := time.Now()

	// Get the set value from source
	value, err := source.GetValue(key)
	if err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "set", 0, false, duration, err.Error())
		return fmt.Errorf("failed to get set value for key %s: %w", key, err)
	}

	setValue, ok := value.([]string)
	if !ok {
		duration := time.Since(startTime)
		errMsg := fmt.Sprintf("expected []string value, got %T", value)
		p.logger.LogKeyTransfer(key, "set", 0, false, duration, errMsg)
		return fmt.Errorf("expected []string value for key %s, got %T", key, value)
	}

	// Calculate size (number of members for set)
	size := int64(len(setValue))

	// Log large data detection
	p.logLargeDataDetection(key, "set", size)

	// Convert to []interface{} for SetValue
	interfaceSlice := make([]interface{}, len(setValue))
	for i, v := range setValue {
		interfaceSlice[i] = v
	}

	// Get TTL from source
	ttl, err := source.GetTTL(key)
	if err != nil {
		p.logger.Warnf("Failed to get TTL for key %s: %v", key, err)
		ttl = -1 // No TTL
	}

	// Set the set value in target
	if err := target.SetValue(key, interfaceSlice); err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "set", size, false, duration, err.Error())
		return fmt.Errorf("failed to set set value for key %s: %w", key, err)
	}

	// Set TTL if it exists
	if ttl > 0 {
		if err := target.SetTTL(key, ttl); err != nil {
			p.logger.Warnf("Failed to set TTL for key %s: %v", key, err)
		}
	}

	duration := time.Since(startTime)
	p.logger.LogKeyTransfer(key, "set", size, true, duration, "")
	return nil
}

// ProcessSortedSet handles sorted set migration with scores
func (p *migrationProcessor) ProcessSortedSet(key string, source, target client.DatabaseClient) error {
	startTime := time.Now()

	// Get the sorted set value from source
	value, err := source.GetValue(key)
	if err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "zset", 0, false, duration, err.Error())
		return fmt.Errorf("failed to get sorted set value for key %s: %w", key, err)
	}

	zsetValue, ok := value.([]redis.Z)
	if !ok {
		duration := time.Since(startTime)
		errMsg := fmt.Sprintf("expected []redis.Z value, got %T", value)
		p.logger.LogKeyTransfer(key, "zset", 0, false, duration, errMsg)
		return fmt.Errorf("expected []redis.Z value for key %s, got %T", key, value)
	}

	// Calculate size (number of members for sorted set)
	size := int64(len(zsetValue))

	// Log large data detection
	p.logLargeDataDetection(key, "zset", size)

	// Get TTL from source
	ttl, err := source.GetTTL(key)
	if err != nil {
		p.logger.Warnf("Failed to get TTL for key %s: %v", key, err)
		ttl = -1 // No TTL
	}

	// Set the sorted set value in target
	if err := target.SetValue(key, zsetValue); err != nil {
		duration := time.Since(startTime)
		p.logger.LogKeyTransfer(key, "zset", size, false, duration, err.Error())
		return fmt.Errorf("failed to set sorted set value for key %s: %w", key, err)
	}

	// Set TTL if it exists
	if ttl > 0 {
		if err := target.SetTTL(key, ttl); err != nil {
			p.logger.Warnf("Failed to set TTL for key %s: %v", key, err)
		}
	}

	duration := time.Since(startTime)
	p.logger.LogKeyTransfer(key, "zset", size, true, duration, "")
	return nil
}
