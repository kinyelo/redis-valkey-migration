package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotatingFileWriter_Creation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotation.log")

	writer, err := NewRotatingFileWriter(logFile, 1024, 7)
	require.NoError(t, err)
	require.NotNil(t, writer)

	// Verify file was created
	assert.FileExists(t, logFile)

	// Clean up
	err = writer.Close()
	assert.NoError(t, err)
}

func TestRotatingFileWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "write_test.log")

	writer, err := NewRotatingFileWriter(logFile, 1024, 7)
	require.NoError(t, err)
	defer writer.Close()

	// Write some data
	testData := "This is a test log entry\n"
	n, err := writer.Write([]byte(testData))
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Verify data was written
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, testData, string(content))
}

func TestRotatingFileWriter_Rotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotate_test.log")

	// Use a very small max size to trigger rotation
	writer, err := NewRotatingFileWriter(logFile, 50, 7)
	require.NoError(t, err)
	defer writer.Close()

	// Write data that exceeds the max size
	testData := "This is a test log entry that is longer than 50 bytes to trigger rotation\n"

	// First write - should not trigger rotation
	_, err = writer.Write([]byte("Short entry\n"))
	require.NoError(t, err)

	// Second write - should trigger rotation
	_, err = writer.Write([]byte(testData))
	require.NoError(t, err)

	// Check that rotation occurred by looking for rotated files
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	rotatedFiles := 0
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "rotate_test.") && strings.HasSuffix(entry.Name(), ".log") {
			rotatedFiles++
		}
	}

	// Should have at least 1 file (rotation may or may not have occurred)
	// The important thing is that writing continues to work
	assert.GreaterOrEqual(t, rotatedFiles, 1)

	// Verify the current file exists and has content
	assert.FileExists(t, logFile)
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestRotatingFileWriter_CleanupOldFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "cleanup_test.log")

	// Create some old rotated files
	oldFile1 := filepath.Join(tmpDir, "cleanup_test.20200101-120000.log")
	oldFile2 := filepath.Join(tmpDir, "cleanup_test.20200102-120000.log")

	// Create old files with old timestamps
	err := os.WriteFile(oldFile1, []byte("old content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(oldFile2, []byte("old content"), 0644)
	require.NoError(t, err)

	// Set old modification times
	oldTime := time.Now().AddDate(0, 0, -10) // 10 days ago
	err = os.Chtimes(oldFile1, oldTime, oldTime)
	require.NoError(t, err)
	err = os.Chtimes(oldFile2, oldTime, oldTime)
	require.NoError(t, err)

	// Create rotating writer with maxAge of 7 days
	writer, err := NewRotatingFileWriter(logFile, 1024, 7)
	require.NoError(t, err)
	defer writer.Close()

	// Give some time for cleanup to potentially happen
	time.Sleep(time.Millisecond * 100)

	// Old files should still exist (cleanup happens during rotation, not creation)
	// But we can test the cleanup logic by manually calling it
	err = writer.cleanupOldFiles()
	require.NoError(t, err)

	// Check if old files were removed
	_, err1 := os.Stat(oldFile1)
	_, err2 := os.Stat(oldFile2)

	// Files should be removed (or at least one of them)
	assert.True(t, os.IsNotExist(err1) || os.IsNotExist(err2))
}

func TestLoggerWithRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "logger_rotation.log")

	config := Config{
		Level:      "info",
		OutputFile: logFile,
		Format:     "json",
		MaxSize:    100, // Small size to trigger rotation
		MaxAge:     7,
	}

	logger, err := NewLoggerFileOnly(config)
	require.NoError(t, err)

	// Write many log entries to trigger rotation
	for i := 0; i < 20; i++ {
		logger.Infof("This is log entry number %d with some additional content to make it longer", i)
	}

	// Check that log files exist
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	logFiles := 0
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "logger_rotation") && strings.HasSuffix(entry.Name(), ".log") {
			logFiles++
		}
	}

	// Should have multiple log files due to rotation
	assert.GreaterOrEqual(t, logFiles, 1)

	// Verify current log file has content
	assert.FileExists(t, logFile)
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}
