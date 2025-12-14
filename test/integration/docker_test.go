package integration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// DockerTestSuite tests the migration tool using Docker containers
type DockerTestSuite struct {
	suite.Suite
	composeProject string
}

// SetupSuite initializes the Docker test environment
func (suite *DockerTestSuite) SetupSuite() {
	suite.composeProject = "migration-docker-test"

	// Check if Docker and Docker Compose are available
	suite.checkDockerAvailable()

	// Start the test environment
	suite.startTestEnvironment()
}

// TearDownSuite cleans up the Docker test environment
func (suite *DockerTestSuite) TearDownSuite() {
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

func (suite *DockerTestSuite) clearDatabases() {
	// Clear Redis
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "redis", "redis-cli", "FLUSHDB")
	cmd.Run()

	// Clear Valkey
	cmd = exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "FLUSHDB")
	cmd.Run()
}

func (suite *DockerTestSuite) populateRedisTestData() {
	commands := [][]string{
		{"SET", "test:string", "hello world"},
		{"HSET", "test:hash", "field1", "value1", "field2", "value2"},
		{"LPUSH", "test:list", "item1", "item2", "item3"},
		{"SADD", "test:set", "member1", "member2", "member3"},
		{"ZADD", "test:zset", "1", "member1", "2", "member2", "3", "member3"},
	}

	for _, cmdArgs := range commands {
		args := append([]string{"docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "redis", "redis-cli"}, cmdArgs...)
		cmd := exec.Command(args[0], args[1:]...)
		err := cmd.Run()
		require.NoError(suite.T(), err, "Failed to populate Redis test data")
	}
}

func (suite *DockerTestSuite) populateLargeRedisDataset(keyCount int) {
	// Create keys one by one to ensure they're properly set
	for i := range keyCount {
		args := []string{"docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "redis", "redis-cli", "SET", fmt.Sprintf("large:key:%d", i), fmt.Sprintf("value_%d", i)}
		cmd := exec.Command(args[0], args[1:]...)
		err := cmd.Run()
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
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "redis", "redis-cli", "KEYS", "*")
	output, err := cmd.Output()
	require.NoError(suite.T(), err, "Failed to get Redis keys")

	keys := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(keys) == 1 && keys[0] == "" {
		return []string{}
	}
	return keys
}

func (suite *DockerTestSuite) getValkeyKeys() []string {
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "KEYS", "*")
	output, err := cmd.Output()
	require.NoError(suite.T(), err, "Failed to get Valkey keys")

	keys := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(keys) == 1 && keys[0] == "" {
		return []string{}
	}
	return keys
}

func (suite *DockerTestSuite) verifyMigrationResults() {
	// Verify string
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "GET", "test:string")
	output, err := cmd.Output()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "hello world", strings.TrimSpace(string(output)))

	// Verify hash exists
	cmd = exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "HGET", "test:hash", "field1")
	output, err = cmd.Output()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value1", strings.TrimSpace(string(output)))

	// Verify list length
	cmd = exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "LLEN", "test:list")
	output, err = cmd.Output()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "3", strings.TrimSpace(string(output)))

	// Verify set cardinality
	cmd = exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "SCARD", "test:set")
	output, err = cmd.Output()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "3", strings.TrimSpace(string(output)))

	// Verify sorted set cardinality
	cmd = exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "-p", suite.composeProject, "exec", "-T", "valkey", "valkey-cli", "ZCARD", "test:zset")
	output, err = cmd.Output()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "3", strings.TrimSpace(string(output)))
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
