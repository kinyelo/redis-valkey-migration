package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: redis-valkey-migration, Property 7: Comprehensive Logging**
// **Validates: Requirements 3.3, 7.1, 7.2, 7.3, 7.5**
func TestProperty_ComprehensiveLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: For any migration operation (connection, transfer, error, completion),
	// the Migration Tool should generate structured log entries with timestamps,
	// operation details, and appropriate log levels
	properties.Property("comprehensive logging generates structured entries", prop.ForAll(
		func(operation string, logLevel string, useJSON bool) bool {
			// Create temporary log file
			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "test.log")

			config := Config{
				Level:      logLevel,
				OutputFile: logFile,
				Format:     getFormat(useJSON),
				MaxSize:    0, // Use simple file output for testing
			}

			logger, err := NewLoggerFileOnly(config)
			if err != nil {
				t.Logf("Failed to create logger: %v", err)
				return false
			}

			// Perform various logging operations based on the operation type
			switch operation {
			case "connection":
				logger.LogConnection("connect", "localhost", 6379, 0, true, time.Millisecond*100)
			case "transfer":
				logger.LogKeyTransfer("test:key", "string", 1024, true, time.Millisecond*50, "")
			case "progress":
				logger.LogProgress(1000, 500, 10, 25.5)
			case "error":
				logger.LogError("transfer", "test:key", "connection timeout", "stack trace here", 2)
			case "summary":
				stats := MigrationStats{
					TotalKeys:        1000,
					SuccessfulKeys:   990,
					FailedKeys:       10,
					BytesTransferred: 1048576,
					Duration:         time.Minute * 5,
					Throughput:       3.3,
				}
				logger.LogSummary(stats)
			default:
				// Basic logging operations
				logger.Info("Test info message")
			}

			// Read the log file and verify structure
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Logf("Failed to read log file: %v", err)
				return false
			}

			if len(content) == 0 {
				t.Logf("Log file is empty")
				return false
			}

			logLines := strings.Split(strings.TrimSpace(string(content)), "\n")

			// Verify each log line has proper structure
			for _, line := range logLines {
				if strings.TrimSpace(line) == "" {
					continue
				}

				// Check if log entry has timestamp and proper structure
				if !verifyLogStructure(line, useJSON) {
					t.Logf("Log line does not have proper structure: %s", line)
					return false
				}

				// Verify operation-specific fields are present
				if !verifyOperationFields(line, operation, useJSON) {
					t.Logf("Log line missing operation-specific fields for %s: %s", operation, line)
					return false
				}
			}

			return true
		},
		gen.OneConstOf("connection", "transfer", "progress", "error", "summary", "basic"),
		gen.OneConstOf("debug", "info"), // Use levels that will show our Info-level logs
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// verifyLogStructure checks if a log line has proper timestamp and structure
func verifyLogStructure(logLine string, isJSON bool) bool {
	if isJSON {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLine), &logEntry); err != nil {
			return false
		}

		// Check for required fields
		if _, hasTime := logEntry["time"]; !hasTime {
			return false
		}
		if _, hasLevel := logEntry["level"]; !hasLevel {
			return false
		}
		if _, hasMsg := logEntry["msg"]; !hasMsg {
			return false
		}

		return true
	} else {
		// Text format should have timestamp at the beginning
		// Format: "time=2023-01-01T12:00:00Z level=info msg=..."
		return strings.Contains(logLine, "time=") &&
			strings.Contains(logLine, "level=") &&
			strings.Contains(logLine, "msg=")
	}
}

// verifyOperationFields checks if operation-specific fields are present
func verifyOperationFields(logLine string, operation string, isJSON bool) bool {
	var logEntry map[string]interface{}

	if isJSON {
		if err := json.Unmarshal([]byte(logLine), &logEntry); err != nil {
			return false
		}
	} else {
		// For text format, convert to map-like structure for easier checking
		logEntry = parseTextLogLine(logLine)
	}

	switch operation {
	case "connection":
		return hasFields(logEntry, []string{"operation", "host", "port", "database", "success", "duration"})
	case "transfer":
		return hasFields(logEntry, []string{"key", "data_type", "size", "success", "duration"})
	case "progress":
		return hasFields(logEntry, []string{"total_keys", "processed_keys", "failed_keys", "remaining_keys", "throughput"})
	case "error":
		return hasFields(logEntry, []string{"operation", "error_message", "retry_attempt"})
	case "summary":
		return hasFields(logEntry, []string{"total_keys", "successful_keys", "failed_keys", "bytes_transferred", "duration", "throughput"})
	default:
		// Basic operations just need the core structure
		return true
	}
}

// hasFields checks if all required fields are present in the log entry
func hasFields(logEntry map[string]interface{}, requiredFields []string) bool {
	for _, field := range requiredFields {
		if _, exists := logEntry[field]; !exists {
			return false
		}
	}
	return true
}

// parseTextLogLine converts a text log line to a map for easier field checking
func parseTextLogLine(logLine string) map[string]interface{} {
	result := make(map[string]interface{})

	// Simple parsing for key=value pairs
	parts := strings.Fields(logLine)
	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				result[kv[0]] = kv[1]
			}
		}
	}

	return result
}

// getFormat returns the appropriate format string
func getFormat(useJSON bool) string {
	if useJSON {
		return "json"
	}
	return "text"
}

// Test helper to verify log file creation and rotation
func TestProperty_LogFileCreationAndRotation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("log files are created with proper structure", prop.ForAll(
		func(maxSize int64, maxAge int) bool {
			if maxSize <= 0 {
				maxSize = 1024 // Minimum size for testing
			}
			if maxAge < 0 {
				maxAge = 1 // Minimum age
			}

			tmpDir := t.TempDir()
			logFile := filepath.Join(tmpDir, "rotation_test.log")

			config := Config{
				Level:      "info",
				OutputFile: logFile,
				Format:     "json",
				MaxSize:    maxSize,
				MaxAge:     maxAge,
			}

			logger, err := NewLogger(config)
			if err != nil {
				t.Logf("Failed to create logger: %v", err)
				return false
			}

			// Generate enough log entries to potentially trigger rotation
			for i := 0; i < 100; i++ {
				logger.Infof("Test log entry number %d with some additional content to increase size", i)
			}

			// Verify log file exists
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				t.Logf("Log file was not created: %s", logFile)
				return false
			}

			// Verify log file has content
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Logf("Failed to read log file: %v", err)
				return false
			}

			if len(content) == 0 {
				t.Logf("Log file is empty")
				return false
			}

			// Verify content is valid JSON (since we're using JSON format)
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}

				var logEntry map[string]interface{}
				if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
					t.Logf("Invalid JSON in log line: %s, error: %v", line, err)
					return false
				}
			}

			return true
		},
		gen.Int64Range(512, 10240), // maxSize between 512 bytes and 10KB
		gen.IntRange(1, 30),        // maxAge between 1 and 30 days
	))

	properties.TestingRun(t)
}
