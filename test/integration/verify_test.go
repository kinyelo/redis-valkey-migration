package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/internal/engine"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// VerifyTestSuite tests the --verify flag functionality
type VerifyTestSuite struct {
	suite.Suite
	redisClient  *redis.Client
	valkeyClient *redis.Client
	redisConfig  *client.ClientConfig
	valkeyConfig *client.ClientConfig
	logger       logger.Logger
}

// SetupSuite initializes the test suite
func (suite *VerifyTestSuite) SetupSuite() {
	// Skip tests if Docker containers are not available
	if testing.Short() {
		suite.T().Skip("Skipping verify tests in short mode")
	}

	// Get configuration from environment or use defaults
	redisHost := getEnvOrDefault("REDIS_HOST", "127.0.0.1")
	redisPort := getEnvOrDefault("REDIS_PORT", "16379")
	valkeyHost := getEnvOrDefault("VALKEY_HOST", "127.0.0.1")
	valkeyPort := getEnvOrDefault("VALKEY_PORT", "16380")

	suite.redisConfig = client.NewClientConfig(
		redisHost,
		parsePort(redisPort),
		os.Getenv("REDIS_PASSWORD"),
		0,
	)

	suite.valkeyConfig = client.NewClientConfig(
		valkeyHost,
		parsePort(valkeyPort),
		os.Getenv("VALKEY_PASSWORD"),
		0,
	)

	// Create Redis clients for direct testing
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", suite.redisConfig.Host, suite.redisConfig.Port),
		Password: suite.redisConfig.Password,
		DB:       suite.redisConfig.Database,
	})

	suite.valkeyClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", suite.valkeyConfig.Host, suite.valkeyConfig.Port),
		Password: suite.valkeyConfig.Password,
		DB:       suite.valkeyConfig.Database,
	})

	// Set up logger
	logConfig := logger.Config{
		Level:      "debug",
		OutputFile: "verify_test.log",
		MaxSize:    10 * 1024 * 1024,
		MaxAge:     1,
		Format:     "text",
	}

	var err error
	suite.logger, err = logger.NewLogger(logConfig)
	require.NoError(suite.T(), err)

	// Test connections - skip if services are not available
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = suite.redisClient.Ping(ctx).Err()
	if err != nil {
		suite.T().Skipf("Skipping verify tests: Redis not available on %s:%d (%v)", redisHost, parsePort(redisPort), err)
	}

	err = suite.valkeyClient.Ping(ctx).Err()
	if err != nil {
		suite.T().Skipf("Skipping verify tests: Valkey not available on %s:%d (%v)", valkeyHost, parsePort(valkeyPort), err)
	}

	suite.logger.Info("Verify test suite initialized successfully")
}

// TearDownSuite cleans up the test suite
func (suite *VerifyTestSuite) TearDownSuite() {
	if suite.redisClient != nil {
		suite.redisClient.Close()
	}
	if suite.valkeyClient != nil {
		suite.valkeyClient.Close()
	}
}

// SetupTest prepares each test
func (suite *VerifyTestSuite) SetupTest() {
	ctx := context.Background()

	// Clear both databases before each test
	suite.redisClient.FlushDB(ctx)
	suite.valkeyClient.FlushDB(ctx)

	// Clean up any resume files from previous tests
	suite.cleanupResumeFiles()
}

// cleanupResumeFiles removes resume files that might interfere with tests
func (suite *VerifyTestSuite) cleanupResumeFiles() {
	resumeFiles := []string{
		"migration_resume.json",
		"verify_enabled_resume.json",
		"verify_disabled_resume.json",
		"verify_corrupted_resume.json",
	}

	for _, file := range resumeFiles {
		os.Remove(file) // Ignore errors - file might not exist
	}
}

// TestVerifyEnabled tests migration with verification enabled (--verify=true)
func (suite *VerifyTestSuite) TestVerifyEnabled() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// Create migration engine with verification enabled
	migrationEngine := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            50,
		MaxConcurrency:       3,
		VerifyAfterMigration: true, // This simulates --verify=true
		ContinueOnError:      false,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_enabled_resume.json",
	})

	// Run migration
	err := migrationEngine.Migrate()
	require.NoError(suite.T(), err, "Migration with verification enabled should complete successfully")

	// Verify all data was migrated correctly
	suite.verifyMigration(ctx)

	// Check statistics
	stats := migrationEngine.GetStats()
	assert.Equal(suite.T(), 5, stats.TotalKeys, "Should have migrated 5 keys")
	assert.Equal(suite.T(), 5, stats.ProcessedKeys, "Should have processed 5 keys")
	assert.Equal(suite.T(), 0, stats.FailedKeys, "Should have no failed keys")

	suite.logger.Info("Verification enabled test completed successfully")
}

// TestVerifyDisabled tests migration with verification disabled (--verify=false)
func (suite *VerifyTestSuite) TestVerifyDisabled() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// Create migration engine with verification disabled
	migrationEngine := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            50,
		MaxConcurrency:       3,
		VerifyAfterMigration: false, // This simulates --verify=false
		ContinueOnError:      true,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_disabled_resume.json",
	})

	// Run migration
	err := migrationEngine.Migrate()
	require.NoError(suite.T(), err, "Migration with verification disabled should complete successfully")

	// Manually verify data was migrated (since verification was disabled)
	suite.verifyMigration(ctx)

	// Check statistics
	stats := migrationEngine.GetStats()
	assert.Equal(suite.T(), 5, stats.TotalKeys, "Should have migrated 5 keys")
	assert.Equal(suite.T(), 5, stats.ProcessedKeys, "Should have processed 5 keys")
	assert.Equal(suite.T(), 0, stats.FailedKeys, "Should have no failed keys")

	suite.logger.Info("Verification disabled test completed successfully")
}

// TestVerifyWithCorruptedData tests verification behavior when data is corrupted
func (suite *VerifyTestSuite) TestVerifyWithCorruptedData() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// First, run migration without verification to get data into Valkey
	migrationEngine := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            50,
		MaxConcurrency:       3,
		VerifyAfterMigration: false, // Don't verify initially
		ContinueOnError:      true,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_corrupted_resume.json",
	})

	err := migrationEngine.Migrate()
	require.NoError(suite.T(), err, "Initial migration should complete successfully")

	// Corrupt some data in Valkey to simulate data integrity issues
	suite.corruptValkeyData(ctx)

	// Now run migration again with verification enabled
	migrationEngineWithVerify := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            50,
		MaxConcurrency:       3,
		VerifyAfterMigration: true, // Enable verification
		ContinueOnError:      false,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_corrupted_resume.json",
	})

	// This should fail due to verification errors
	err = migrationEngineWithVerify.Migrate()
	assert.Error(suite.T(), err, "Migration with verification should fail when data is corrupted")
	assert.Contains(suite.T(), err.Error(), "verification", "Error should mention verification failure")

	suite.logger.Info("Verification with corrupted data test completed successfully")
}

// TestVerifyLargeDataset tests verification with a large dataset
func (suite *VerifyTestSuite) TestVerifyLargeDataset() {
	ctx := context.Background()

	// Create a larger dataset
	keyCount := 500
	suite.populateLargeDataset(ctx, keyCount)

	// Create migration engine with verification enabled
	migrationEngine := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            100,
		MaxConcurrency:       5,
		VerifyAfterMigration: true, // Enable verification
		ContinueOnError:      true,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_large_resume.json",
	})

	// Run migration
	err := migrationEngine.Migrate()
	require.NoError(suite.T(), err, "Large dataset migration with verification should complete successfully")

	// Verify key count
	redisKeys, err := suite.redisClient.Keys(ctx, "*").Result()
	require.NoError(suite.T(), err)

	valkeyKeys, err := suite.valkeyClient.Keys(ctx, "*").Result()
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), len(redisKeys), len(valkeyKeys), "Key counts should match")
	assert.Equal(suite.T(), keyCount, len(valkeyKeys), "Should have migrated all keys")

	// Check statistics
	stats := migrationEngine.GetStats()
	assert.Equal(suite.T(), keyCount, stats.TotalKeys, "Should have correct total key count")
	assert.Equal(suite.T(), keyCount, stats.ProcessedKeys, "Should have processed all keys")
	assert.Equal(suite.T(), 0, stats.FailedKeys, "Should have no failed keys")

	suite.logger.Info("Large dataset verification test completed successfully")
}

// TestVerifyComplexDataTypes tests verification with complex data types
func (suite *VerifyTestSuite) TestVerifyComplexDataTypes() {
	ctx := context.Background()

	// Populate Redis with complex data
	suite.populateComplexTestData(ctx)

	// Create migration engine with verification enabled
	migrationEngine := suite.createMigrationEngineWithConfig(&engine.EngineConfig{
		BatchSize:            50,
		MaxConcurrency:       3,
		VerifyAfterMigration: true, // Enable verification
		ContinueOnError:      false,
		ProgressInterval:     time.Second,
		ResumeFile:           "verify_complex_resume.json",
	})

	// Run migration
	err := migrationEngine.Migrate()
	require.NoError(suite.T(), err, "Complex data migration with verification should complete successfully")

	// Manually verify complex data structures
	suite.verifyComplexData(ctx)

	// Check statistics
	stats := migrationEngine.GetStats()
	assert.Greater(suite.T(), stats.TotalKeys, 0, "Should have migrated keys")
	assert.Equal(suite.T(), stats.TotalKeys, stats.ProcessedKeys, "All keys should be processed")
	assert.Equal(suite.T(), 0, stats.FailedKeys, "Should have no failed keys")

	suite.logger.Info("Complex data types verification test completed successfully")
}

// Helper methods

func (suite *VerifyTestSuite) populateTestData(ctx context.Context) {
	// String
	suite.redisClient.Set(ctx, "verify:string", "hello world", 0)

	// Hash
	suite.redisClient.HSet(ctx, "verify:hash", map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
	})

	// List
	suite.redisClient.LPush(ctx, "verify:list", "item3", "item2", "item1")

	// Set
	suite.redisClient.SAdd(ctx, "verify:set", "member1", "member2", "member3")

	// Sorted Set
	suite.redisClient.ZAdd(ctx, "verify:zset", redis.Z{Score: 1, Member: "member1"})
	suite.redisClient.ZAdd(ctx, "verify:zset", redis.Z{Score: 2, Member: "member2"})
	suite.redisClient.ZAdd(ctx, "verify:zset", redis.Z{Score: 3, Member: "member3"})
}

func (suite *VerifyTestSuite) populateLargeDataset(ctx context.Context, keyCount int) {
	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("verify:large:key:%d", i)
		value := fmt.Sprintf("value_%d", i)
		suite.redisClient.Set(ctx, key, value, 0)
	}
}

func (suite *VerifyTestSuite) populateComplexTestData(ctx context.Context) {
	// Complex hash with many fields
	hashData := make(map[string]interface{})
	for i := 0; i < 50; i++ {
		hashData[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	suite.redisClient.HSet(ctx, "verify:complex:hash", hashData)

	// Large list
	for i := 0; i < 100; i++ {
		suite.redisClient.LPush(ctx, "verify:complex:list", fmt.Sprintf("item_%d", i))
	}

	// Large set
	for i := 0; i < 100; i++ {
		suite.redisClient.SAdd(ctx, "verify:complex:set", fmt.Sprintf("member_%d", i))
	}

	// Large sorted set
	for i := 0; i < 100; i++ {
		suite.redisClient.ZAdd(ctx, "verify:complex:zset", redis.Z{
			Score:  float64(i),
			Member: fmt.Sprintf("member_%d", i),
		})
	}

	// Multiple data types with similar prefixes
	for i := 0; i < 20; i++ {
		suite.redisClient.Set(ctx, fmt.Sprintf("verify:prefix:string:%d", i), fmt.Sprintf("value_%d", i), 0)
		suite.redisClient.HSet(ctx, fmt.Sprintf("verify:prefix:hash:%d", i), "field", fmt.Sprintf("value_%d", i))
	}
}

func (suite *VerifyTestSuite) corruptValkeyData(ctx context.Context) {
	// Corrupt string data
	suite.valkeyClient.Set(ctx, "verify:string", "corrupted data", 0)

	// Corrupt hash data by adding extra field
	suite.valkeyClient.HSet(ctx, "verify:hash", "extra_field", "extra_value")

	// Corrupt list data by adding extra item
	suite.valkeyClient.LPush(ctx, "verify:list", "extra_item")

	// Corrupt set data by adding extra member
	suite.valkeyClient.SAdd(ctx, "verify:set", "extra_member")

	// Corrupt sorted set data by changing a score
	suite.valkeyClient.ZAdd(ctx, "verify:zset", redis.Z{Score: 999, Member: "member1"})
}

func (suite *VerifyTestSuite) verifyMigration(ctx context.Context) {
	// Verify string
	val, err := suite.valkeyClient.Get(ctx, "verify:string").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "hello world", val)

	// Verify hash
	hashVals, err := suite.valkeyClient.HGetAll(ctx, "verify:hash").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value1", hashVals["field1"])
	assert.Equal(suite.T(), "value2", hashVals["field2"])
	assert.Equal(suite.T(), "value3", hashVals["field3"])

	// Verify list
	listVals, err := suite.valkeyClient.LRange(ctx, "verify:list", 0, -1).Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"item1", "item2", "item3"}, listVals)

	// Verify set
	setVals, err := suite.valkeyClient.SMembers(ctx, "verify:set").Result()
	require.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"member1", "member2", "member3"}, setVals)

	// Verify sorted set
	zsetVals, err := suite.valkeyClient.ZRangeWithScores(ctx, "verify:zset", 0, -1).Result()
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), zsetVals, 3)
	assert.Equal(suite.T(), "member1", zsetVals[0].Member)
	assert.Equal(suite.T(), float64(1), zsetVals[0].Score)
}

func (suite *VerifyTestSuite) verifyComplexData(ctx context.Context) {
	// Verify complex hash
	hashSize, err := suite.valkeyClient.HLen(ctx, "verify:complex:hash").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(50), hashSize)

	// Verify large list
	listSize, err := suite.valkeyClient.LLen(ctx, "verify:complex:list").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(100), listSize)

	// Verify large set
	setSize, err := suite.valkeyClient.SCard(ctx, "verify:complex:set").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(100), setSize)

	// Verify large sorted set
	zsetSize, err := suite.valkeyClient.ZCard(ctx, "verify:complex:zset").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(100), zsetSize)
}

func (suite *VerifyTestSuite) createMigrationEngineWithConfig(engineConfig *engine.EngineConfig) *engine.MigrationEngine {
	redisClient := client.NewRedisClient(suite.redisConfig)
	valkeyClient := client.NewValkeyClient(suite.valkeyConfig)

	migrationEngine, err := engine.NewMigrationEngine(
		redisClient,
		suite.redisConfig,
		valkeyClient,
		suite.valkeyConfig,
		suite.logger,
		engineConfig,
	)
	require.NoError(suite.T(), err)

	return migrationEngine
}

// TestVerifyTestSuite runs the verification test suite
func TestVerifyTestSuite(t *testing.T) {
	suite.Run(t, new(VerifyTestSuite))
}
