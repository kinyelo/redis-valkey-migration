package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config with file output",
			config: Config{
				Level:      "info",
				OutputFile: filepath.Join(t.TempDir(), "test.log"),
				Format:     "json",
			},
			expectError: false,
		},
		{
			name: "valid config without file output",
			config: Config{
				Level:  "debug",
				Format: "text",
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			config: Config{
				Level:  "invalid",
				Format: "json",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
			}
		})
	}
}

func TestLogFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := Config{
		Level:      "info",
		OutputFile: logFile,
		Format:     "json",
	}

	logger, err := NewLoggerFileOnly(config)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Log a message
	logger.Info("Test message")

	// Verify file was created
	assert.FileExists(t, logFile)

	// Verify file has content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// Verify content is valid JSON
	var logEntry map[string]interface{}
	err = json.Unmarshal(content, &logEntry)
	assert.NoError(t, err)

	// Verify required fields
	assert.Contains(t, logEntry, "time")
	assert.Contains(t, logEntry, "level")
	assert.Contains(t, logEntry, "msg")
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "Test message", logEntry["msg"])
}

func TestLogLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "levels.log")

	config := Config{
		Level:      "warn", // Only warn and error should be logged
		OutputFile: logFile,
		Format:     "json",
	}

	logger, err := NewLoggerFileOnly(config)
	require.NoError(t, err)

	// Log messages at different levels
	logger.Debug("Debug message")  // Should not appear
	logger.Info("Info message")    // Should not appear
	logger.Warn("Warning message") // Should appear
	logger.Error("Error message")  // Should appear

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Should only have 2 lines (warn and error)
	assert.Len(t, lines, 2)

	// Verify warn message
	var warnEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[0]), &warnEntry)
	require.NoError(t, err)
	assert.Equal(t, "warning", warnEntry["level"])
	assert.Equal(t, "Warning message", warnEntry["msg"])

	// Verify error message
	var errorEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[1]), &errorEntry)
	require.NoError(t, err)
	assert.Equal(t, "error", errorEntry["level"])
	assert.Equal(t, "Error message", errorEntry["msg"])
}

func TestLogFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"JSON format", "json"},
		{"Text format", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "format_test.log")

			config := Config{
				Level:      "info",
				OutputFile: logFile,
				Format:     tt.format,
			}

			logger, err := NewLoggerFileOnly(config)
			require.NoError(t, err)

			logger.Info("Test message")

			content, err := os.ReadFile(logFile)
			require.NoError(t, err)
			assert.NotEmpty(t, content)

			if tt.format == "json" {
				// Should be valid JSON
				var logEntry map[string]interface{}
				err = json.Unmarshal(content, &logEntry)
				assert.NoError(t, err)
			} else {
				// Text format should contain key=value pairs
				contentStr := string(content)
				assert.Contains(t, contentStr, "time=")
				assert.Contains(t, contentStr, "level=info")
				assert.Contains(t, contentStr, "msg=")
			}
		})
	}
}

func TestSpecializedLoggingMethods(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "specialized.log")

	config := Config{
		Level:      "info",
		OutputFile: logFile,
		Format:     "json",
	}

	logger, err := NewLoggerFileOnly(config)
	require.NoError(t, err)

	// Test LogConnection
	logger.LogConnection("connect", "localhost", 6379, 0, true, time.Millisecond*100)

	// Test LogKeyTransfer
	logger.LogKeyTransfer("test:key", "string", 1024, true, time.Millisecond*50, "")

	// Test LogProgress
	logger.LogProgress(1000, 500, 10, 25.5)

	// Test LogError
	logger.LogError("transfer", "test:key", "connection timeout", "stack trace", 2)

	// Test LogSummary
	stats := MigrationStats{
		TotalKeys:        1000,
		SuccessfulKeys:   990,
		FailedKeys:       10,
		BytesTransferred: 1048576,
		Duration:         time.Minute * 5,
		Throughput:       3.3,
	}
	logger.LogSummary(stats)

	// Read and verify log entries
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Len(t, lines, 5) // Should have 5 log entries

	// Verify LogConnection entry
	var connEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[0]), &connEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", connEntry["level"])
	assert.Contains(t, connEntry, "operation")
	assert.Contains(t, connEntry, "host")
	assert.Contains(t, connEntry, "port")
	assert.Contains(t, connEntry, "database")
	assert.Contains(t, connEntry, "success")
	assert.Contains(t, connEntry, "duration")

	// Verify LogKeyTransfer entry
	var transferEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[1]), &transferEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", transferEntry["level"])
	assert.Contains(t, transferEntry, "key")
	assert.Contains(t, transferEntry, "data_type")
	assert.Contains(t, transferEntry, "size")
	assert.Contains(t, transferEntry, "success")
	assert.Contains(t, transferEntry, "duration")

	// Verify LogProgress entry
	var progressEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[2]), &progressEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", progressEntry["level"])
	assert.Contains(t, progressEntry, "total_keys")
	assert.Contains(t, progressEntry, "processed_keys")
	assert.Contains(t, progressEntry, "failed_keys")
	assert.Contains(t, progressEntry, "remaining_keys")
	assert.Contains(t, progressEntry, "throughput")

	// Verify LogError entry
	var errorEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[3]), &errorEntry)
	require.NoError(t, err)
	assert.Equal(t, "error", errorEntry["level"])
	assert.Contains(t, errorEntry, "operation")
	assert.Contains(t, errorEntry, "error_message")
	assert.Contains(t, errorEntry, "retry_attempt")

	// Verify LogSummary entry
	var summaryEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[4]), &summaryEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", summaryEntry["level"])
	assert.Contains(t, summaryEntry, "total_keys")
	assert.Contains(t, summaryEntry, "successful_keys")
	assert.Contains(t, summaryEntry, "failed_keys")
	assert.Contains(t, summaryEntry, "bytes_transferred")
	assert.Contains(t, summaryEntry, "duration")
	assert.Contains(t, summaryEntry, "throughput")
}

func TestWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "fields.log")

	config := Config{
		Level:      "info",
		OutputFile: logFile,
		Format:     "json",
	}

	logger, err := NewLoggerFileOnly(config)
	require.NoError(t, err)

	// Test WithField
	logger.WithField("key", "value").Info("Test with field")

	// Test WithFields
	fields := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	}
	logger.WithFields(fields).Info("Test with multiple fields")

	// Read and verify log entries
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Len(t, lines, 2)

	// Verify WithField entry
	var fieldEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[0]), &fieldEntry)
	require.NoError(t, err)
	assert.Equal(t, "value", fieldEntry["key"])

	// Verify WithFields entry
	var fieldsEntry map[string]interface{}
	err = json.Unmarshal([]byte(lines[1]), &fieldsEntry)
	require.NoError(t, err)
	assert.Equal(t, "value1", fieldsEntry["field1"])
	assert.Equal(t, float64(42), fieldsEntry["field2"]) // JSON numbers are float64
	assert.Equal(t, true, fieldsEntry["field3"])
}
