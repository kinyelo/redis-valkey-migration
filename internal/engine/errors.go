package engine

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorType represents different categories of errors
type ErrorType int

const (
	// ConnectionError represents database connection failures
	ConnectionError ErrorType = iota
	// NetworkError represents network-related failures
	NetworkError
	// AuthenticationError represents authentication failures
	AuthenticationError
	// DataError represents data processing failures
	DataError
	// ConfigurationError represents configuration-related failures
	ConfigurationError
	// CriticalError represents critical system failures
	CriticalError
)

// String returns string representation of ErrorType
func (et ErrorType) String() string {
	switch et {
	case ConnectionError:
		return "ConnectionError"
	case NetworkError:
		return "NetworkError"
	case AuthenticationError:
		return "AuthenticationError"
	case DataError:
		return "DataError"
	case ConfigurationError:
		return "ConfigurationError"
	case CriticalError:
		return "CriticalError"
	default:
		return "UnknownError"
	}
}

// MigrationError represents a structured error with context
type MigrationError struct {
	Type      ErrorType         `json:"type"`
	Operation string            `json:"operation"`
	Key       string            `json:"key,omitempty"`
	Message   string            `json:"message"`
	Cause     error             `json:"cause,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Retryable bool              `json:"retryable"`
}

// Error implements the error interface
func (me *MigrationError) Error() string {
	if me.Key != "" {
		return fmt.Sprintf("%s in %s for key '%s': %s", me.Type.String(), me.Operation, me.Key, me.Message)
	}
	return fmt.Sprintf("%s in %s: %s", me.Type.String(), me.Operation, me.Message)
}

// Unwrap returns the underlying cause error
func (me *MigrationError) Unwrap() error {
	return me.Cause
}

// NewMigrationError creates a new migration error
func NewMigrationError(errorType ErrorType, operation, message string) *MigrationError {
	return &MigrationError{
		Type:      errorType,
		Operation: operation,
		Message:   message,
		Metadata:  make(map[string]string),
		Retryable: isRetryableByType(errorType),
	}
}

// WithKey adds key context to the error
func (me *MigrationError) WithKey(key string) *MigrationError {
	me.Key = key
	return me
}

// WithCause adds the underlying cause error
func (me *MigrationError) WithCause(err error) *MigrationError {
	me.Cause = err
	return me
}

// WithMetadata adds metadata to the error
func (me *MigrationError) WithMetadata(key, value string) *MigrationError {
	if me.Metadata == nil {
		me.Metadata = make(map[string]string)
	}
	me.Metadata[key] = value
	return me
}

// WithRetryable sets whether the error is retryable
func (me *MigrationError) WithRetryable(retryable bool) *MigrationError {
	me.Retryable = retryable
	return me
}

// isRetryableByType determines if an error type is generally retryable
func isRetryableByType(errorType ErrorType) bool {
	switch errorType {
	case ConnectionError, NetworkError:
		return true
	case AuthenticationError, ConfigurationError, CriticalError:
		return false
	case DataError:
		return false // Data errors usually indicate corruption or format issues
	default:
		return false
	}
}

// ClassifyError analyzes an error and returns its type
func ClassifyError(err error) ErrorType {
	if err == nil {
		return CriticalError // This shouldn't happen
	}

	errStr := strings.ToLower(err.Error())

	// Connection-related errors
	connectionPatterns := []string{
		"connection refused",
		"connection reset",
		"connection lost",
		"connection closed",
		"no connection",
		"dial tcp",
		"connect:",
	}

	for _, pattern := range connectionPatterns {
		if strings.Contains(errStr, pattern) {
			return ConnectionError
		}
	}

	// Network-related errors
	networkPatterns := []string{
		"timeout",
		"network",
		"i/o timeout",
		"broken pipe",
		"host unreachable",
		"no route to host",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errStr, pattern) {
			return NetworkError
		}
	}

	// Authentication errors
	authPatterns := []string{
		"auth",
		"authentication",
		"unauthorized",
		"invalid password",
		"access denied",
		"permission denied",
	}

	for _, pattern := range authPatterns {
		if strings.Contains(errStr, pattern) {
			return AuthenticationError
		}
	}

	// Configuration errors
	configPatterns := []string{
		"invalid configuration",
		"config",
		"invalid host",
		"invalid port",
		"invalid database",
	}

	for _, pattern := range configPatterns {
		if strings.Contains(errStr, pattern) {
			return ConfigurationError
		}
	}

	// Data processing errors
	dataPatterns := []string{
		"invalid data",
		"corrupt",
		"parse error",
		"invalid format",
		"unsupported type",
		"serialization",
	}

	for _, pattern := range dataPatterns {
		if strings.Contains(errStr, pattern) {
			return DataError
		}
	}

	// Default to critical error for unclassified errors
	return CriticalError
}

// WrapError wraps an existing error as a MigrationError
func WrapError(err error, operation string) *MigrationError {
	if err == nil {
		return nil
	}

	// If it's already a MigrationError, return it
	var migErr *MigrationError
	if errors.As(err, &migErr) {
		return migErr
	}

	errorType := ClassifyError(err)
	return NewMigrationError(errorType, operation, err.Error()).WithCause(err)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var migErr *MigrationError
	if errors.As(err, &migErr) {
		return migErr.Retryable
	}

	// For non-MigrationError types, classify and check
	errorType := ClassifyError(err)
	return isRetryableByType(errorType)
}

// IsCritical checks if an error is critical (requires shutdown)
func IsCritical(err error) bool {
	if err == nil {
		return false
	}

	var migErr *MigrationError
	if errors.As(err, &migErr) {
		return migErr.Type == CriticalError
	}

	return ClassifyError(err) == CriticalError
}

// ErrorAggregator collects and manages multiple errors
type ErrorAggregator struct {
	errors []error
}

// NewErrorAggregator creates a new error aggregator
func NewErrorAggregator() *ErrorAggregator {
	return &ErrorAggregator{
		errors: make([]error, 0),
	}
}

// Add adds an error to the aggregator
func (ea *ErrorAggregator) Add(err error) {
	if err != nil {
		ea.errors = append(ea.errors, err)
	}
}

// HasErrors returns true if there are any errors
func (ea *ErrorAggregator) HasErrors() bool {
	return len(ea.errors) > 0
}

// Count returns the number of errors
func (ea *ErrorAggregator) Count() int {
	return len(ea.errors)
}

// Errors returns all collected errors
func (ea *ErrorAggregator) Errors() []error {
	return ea.errors
}

// Error returns a combined error message
func (ea *ErrorAggregator) Error() string {
	if len(ea.errors) == 0 {
		return ""
	}

	if len(ea.errors) == 1 {
		return ea.errors[0].Error()
	}

	var messages []string
	for i, err := range ea.errors {
		messages = append(messages, fmt.Sprintf("%d: %s", i+1, err.Error()))
	}

	return fmt.Sprintf("multiple errors occurred:\n%s", strings.Join(messages, "\n"))
}

// HasCriticalErrors checks if any collected errors are critical
func (ea *ErrorAggregator) HasCriticalErrors() bool {
	for _, err := range ea.errors {
		if IsCritical(err) {
			return true
		}
	}
	return false
}

// GetCriticalErrors returns only the critical errors
func (ea *ErrorAggregator) GetCriticalErrors() []error {
	var critical []error
	for _, err := range ea.errors {
		if IsCritical(err) {
			critical = append(critical, err)
		}
	}
	return critical
}

// Clear removes all collected errors
func (ea *ErrorAggregator) Clear() {
	ea.errors = ea.errors[:0]
}
