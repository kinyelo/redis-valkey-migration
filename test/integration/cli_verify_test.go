package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// CLIVerifyTestSuite tests the --verify flag from the command line interface
type CLIVerifyTestSuite struct {
	suite.Suite
	redisClient  *redis.Client
	valkeyClient *redis.Client
	projectRoot  string
}

// SetupSuite initializes the CLI test suite
func (suite *CLIVerifyTestSuite) SetupSuite() {
	if testing.Short() {
		suite.T().Skip("Skipping CLI verify tests in short mode")
	}

	// Check if Docker and Docker Compose are available
	suite.checkDockerAvailable()

	// Start test environment
	suite.startTestEnvironment()

	// Initialize clients
	suite.initializeClients()

	// Get project root
	suite.projectRoot = suite.getProjectRoot()

	// Build the migration tool
	suite.buildMigrationTool()
}

// TearDownSuite cleans up the CLI test environment
func (suite *CLIVerifyTestSuite) TearDownSuite() {
	if suite.redisClient != nil {
		suite.redisClient.Close()
	}
	if suite.valkeyClient != nil {
		suite.valkeyClient.Close()
	}
	suite.stopTestEnvironment()
}

// SetupTest prepares each test
func (suite *CLIVerifyTestSuite) SetupTest() {
	ctx := context.Background()
	suite.redisClient.FlushDB(ctx)
	suite.valkeyClient.FlushDB(ctx)
	suite.cleanupResumeFiles()
}

// TestCLIVerifyFlagEnabled tests the --verify=true flag
func (suite *CLIVerifyTestSuite) TestCLIVerifyFlagEnabled() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// Run migration with --verify=true
	args := []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--verify", "true",
		"--log-level", "debug",
		"--resume-file", "cli_verify_enabled_resume.json",
	}

	output, err := suite.runMigrationCommand(args)
	require.NoError(suite.T(), err, "Migration with --verify=true should succeed")

	// Check that verification was performed
	assert.Contains(suite.T(), output, "Verifying migration results", "Output should mention verification")
	assert.Contains(suite.T(), output, "Migration verification completed successfully", "Output should confirm successful verification")

	// Verify data was migrated correctly
	suite.verifyMigrationResults(ctx)
}

// TestCLIVerifyFlagDisabled tests the --verify=false flag
func (suite *CLIVerifyTestSuite) TestCLIVerifyFlagDisabled() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// Run migration with --verify=false
	args := []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--verify", "false",
		"--log-level", "debug",
		"--resume-file", "cli_verify_disabled_resume.json",
	}

	output, err := suite.runMigrationCommand(args)
	require.NoError(suite.T(), err, "Migration with --verify=false should succeed")

	// Check that verification was NOT performed
	assert.NotContains(suite.T(), output, "Verifying migration results", "Output should not mention verification when disabled")
	assert.NotContains(suite.T(), output, "Migration verification completed", "Output should not mention verification completion when disabled")

	// Verify data was still migrated correctly (manual verification)
	suite.verifyMigrationResults(ctx)
}

// TestCLIVerifyFlagDefault tests the default behavior (should be enabled)
func (suite *CLIVerifyTestSuite) TestCLIVerifyFlagDefault() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// Run migration without specifying --verify flag (should default to true)
	args := []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--log-level", "debug",
		"--resume-file", "cli_verify_default_resume.json",
	}

	output, err := suite.runMigrationCommand(args)
	require.NoError(suite.T(), err, "Migration with default verify setting should succeed")

	// Check that verification was performed by default
	assert.Contains(suite.T(), output, "Verifying migration results", "Output should mention verification by default")
	assert.Contains(suite.T(), output, "Migration verification completed successfully", "Output should confirm successful verification by default")

	// Verify data was migrated correctly
	suite.verifyMigrationResults(ctx)
}

// TestCLIVerifyWithCorruptedData tests verification failure detection
func (suite *CLIVerifyTestSuite) TestCLIVerifyWithCorruptedData() {
	ctx := context.Background()

	// Populate Redis with test data
	suite.populateTestData(ctx)

	// First, run migration without verification to get data into Valkey
	args := []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--verify", "false",
		"--log-level", "debug",
		"--resume-file", "cli_verify_corrupted_initial_resume.json",
	}

	_, err := suite.runMigrationCommand(args)
	require.NoError(suite.T(), err, "Initial migration should succeed")

	// Corrupt some data in Valkey
	suite.corruptValkeyData(ctx)

	// Now run migration again with verification enabled - this should fail
	args = []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--verify", "true",
		"--log-level", "debug",
		"--resume-file", "cli_verify_corrupted_verify_resume.json",
	}

	output, err := suite.runMigrationCommand(args)
	assert.Error(suite.T(), err, "Migration with verification should fail when data is corrupted")
	assert.Contains(suite.T(), output, "verification", "Output should mention verification failure")
}

// TestCLIVerifyLargeDataset tests verification with a larger dataset
func (suite *CLIVerifyTestSuite) TestCLIVerifyLargeDataset() {
	ctx := context.Background()

	// Create a larger dataset
	keyCount := 200 // Smaller than engine tests to keep CLI tests faster
	suite.populateLargeDataset(ctx, keyCount)

	// Run migration with verification enabled
	args := []string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--verify", "true",
		"--batch-size", "50",
		"--max-concurrency", "3",
		"--log-level", "info",
		"--resume-file", "cli_verify_large_resume.json",
	}

	output, err := suite.runMigrationCommand(args)
	require.NoError(suite.T(), err, "Large dataset migration with verification should succeed")

	// Check that verification was performed
	assert.Contains(suite.T(), output, "Verifying migration results", "Output should mention verification")
	assert.Contains(suite.T(), output, "Migration verification completed successfully", "Output should confirm successful verification")

	// Verify key count
	redisKeys, err := suite.redisClient.Keys(ctx, "*").Result()
	require.NoError(suite.T(), err)

	valkeyKeys, err := suite.valkeyClient.Keys(ctx, "*").Result()
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), len(redisKeys), len(valkeyKeys), "Key counts should match")
	assert.Equal(suite.T(), keyCount, len(valkeyKeys), "Should have migrated all keys")
}

// Helper methods

func (suite *CLIVerifyTestSuite) checkDockerAvailable() {
	// Check Docker
	cmd := exec.Command("docker", "--version")
	err := cmd.Run()
	require.NoError(suite.T(), err, "Docker must be available for CLI tests")

	// Check Docker Compose
	cmd = exec.Command("docker", "compose", "version")
	err = cmd.Run()
	if err != nil {
		cmd = exec.Command("docker-compose", "--version")
		err = cmd.Run()
		require.NoError(suite.T(), err, "Docker Compose must be available for CLI tests")
	}
}

func (suite *CLIVerifyTestSuite) startTestEnvironment() {
	// Clean up any existing containers first
	suite.stopTestEnvironment()

	// Start containers
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", "migration-cli-verify-test", "up", "-d")
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Failed to start test environment: %s", string(output))

	// Wait for services to be ready
	suite.waitForServices()
}

func (suite *CLIVerifyTestSuite) stopTestEnvironment() {
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", "migration-cli-verify-test", "down", "-v")
	cmd.Run() // Don't fail if cleanup fails
}

func (suite *CLIVerifyTestSuite) waitForServices() {
	// Wait for Redis
	for i := range 30 {
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", "migration-cli-verify-test", "exec", "-T", "redis", "redis-cli", "ping")
		if cmd.Run() == nil {
			break
		}
		time.Sleep(time.Second)
		if i == 29 {
			suite.T().Fatal("Redis failed to start within 30 seconds")
		}
	}

	// Wait for Valkey
	for i := range 30 {
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", "migration-cli-verify-test", "exec", "-T", "valkey", "valkey-cli", "ping")
		if cmd.Run() == nil {
			break
		}
		time.Sleep(time.Second)
		if i == 29 {
			suite.T().Fatal("Valkey failed to start within 30 seconds")
		}
	}
}

func (suite *CLIVerifyTestSuite) initializeClients() {
	// Create Redis client (connecting to localhost:16379)
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:16379",
		DB:   0,
	})

	// Create Valkey client (connecting to localhost:16380)
	suite.valkeyClient = redis.NewClient(&redis.Options{
		Addr: "localhost:16380",
		DB:   0,
	})

	// Test connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := suite.redisClient.Ping(ctx).Err()
	require.NoError(suite.T(), err, "Failed to connect to Redis")

	err = suite.valkeyClient.Ping(ctx).Err()
	require.NoError(suite.T(), err, "Failed to connect to Valkey")
}

func (suite *CLIVerifyTestSuite) getProjectRoot() string {
	cmd := exec.Command("pwd")
	cmd.Dir = "../.."
	output, err := cmd.Output()
	require.NoError(suite.T(), err)
	return strings.TrimSpace(string(output))
}

func (suite *CLIVerifyTestSuite) buildMigrationTool() {
	// Build the migration tool for Linux
	buildCmd := exec.Command("go", "build", "-o", "build/redis-valkey-migration-linux", ".")
	buildCmd.Dir = suite.projectRoot
	buildCmd.Env = append(buildCmd.Env, "GOOS=linux", "GOARCH=amd64")
	// Copy current environment
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GOOS=") && !strings.HasPrefix(env, "GOARCH=") {
			buildCmd.Env = append(buildCmd.Env, env)
		}
	}

	buildOutput, err := buildCmd.CombinedOutput()
	require.NoError(suite.T(), err, "Failed to build migration tool: %s", string(buildOutput))
}

func (suite *CLIVerifyTestSuite) runMigrationCommand(args []string) (string, error) {
	// Prepare migration command
	migrationArgs := []string{"./build/redis-valkey-migration-linux", "migrate"}
	migrationArgs = append(migrationArgs, args...)

	// Run migration in Docker network context
	dockerArgs := []string{"docker", "run", "--rm", "--network", "migration-cli-verify-test_test-network"}
	dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/app", suite.projectRoot))
	dockerArgs = append(dockerArgs, "-w", "/app")
	dockerArgs = append(dockerArgs, "golang:1.25-alpine")
	dockerArgs = append(dockerArgs, migrationArgs...)

	cmd := exec.Command(dockerArgs[0], dockerArgs[1:]...)
	output, err := cmd.CombinedOutput()

	outputStr := string(output)
	if err != nil {
		suite.T().Logf("Migration command failed: %s", outputStr)
		return outputStr, err
	}

	suite.T().Logf("Migration command succeeded: %s", outputStr)
	return outputStr, nil
}

func (suite *CLIVerifyTestSuite) populateTestData(ctx context.Context) {
	// String
	suite.redisClient.Set(ctx, "cli:verify:string", "hello world", 0)

	// Hash
	suite.redisClient.HSet(ctx, "cli:verify:hash", map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	})

	// List
	suite.redisClient.LPush(ctx, "cli:verify:list", "item2", "item1")

	// Set
	suite.redisClient.SAdd(ctx, "cli:verify:set", "member1", "member2")

	// Sorted Set
	suite.redisClient.ZAdd(ctx, "cli:verify:zset", redis.Z{Score: 1, Member: "member1"})
	suite.redisClient.ZAdd(ctx, "cli:verify:zset", redis.Z{Score: 2, Member: "member2"})
}

func (suite *CLIVerifyTestSuite) populateLargeDataset(ctx context.Context, keyCount int) {
	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("cli:verify:large:key:%d", i)
		value := fmt.Sprintf("value_%d", i)
		suite.redisClient.Set(ctx, key, value, 0)

		// Log progress every 50 keys
		if (i+1)%50 == 0 {
			suite.T().Logf("Populated %d/%d keys", i+1, keyCount)
		}
	}
}

func (suite *CLIVerifyTestSuite) corruptValkeyData(ctx context.Context) {
	// Corrupt string data
	suite.valkeyClient.Set(ctx, "cli:verify:string", "corrupted data", 0)

	// Corrupt hash data by adding extra field
	suite.valkeyClient.HSet(ctx, "cli:verify:hash", "extra_field", "extra_value")
}

func (suite *CLIVerifyTestSuite) verifyMigrationResults(ctx context.Context) {
	// Verify string
	val, err := suite.valkeyClient.Get(ctx, "cli:verify:string").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "hello world", val)

	// Verify hash
	hashVals, err := suite.valkeyClient.HGetAll(ctx, "cli:verify:hash").Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value1", hashVals["field1"])
	assert.Equal(suite.T(), "value2", hashVals["field2"])

	// Verify list
	listVals, err := suite.valkeyClient.LRange(ctx, "cli:verify:list", 0, -1).Result()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"item1", "item2"}, listVals)

	// Verify set
	setVals, err := suite.valkeyClient.SMembers(ctx, "cli:verify:set").Result()
	require.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"member1", "member2"}, setVals)

	// Verify sorted set
	zsetVals, err := suite.valkeyClient.ZRangeWithScores(ctx, "cli:verify:zset", 0, -1).Result()
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), zsetVals, 2)
}

func (suite *CLIVerifyTestSuite) cleanupResumeFiles() {
	resumeFiles := []string{
		"cli_verify_enabled_resume.json",
		"cli_verify_disabled_resume.json",
		"cli_verify_default_resume.json",
		"cli_verify_corrupted_initial_resume.json",
		"cli_verify_corrupted_verify_resume.json",
		"cli_verify_large_resume.json",
	}

	for _, file := range resumeFiles {
		os.Remove(file) // Ignore errors
	}
}

// TestCLIVerifyTestSuite runs the CLI verification test suite
func TestCLIVerifyTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI verify tests in short mode")
	}

	suite.Run(t, new(CLIVerifyTestSuite))
}
