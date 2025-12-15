package integration

import (
	"testing"

	"github.com/kinyelo/redis-valkey-migration/internal/engine"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyFlagParsing tests that the --verify flag is properly parsed
func TestVerifyFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedVerify bool
		shouldError    bool
	}{
		{
			name:           "verify flag enabled explicitly",
			args:           []string{"--verify=true"},
			expectedVerify: true,
			shouldError:    false,
		},
		{
			name:           "verify flag disabled explicitly",
			args:           []string{"--verify=false"},
			expectedVerify: false,
			shouldError:    false,
		},
		{
			name:           "verify flag enabled with equals sign",
			args:           []string{"--verify=true"},
			expectedVerify: true,
			shouldError:    false,
		},
		{
			name:           "verify flag disabled with equals sign",
			args:           []string{"--verify=false"},
			expectedVerify: false,
			shouldError:    false,
		},
		{
			name:           "verify flag not specified (should default to true)",
			args:           []string{},
			expectedVerify: true,
			shouldError:    false,
		},
		{
			name:           "verify flag with other flags",
			args:           []string{"--redis-host", "localhost", "--verify=false", "--batch-size", "100"},
			expectedVerify: false,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test command similar to the migrate command
			cmd := &cobra.Command{
				Use: "test-migrate",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Create engine config to test the flag value (same logic as main.go)
					engineConfig := engine.DefaultEngineConfig()

					// Override with command-line flags if they were changed
					verify, err := cmd.Flags().GetBool("verify")
					if err != nil {
						return err
					}

					changed := cmd.Flags().Changed("verify")

					if changed {
						engineConfig.VerifyAfterMigration = verify
					}

					// Assert the expected value
					assert.Equal(t, tt.expectedVerify, engineConfig.VerifyAfterMigration)
					return nil
				},
			}

			// Add the verify flag (same as in main.go)
			cmd.Flags().Bool("verify", true, "verify data integrity after migration completion")

			// Add other flags that might be used in tests
			cmd.Flags().String("redis-host", "localhost", "Redis host")
			cmd.Flags().Int("batch-size", 1000, "Batch size")

			// Set the arguments and execute
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestVerifyFlagInEngineConfig tests that the verify flag properly affects engine configuration
func TestVerifyFlagInEngineConfig(t *testing.T) {
	tests := []struct {
		name           string
		flagValue      bool
		flagChanged    bool
		expectedVerify bool
	}{
		{
			name:           "flag set to true",
			flagValue:      true,
			flagChanged:    true,
			expectedVerify: true,
		},
		{
			name:           "flag set to false",
			flagValue:      false,
			flagChanged:    true,
			expectedVerify: false,
		},
		{
			name:           "flag not changed (default behavior)",
			flagValue:      true, // This is the default value
			flagChanged:    false,
			expectedVerify: true, // Should use default from DefaultEngineConfig
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock command
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().Bool("verify", true, "verify data integrity")

			if tt.flagChanged {
				// Simulate the flag being set
				err := cmd.Flags().Set("verify", "true")
				if tt.flagValue {
					err = cmd.Flags().Set("verify", "true")
				} else {
					err = cmd.Flags().Set("verify", "false")
				}
				require.NoError(t, err)
			}

			// Simulate the engine config creation logic from main.go
			engineConfig := engine.DefaultEngineConfig()
			if verify, err := cmd.Flags().GetBool("verify"); err == nil && cmd.Flags().Changed("verify") {
				engineConfig.VerifyAfterMigration = verify
			}

			assert.Equal(t, tt.expectedVerify, engineConfig.VerifyAfterMigration)
		})
	}
}

// TestDefaultEngineConfigVerifyValue tests that the default engine config has verification enabled
func TestDefaultEngineConfigVerifyValue(t *testing.T) {
	config := engine.DefaultEngineConfig()
	assert.True(t, config.VerifyAfterMigration, "Default engine config should have verification enabled")
}

// TestEngineConfigVerifyField tests that the VerifyAfterMigration field can be set correctly
func TestEngineConfigVerifyField(t *testing.T) {
	config := engine.DefaultEngineConfig()

	// Test setting to false
	config.VerifyAfterMigration = false
	assert.False(t, config.VerifyAfterMigration)

	// Test setting to true
	config.VerifyAfterMigration = true
	assert.True(t, config.VerifyAfterMigration)
}
