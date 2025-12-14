package engine

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationEngineIntegration tests the complete migration workflow
func TestMigrationEngineIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test logger
	logConfig := logger.Config{
		Level:      "debug",
		OutputFile: "test_migration.log",
		Format:     "text",
	}
	log, err := logger.NewLogger(logConfig)
	require.NoError(t, err)

	// Create mock clients for testing
	sourceClient := &IntegrationTestClient{
		keys: map[string]interface{}{
			"test:string1": "hello world",
			"test:string2": "another value",
			"test:string3": "third value",
		},
		keyTypes: map[string]string{
			"test:string1": "string",
			"test:string2": "string",
			"test:string3": "string",
		},
		connected: false,
	}

	targetClient := &IntegrationTestClient{
		keys:      make(map[string]interface{}),
		keyTypes:  make(map[string]string),
		connected: false,
	}

	// Create engine configuration
	engineConfig := &EngineConfig{
		BatchSize:            100,
		ResumeFile:           "test_resume.json",
		VerifyAfterMigration: false, // Disable verification for simpler test
		ContinueOnError:      true,
		MaxConcurrency:       5,
		ProgressInterval:     1 * time.Second,
	}

	// Create migration engine
	engine, err := NewMigrationEngine(
		sourceClient,
		&client.ClientConfig{Host: "localhost", Port: 6379, Database: 0},
		targetClient,
		&client.ClientConfig{Host: "localhost", Port: 6380, Database: 0},
		log,
		engineConfig,
	)
	require.NoError(t, err)

	// Run migration
	err = engine.Migrate()
	require.NoError(t, err)

	// Verify all keys were migrated
	assert.Equal(t, len(sourceClient.keys), len(targetClient.keys))

	for key, expectedValue := range sourceClient.keys {
		actualValue, exists := targetClient.keys[key]
		assert.True(t, exists, "Key %s should exist in target", key)
		assert.Equal(t, expectedValue, actualValue, "Value for key %s should match", key)
	}

	// Verify key types were preserved
	for key, expectedType := range sourceClient.keyTypes {
		actualType, exists := targetClient.keyTypes[key]
		assert.True(t, exists, "Key type for %s should exist in target", key)
		assert.Equal(t, expectedType, actualType, "Key type for %s should match", key)
	}

	// Clean up test files
	os.Remove("test_migration.log")
	os.Remove("test_resume.json")
}

// TestMigrationEngineErrorScenarios tests error handling and recovery
func TestMigrationEngineErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logConfig := logger.Config{
		Level:      "debug",
		OutputFile: "test_error_migration.log",
		Format:     "text",
	}
	log, err := logger.NewLogger(logConfig)
	require.NoError(t, err)

	// Create source client with some keys
	sourceClient := &IntegrationTestClient{
		keys: map[string]interface{}{
			"good:key1": "value1",
			"error:key": "this will fail",
			"good:key2": "value2",
		},
		keyTypes: map[string]string{
			"good:key1": "string",
			"error:key": "string",
			"good:key2": "string",
		},
		connected: false,
	}

	// Create target client that will fail on specific key
	targetClient := &IntegrationTestClient{
		keys:      make(map[string]interface{}),
		keyTypes:  make(map[string]string),
		connected: false,
		failOnKey: "error:key", // This key will cause SetValue to fail
	}

	engineConfig := &EngineConfig{
		BatchSize:            100,
		ResumeFile:           "test_error_resume.json",
		VerifyAfterMigration: false, // Skip verification for error test
		ContinueOnError:      true,  // Continue despite errors
		MaxConcurrency:       1,
		ProgressInterval:     1 * time.Second,
	}

	engine, err := NewMigrationEngine(
		sourceClient,
		&client.ClientConfig{Host: "localhost", Port: 6379, Database: 0},
		targetClient,
		&client.ClientConfig{Host: "localhost", Port: 6380, Database: 0},
		log,
		engineConfig,
	)
	require.NoError(t, err)

	// Run migration - should fail due to critical error
	err = engine.Migrate()
	// Migration should fail due to critical error
	assert.Error(t, err) // Engine fails on critical error

	// Verify at least one good key was migrated before failure
	assert.True(t, targetClient.HasKey("good:key1"))

	// Verify error key was not migrated
	assert.False(t, targetClient.HasKey("error:key"))

	// Clean up
	os.Remove("test_error_migration.log")
	os.Remove("test_error_resume.json")
}

// TestMigrationEngineProgressReporting tests progress monitoring
func TestMigrationEngineProgressReporting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logConfig := logger.Config{
		Level:      "info",
		OutputFile: "test_progress_migration.log",
		Format:     "text",
	}
	log, err := logger.NewLogger(logConfig)
	require.NoError(t, err)

	// Create source with multiple keys
	sourceClient := &IntegrationTestClient{
		keys:      make(map[string]interface{}),
		keyTypes:  make(map[string]string),
		connected: false,
	}

	// Add 50 test keys
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("test:key:%d", i)
		sourceClient.keys[key] = fmt.Sprintf("value%d", i)
		sourceClient.keyTypes[key] = "string"
	}

	targetClient := &IntegrationTestClient{
		keys:      make(map[string]interface{}),
		keyTypes:  make(map[string]string),
		connected: false,
	}

	engineConfig := &EngineConfig{
		BatchSize:            10,
		ResumeFile:           "test_progress_resume.json",
		VerifyAfterMigration: true,
		ContinueOnError:      true,
		MaxConcurrency:       2,
		ProgressInterval:     500 * time.Millisecond, // Fast progress reporting
	}

	engine, err := NewMigrationEngine(
		sourceClient,
		&client.ClientConfig{Host: "localhost", Port: 6379, Database: 0},
		targetClient,
		&client.ClientConfig{Host: "localhost", Port: 6380, Database: 0},
		log,
		engineConfig,
	)
	require.NoError(t, err)

	// Run migration
	err = engine.Migrate()
	require.NoError(t, err)

	// Verify all keys migrated
	assert.Equal(t, 50, len(targetClient.keys))

	// Verify statistics
	stats := engine.GetStats()
	assert.Equal(t, 50, stats.TotalKeys)
	assert.Equal(t, 50, stats.ProcessedKeys)
	assert.Equal(t, 0, stats.FailedKeys)

	// Clean up
	os.Remove("test_progress_migration.log")
	os.Remove("test_progress_resume.json")
}

// IntegrationTestClient implements DatabaseClient for integration testing
type IntegrationTestClient struct {
	keys      map[string]interface{}
	keyTypes  map[string]string
	connected bool
	failOnKey string // Key that will cause operations to fail
}

func (m *IntegrationTestClient) Connect() error {
	m.connected = true
	return nil
}

func (m *IntegrationTestClient) Disconnect() error {
	m.connected = false
	return nil
}

func (m *IntegrationTestClient) GetAllKeys() ([]string, error) {
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}

	keys := make([]string, 0, len(m.keys))
	for key := range m.keys {
		keys = append(keys, key)
	}
	return keys, nil
}

func (m *IntegrationTestClient) GetKeyType(key string) (string, error) {
	if !m.connected {
		return "", fmt.Errorf("not connected")
	}

	keyType, exists := m.keyTypes[key]
	if !exists {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return keyType, nil
}

func (m *IntegrationTestClient) GetValue(key string) (interface{}, error) {
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}

	if key == m.failOnKey {
		return nil, fmt.Errorf("simulated error for key: %s", key)
	}

	value, exists := m.keys[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

func (m *IntegrationTestClient) SetValue(key string, value interface{}) error {
	if !m.connected {
		return fmt.Errorf("not connected")
	}

	if key == m.failOnKey {
		return fmt.Errorf("simulated error for key: %s", key)
	}

	m.keys[key] = value

	// Determine type based on value
	switch value.(type) {
	case string:
		m.keyTypes[key] = "string"
	case map[string]string:
		m.keyTypes[key] = "hash"
	case []string:
		// Could be list or set, check existing type
		if existingType, exists := m.keyTypes[key]; exists {
			m.keyTypes[key] = existingType
		} else {
			m.keyTypes[key] = "list" // Default to list
		}
	case []interface{}:
		m.keyTypes[key] = "set"
	case []redis.Z:
		m.keyTypes[key] = "zset"
	default:
		m.keyTypes[key] = "string"
	}

	return nil
}

func (m *IntegrationTestClient) Exists(key string) (bool, error) {
	if !m.connected {
		return false, fmt.Errorf("not connected")
	}

	_, exists := m.keys[key]
	return exists, nil
}

func (m *IntegrationTestClient) Ping() error {
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

func (m *IntegrationTestClient) GetTTL(key string) (time.Duration, error) {
	if !m.connected {
		return 0, fmt.Errorf("not connected")
	}
	return -1, nil // No TTL
}

func (m *IntegrationTestClient) SetTTL(key string, ttl time.Duration) error {
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// HasKey checks if a key exists (helper for tests)
func (m *IntegrationTestClient) HasKey(key string) bool {
	_, exists := m.keys[key]
	return exists
}
