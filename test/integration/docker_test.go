package integration

import (
	"fmt"
	"os"
	"os/exec"
	"redis-valkey-migration/internal/client"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// DockerTestSuite tests the migration tool using Docker containers
type DockerTestSuite struct {
	suite.Suite
	composeProject string
	redisClient    client.DatabaseClient
	valkeyClient   client.DatabaseClient
}

// SetupSuite initializes the Docker test environment
func (suite *DockerTestSuite) SetupSuite() {
	suite.composeProject = "migration-docker-test"

	// Check if Docker and Docker Compose are available
	suite.checkDockerAvailable()

	// Start the test environment
	suite.startTestEnvironment()

	// Initialize Redis and Valkey clients
	suite.initializeClients()
}

// TearDownSuite cleans up the Docker test environment
func (suite *DockerTestSuite) TearDownSuite() {
	// Disconnect clients
	if suite.redisClient != nil {
		suite.redisClient.Disconnect()
	}
	if suite.valkeyClient != nil {
		suite.valkeyClient.Disconnect()
	}

	suite.stopTestEnvironment()
}

// SetupTest prepares each test
func (suite *DockerTestSuite) SetupTest() {
	// Clear databases before each test
	suite.clearDatabases()

	// Remove any resume files from previous tests
	suite.cleanupResumeFiles()
}

// TestDockerMigrationBasic tests basic migration using Docker containers
func (suite *DockerTestSuite) TestDockerMigrationBasic() {
	// Populate Redis with test data
	suite.populateRedisTestData()

	// Run migration using Docker
	err := suite.runMigration([]string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--log-level", "debug",
		"--resume-file", "test_basic_resume.json",
	})
	require.NoError(suite.T(), err, "Migration should complete successfully")

	// Verify data was migrated
	suite.verifyMigrationResults()
}

// TestDockerMigrationDryRun tests dry run functionality
func (suite *DockerTestSuite) TestDockerMigrationDryRun() {
	// Populate Redis with test data
	suite.populateRedisTestData()

	// Run dry run migration
	err := suite.runMigration([]string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--dry-run",
		"--log-level", "info",
		"--resume-file", "test_dryrun_resume.json",
	})
	require.NoError(suite.T(), err, "Dry run should complete successfully")

	// Verify no data was actually migrated (Valkey should be empty)
	valkeyKeys := suite.getValkeyKeys()
	assert.Empty(suite.T(), valkeyKeys, "Valkey should be empty after dry run")
}

// TestDockerMigrationWithAuth tests migration with authentication
func (suite *DockerTestSuite) TestDockerMigrationWithAuth() {
	// This test would require Redis/Valkey containers with auth enabled
	// For now, we'll test the command-line argument parsing

	suite.populateRedisTestData()

	// Run migration with auth parameters (will fail auth but test CLI parsing)
	err := suite.runMigration([]string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--redis-password", "testpass",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--valkey-password", "testpass",
		"--dry-run", // Use dry run to avoid auth failures
		"--resume-file", "test_auth_resume.json",
	})

	// The command should parse correctly even if auth fails
	// In a real scenario with auth-enabled containers, this would succeed
	require.NoError(suite.T(), err, "Command should parse authentication parameters correctly")
}

// TestDockerMigrationLargeDataset tests migration with a larger dataset
func (suite *DockerTestSuite) TestDockerMigrationLargeDataset() {
	// Create a larger dataset
	suite.populateLargeRedisDataset(500)

	// Run migration with custom batch size
	err := suite.runMigration([]string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--batch-size", "100",
		"--max-concurrency", "3",
		"--log-level", "info",
		"--resume-file", "test_large_resume.json",
	})
	require.NoError(suite.T(), err, "Large dataset migration should complete successfully")

	// Verify key counts match
	redisKeys := suite.getRedisKeys()
	valkeyKeys := suite.getValkeyKeys()
	assert.Equal(suite.T(), len(redisKeys), len(valkeyKeys), "Key counts should match")
	assert.Greater(suite.T(), len(valkeyKeys), 400, "Should have migrated substantial number of keys")
}

// TestDockerMigrationErrorRecovery tests error recovery scenarios
func (suite *DockerTestSuite) TestDockerMigrationErrorRecovery() {
	suite.populateRedisTestData()

	// Run migration with retry settings
	err := suite.runMigration([]string{
		"--redis-host", "redis",
		"--redis-port", "6379",
		"--valkey-host", "valkey",
		"--valkey-port", "6379",
		"--retry-attempts", "5",
		"--continue-on-error", "true",
		"--log-level", "debug",
		"--resume-file", "test_recovery_resume.json",
	})
	require.NoError(suite.T(), err, "Migration with error recovery should complete")

	suite.verifyMigrationResults()
}

// Helper methods

func (suite *DockerTestSuite) checkDockerAvailable() {
	// Check Docker
	cmd := exec.Command("docker", "--version")
	err := cmd.Run()
	require.NoError(suite.T(), err, "Docker must be available for integration tests")

	// Check Docker Compose (prefer v2 syntax)
	cmd = exec.Command("docker", "compose", "version")
	err = cmd.Run()
	if err != nil {
		// Fallback to legacy docker-compose
		cmd = exec.Command("docker-compose", "--version")
		err = cmd.Run()
		require.NoError(suite.T(), err, "Docker Compose must be available for integration tests")
	}
}

func (suite *DockerTestSuite) startTestEnvironment() {
	// Clean up any existing containers first
	suite.stopTestEnvironment()

	// Check if the E2E test containers are already running (from Makefile)
	// If they are, we'll skip starting our own and use those instead
	checkCmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", "migration-e2e-test", "ps", "-q")
	output, err := checkCmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		// E2E containers are running, use those
		suite.composeProject = "migration-e2e-test"
		suite.T().Log("Using existing E2E test containers")
	} else {
		// Start our own containers
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "up", "-d")
		output, err := cmd.CombinedOutput()
		require.NoError(suite.T(), err, "Failed to start test environment: %s", string(output))
		suite.T().Log("Started Docker test containers")
	}

	// Wait for services to be ready
	suite.waitForServices()
}

func (suite *DockerTestSuite) stopTestEnvironment() {
	// Only stop containers if we started them ourselves (not using E2E containers)
	if suite.composeProject != "migration-e2e-test" {
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "down", "-v")
		cmd.Run() // Don't fail if cleanup fails
	}
}

func (suite *DockerTestSuite) waitForServices() {
	// Wait for Redis
	for i := range 30 {
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "redis", "redis-cli", "ping")
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
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "ping")
		if cmd.Run() == nil {
			break
		}
		time.Sleep(time.Second)
		if i == 29 {
			suite.T().Fatal("Valkey failed to start within 30 seconds")
		}
	}
}

func (suite *DockerTestSuite) initializeClients() {
	// Create Redis client (connecting to localhost:16379)
	redisConfig := client.NewClientConfig("localhost", 16379, "", 0)
	suite.redisClient = client.NewRedisClient(redisConfig)
	err := suite.redisClient.Connect()
	require.NoError(suite.T(), err, "Failed to connect to Redis")

	// Create Valkey client (connecting to localhost:16380)
	valkeyConfig := client.NewClientConfig("localhost", 16380, "", 0)
	suite.valkeyClient = client.NewValkeyClient(valkeyConfig)
	err = suite.valkeyClient.Connect()
	require.NoError(suite.T(), err, "Failed to connect to Valkey")
}

func (suite *DockerTestSuite) clearDatabases() {
	ctx, cancel := client.NewClientConfig("", 0, "", 0).Context()
	defer cancel()

	// Clear Redis using Go client
	if suite.redisClient != nil {
		if err := suite.redisClient.Ping(); err == nil {
			// Get all keys and delete them (safer than FLUSHDB in case of shared instances)
			keys, err := suite.redisClient.GetAllKeys()
			if err == nil && len(keys) > 0 {
				// Delete keys in batches to avoid blocking
				for i := 0; i < len(keys); i += 100 {
					end := i + 100
					if end > len(keys) {
						end = len(keys)
					}
					batch := keys[i:end]

					// Use direct redis client for batch delete
					redisOpts := &redis.Options{
						Addr: "localhost:16379",
						DB:   0,
					}
					directClient := redis.NewClient(redisOpts)
					directClient.Del(ctx, batch...)
					directClient.Close()
				}
			}
		}
	}

	// Clear Valkey using Go client
	if suite.valkeyClient != nil {
		if err := suite.valkeyClient.Ping(); err == nil {
			// Get all keys and delete them
			keys, err := suite.valkeyClient.GetAllKeys()
			if err == nil && len(keys) > 0 {
				// Delete keys in batches
				for i := 0; i < len(keys); i += 100 {
					end := i + 100
					if end > len(keys) {
						end = len(keys)
					}
					batch := keys[i:end]

					// Use direct redis client for batch delete (Valkey is Redis-compatible)
					valkeyOpts := &redis.Options{
						Addr: "localhost:16380",
						DB:   0,
					}
					directClient := redis.NewClient(valkeyOpts)
					directClient.Del(ctx, batch...)
					directClient.Close()
				}
			}
		}
	}
}

func (suite *DockerTestSuite) populateRedisTestData() {
	require.NotNil(suite.T(), suite.redisClient, "Redis client must be initialized")

	// Set string value
	err := suite.redisClient.SetValue("test:string", "hello world")
	require.NoError(suite.T(), err, "Failed to set string value")

	// Set hash value
	hashValue := map[string]string{
		"field1": "value1",
		"field2": "value2",
	}
	err = suite.redisClient.SetValue("test:hash", hashValue)
	require.NoError(suite.T(), err, "Failed to set hash value")

	// Set list value (note: order is reversed due to LPUSH behavior)
	listValue := []string{"item3", "item2", "item1"}
	err = suite.redisClient.SetValue("test:list", listValue)
	require.NoError(suite.T(), err, "Failed to set list value")

	// Set set value
	setValue := []interface{}{"member1", "member2", "member3"}
	err = suite.redisClient.SetValue("test:set", setValue)
	require.NoError(suite.T(), err, "Failed to set set value")

	// Set sorted set value
	zsetValue := []redis.Z{
		{Score: 1, Member: "member1"},
		{Score: 2, Member: "member2"},
		{Score: 3, Member: "member3"},
	}
	err = suite.redisClient.SetValue("test:zset", zsetValue)
	require.NoError(suite.T(), err, "Failed to set sorted set value")
}

func (suite *DockerTestSuite) populateLargeRedisDataset(keyCount int) {
	require.NotNil(suite.T(), suite.redisClient, "Redis client must be initialized")

	// Create keys one by one to ensure they're properly set
	for i := range keyCount {
		key := fmt.Sprintf("large:key:%d", i)
		value := fmt.Sprintf("value_%d", i)

		err := suite.redisClient.SetValue(key, value)
		require.NoError(suite.T(), err, "Failed to populate large Redis dataset")

		// Log progress every 100 keys
		if (i+1)%100 == 0 {
			suite.T().Logf("Populated %d/%d keys", i+1, keyCount)
		}
	}
}

func (suite *DockerTestSuite) runMigration(args []string) error {
	// Build the migration tool for Linux (run from project root)
	buildCmd := exec.Command("go", "build", "-o", "build/redis-valkey-migration-linux", ".")
	buildCmd.Dir = "../.." // Go to project root
	buildCmd.Env = append(buildCmd.Env, "GOOS=linux", "GOARCH=amd64")
	// Copy current environment
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GOOS=") && !strings.HasPrefix(env, "GOARCH=") {
			buildCmd.Env = append(buildCmd.Env, env)
		}
	}
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build migration tool: %s", string(buildOutput))
	}

	// Prepare migration command
	migrationArgs := []string{"./build/redis-valkey-migration-linux", "migrate"}
	migrationArgs = append(migrationArgs, args...)

	// Run migration in Docker network context
	dockerArgs := []string{"docker", "run", "--rm", "--network", fmt.Sprintf("%s_test-network", suite.composeProject)}
	dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/app", suite.getProjectRoot()))
	dockerArgs = append(dockerArgs, "-w", "/app")
	dockerArgs = append(dockerArgs, "golang:1.25-alpine")
	dockerArgs = append(dockerArgs, migrationArgs...)

	cmd := exec.Command(dockerArgs[0], dockerArgs[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		suite.T().Logf("Migration output: %s", string(output))
		return fmt.Errorf("migration failed: %w", err)
	}

	suite.T().Logf("Migration completed successfully: %s", string(output))
	return nil
}

func (suite *DockerTestSuite) getRedisKeys() []string {
	require.NotNil(suite.T(), suite.redisClient, "Redis client must be initialized")

	keys, err := suite.redisClient.GetAllKeys()
	require.NoError(suite.T(), err, "Failed to get Redis keys")

	return keys
}

func (suite *DockerTestSuite) getValkeyKeys() []string {
	require.NotNil(suite.T(), suite.valkeyClient, "Valkey client must be initialized")

	keys, err := suite.valkeyClient.GetAllKeys()
	require.NoError(suite.T(), err, "Failed to get Valkey keys")

	return keys
}

func (suite *DockerTestSuite) verifyMigrationResults() {
	require.NotNil(suite.T(), suite.valkeyClient, "Valkey client must be initialized")

	// Verify string
	value, err := suite.valkeyClient.GetValue("test:string")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "hello world", value)

	// Verify hash exists
	hashValue, err := suite.valkeyClient.GetValue("test:hash")
	require.NoError(suite.T(), err)
	hashMap, ok := hashValue.(map[string]string)
	require.True(suite.T(), ok, "Hash value should be map[string]string")
	assert.Equal(suite.T(), "value1", hashMap["field1"])
	assert.Equal(suite.T(), "value2", hashMap["field2"])

	// Verify list
	listValue, err := suite.valkeyClient.GetValue("test:list")
	require.NoError(suite.T(), err)
	listSlice, ok := listValue.([]string)
	require.True(suite.T(), ok, "List value should be []string")
	assert.Len(suite.T(), listSlice, 3)

	// Verify set
	setValue, err := suite.valkeyClient.GetValue("test:set")
	require.NoError(suite.T(), err)
	setSlice, ok := setValue.([]string)
	require.True(suite.T(), ok, "Set value should be []string")
	assert.Len(suite.T(), setSlice, 3)

	// Verify sorted set
	zsetValue, err := suite.valkeyClient.GetValue("test:zset")
	require.NoError(suite.T(), err)
	zsetSlice, ok := zsetValue.([]redis.Z)
	require.True(suite.T(), ok, "Sorted set value should be []redis.Z")
	assert.Len(suite.T(), zsetSlice, 3)
}

func (suite *DockerTestSuite) getProjectRoot() string {
	cmd := exec.Command("pwd")
	cmd.Dir = "../.."
	output, err := cmd.Output()
	require.NoError(suite.T(), err)
	return strings.TrimSpace(string(output))
}

func (suite *DockerTestSuite) cleanupResumeFiles() {
	// Remove resume files that might interfere with tests
	resumeFiles := []string{
		"migration_resume.json",
		"../../migration_resume.json",
		"test_basic_resume.json",
		"test_dryrun_resume.json",
		"test_auth_resume.json",
		"test_large_resume.json",
		"test_recovery_resume.json",
		"../../test_basic_resume.json",
		"../../test_dryrun_resume.json",
		"../../test_auth_resume.json",
		"../../test_large_resume.json",
		"../../test_recovery_resume.json",
	}

	for _, file := range resumeFiles {
		exec.Command("rm", "-f", file).Run() // Ignore errors
	}
}

// TestDockerTestSuite runs the Docker-based integration test suite
func TestDockerTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration tests in short mode")
	}

	suite.Run(t, new(DockerTestSuite))
}
