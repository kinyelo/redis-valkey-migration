package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger interface defines the logging contract
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger

	LogConnection(operation, host string, port int, database int, success bool, duration time.Duration)
	LogKeyTransfer(key, dataType string, size int64, success bool, duration time.Duration, errorMsg string)
	LogProgress(totalKeys, processedKeys, failedKeys int, throughput float64)
	LogError(operation, key, errorMsg, stackTrace string, retryAttempt int)
	LogSummary(stats MigrationStats)
}

// MigrationStats represents migration statistics for logging
type MigrationStats struct {
	TotalKeys        int
	SuccessfulKeys   int
	FailedKeys       int
	BytesTransferred int64
	Duration         time.Duration
	Throughput       float64
}

// Config holds logger configuration
type Config struct {
	Level      string
	OutputFile string
	MaxSize    int64  // Maximum size in bytes before rotation
	MaxAge     int    // Maximum age in days
	Format     string // "json" or "text"
}

// migrationLogger implements the Logger interface using logrus
type migrationLogger struct {
	logger *logrus.Logger
	config Config
}

// entryLogger implements the Logger interface using logrus.Entry (for WithField/WithFields)
type entryLogger struct {
	entry  *logrus.Entry
	config Config
}

// NewLogger creates a new logger instance with the specified configuration
func NewLogger(config Config) (Logger, error) {
	return newLoggerWithOutput(config, false)
}

// NewLoggerFileOnly creates a logger that only writes to file (for testing)
func NewLoggerFileOnly(config Config) (Logger, error) {
	return newLoggerWithOutput(config, true)
}

// newLoggerWithOutput creates a logger with optional file-only output
func newLoggerWithOutput(config Config, fileOnly bool) (Logger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", config.Level, err)
	}
	logger.SetLevel(level)

	// Set formatter
	if config.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}

	// Set output
	if config.OutputFile != "" {
		// Use rotating file writer if MaxSize is specified
		if config.MaxSize > 0 {
			rotatingWriter, err := NewRotatingFileWriter(config.OutputFile, config.MaxSize, config.MaxAge)
			if err != nil {
				return nil, fmt.Errorf("failed to create rotating file writer: %w", err)
			}

			// Use rotating file output (and optionally stdout)
			if fileOnly {
				logger.SetOutput(rotatingWriter)
			} else {
				logger.SetOutput(io.MultiWriter(os.Stdout, rotatingWriter))
			}
		} else {
			// Use simple file output
			if err := ensureLogDir(config.OutputFile); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}

			file, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file: %w", err)
			}

			// Use file output (and optionally stdout)
			if fileOnly {
				logger.SetOutput(file)
			} else {
				logger.SetOutput(io.MultiWriter(os.Stdout, file))
			}
		}
	}

	return &migrationLogger{
		logger: logger,
		config: config,
	}, nil
}

// ensureLogDir creates the directory for the log file if it doesn't exist
func ensureLogDir(logFile string) error {
	dir := filepath.Dir(logFile)
	return os.MkdirAll(dir, 0755)
}

// Basic logging methods
func (l *migrationLogger) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *migrationLogger) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *migrationLogger) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

func (l *migrationLogger) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *migrationLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *migrationLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *migrationLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *migrationLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

func (l *migrationLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *migrationLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

func (l *migrationLogger) WithField(key string, value interface{}) Logger {
	return &entryLogger{
		entry:  l.logger.WithField(key, value),
		config: l.config,
	}
}

func (l *migrationLogger) WithFields(fields map[string]interface{}) Logger {
	return &entryLogger{
		entry:  l.logger.WithFields(fields),
		config: l.config,
	}
}

// Specialized logging methods for migration operations

// LogConnection logs database connection operations
func (l *migrationLogger) LogConnection(operation, host string, port int, database int, success bool, duration time.Duration) {
	fields := logrus.Fields{
		"operation": operation,
		"host":      host,
		"port":      port,
		"database":  database,
		"success":   success,
		"duration":  duration.String(),
	}

	if success {
		l.logger.WithFields(fields).Info("Database connection operation completed")
	} else {
		l.logger.WithFields(fields).Error("Database connection operation failed")
	}
}

// LogKeyTransfer logs individual key transfer operations
func (l *migrationLogger) LogKeyTransfer(key, dataType string, size int64, success bool, duration time.Duration, errorMsg string) {
	fields := logrus.Fields{
		"key":       key,
		"data_type": dataType,
		"size":      size,
		"success":   success,
		"duration":  duration.String(),
	}

	if errorMsg != "" {
		fields["error"] = errorMsg
	}

	if success {
		l.logger.WithFields(fields).Info("Key transfer completed")
	} else {
		l.logger.WithFields(fields).Error("Key transfer failed")
	}
}

// LogProgress logs migration progress information
func (l *migrationLogger) LogProgress(totalKeys, processedKeys, failedKeys int, throughput float64) {
	fields := logrus.Fields{
		"total_keys":     totalKeys,
		"processed_keys": processedKeys,
		"failed_keys":    failedKeys,
		"remaining_keys": totalKeys - processedKeys,
		"throughput":     fmt.Sprintf("%.2f keys/sec", throughput),
		"progress_pct":   fmt.Sprintf("%.2f%%", float64(processedKeys)/float64(totalKeys)*100),
	}

	l.logger.WithFields(fields).Info("Migration progress update")
}

// LogError logs detailed error information with context
func (l *migrationLogger) LogError(operation, key, errorMsg, stackTrace string, retryAttempt int) {
	fields := logrus.Fields{
		"operation":     operation,
		"error_message": errorMsg,
		"retry_attempt": retryAttempt,
	}

	if key != "" {
		fields["key"] = key
	}

	if stackTrace != "" {
		fields["stack_trace"] = stackTrace
	}

	l.logger.WithFields(fields).Error("Operation error occurred")
}

// LogSummary logs comprehensive migration summary statistics
func (l *migrationLogger) LogSummary(stats MigrationStats) {
	fields := logrus.Fields{
		"total_keys":        stats.TotalKeys,
		"successful_keys":   stats.SuccessfulKeys,
		"failed_keys":       stats.FailedKeys,
		"bytes_transferred": stats.BytesTransferred,
		"duration":          stats.Duration.String(),
		"throughput":        fmt.Sprintf("%.2f keys/sec", stats.Throughput),
		"success_rate":      fmt.Sprintf("%.2f%%", float64(stats.SuccessfulKeys)/float64(stats.TotalKeys)*100),
	}

	l.logger.WithFields(fields).Info("Migration completed - Summary statistics")
}

// entryLogger methods - implement Logger interface for logrus.Entry

func (e *entryLogger) Debug(args ...interface{}) {
	e.entry.Debug(args...)
}

func (e *entryLogger) Info(args ...interface{}) {
	e.entry.Info(args...)
}

func (e *entryLogger) Warn(args ...interface{}) {
	e.entry.Warn(args...)
}

func (e *entryLogger) Error(args ...interface{}) {
	e.entry.Error(args...)
}

func (e *entryLogger) Fatal(args ...interface{}) {
	e.entry.Fatal(args...)
}

func (e *entryLogger) Debugf(format string, args ...interface{}) {
	e.entry.Debugf(format, args...)
}

func (e *entryLogger) Infof(format string, args ...interface{}) {
	e.entry.Infof(format, args...)
}

func (e *entryLogger) Warnf(format string, args ...interface{}) {
	e.entry.Warnf(format, args...)
}

func (e *entryLogger) Errorf(format string, args ...interface{}) {
	e.entry.Errorf(format, args...)
}

func (e *entryLogger) Fatalf(format string, args ...interface{}) {
	e.entry.Fatalf(format, args...)
}

func (e *entryLogger) WithField(key string, value interface{}) Logger {
	return &entryLogger{
		entry:  e.entry.WithField(key, value),
		config: e.config,
	}
}

func (e *entryLogger) WithFields(fields map[string]interface{}) Logger {
	return &entryLogger{
		entry:  e.entry.WithFields(fields),
		config: e.config,
	}
}

func (e *entryLogger) LogConnection(operation, host string, port int, database int, success bool, duration time.Duration) {
	fields := logrus.Fields{
		"operation": operation,
		"host":      host,
		"port":      port,
		"database":  database,
		"success":   success,
		"duration":  duration.String(),
	}

	if success {
		e.entry.WithFields(fields).Info("Database connection operation completed")
	} else {
		e.entry.WithFields(fields).Error("Database connection operation failed")
	}
}

func (e *entryLogger) LogKeyTransfer(key, dataType string, size int64, success bool, duration time.Duration, errorMsg string) {
	fields := logrus.Fields{
		"key":       key,
		"data_type": dataType,
		"size":      size,
		"success":   success,
		"duration":  duration.String(),
	}

	if errorMsg != "" {
		fields["error"] = errorMsg
	}

	if success {
		e.entry.WithFields(fields).Info("Key transfer completed")
	} else {
		e.entry.WithFields(fields).Error("Key transfer failed")
	}
}

func (e *entryLogger) LogProgress(totalKeys, processedKeys, failedKeys int, throughput float64) {
	fields := logrus.Fields{
		"total_keys":     totalKeys,
		"processed_keys": processedKeys,
		"failed_keys":    failedKeys,
		"remaining_keys": totalKeys - processedKeys,
		"throughput":     fmt.Sprintf("%.2f keys/sec", throughput),
		"progress_pct":   fmt.Sprintf("%.2f%%", float64(processedKeys)/float64(totalKeys)*100),
	}

	e.entry.WithFields(fields).Info("Migration progress update")
}

func (e *entryLogger) LogError(operation, key, errorMsg, stackTrace string, retryAttempt int) {
	fields := logrus.Fields{
		"operation":     operation,
		"error_message": errorMsg,
		"retry_attempt": retryAttempt,
	}

	if key != "" {
		fields["key"] = key
	}

	if stackTrace != "" {
		fields["stack_trace"] = stackTrace
	}

	e.entry.WithFields(fields).Error("Operation error occurred")
}

func (e *entryLogger) LogSummary(stats MigrationStats) {
	fields := logrus.Fields{
		"total_keys":        stats.TotalKeys,
		"successful_keys":   stats.SuccessfulKeys,
		"failed_keys":       stats.FailedKeys,
		"bytes_transferred": stats.BytesTransferred,
		"duration":          stats.Duration.String(),
		"throughput":        fmt.Sprintf("%.2f keys/sec", stats.Throughput),
		"success_rate":      fmt.Sprintf("%.2f%%", float64(stats.SuccessfulKeys)/float64(stats.TotalKeys)*100),
	}

	e.entry.WithFields(fields).Info("Migration completed - Summary statistics")
}
