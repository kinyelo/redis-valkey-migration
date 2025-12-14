package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RotatingFileWriter handles log file rotation based on size and age
type RotatingFileWriter struct {
	filename string
	maxSize  int64
	maxAge   int
	file     *os.File
}

// NewRotatingFileWriter creates a new rotating file writer
func NewRotatingFileWriter(filename string, maxSize int64, maxAge int) (*RotatingFileWriter, error) {
	if err := ensureLogDir(filename); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	writer := &RotatingFileWriter{
		filename: filename,
		maxSize:  maxSize,
		maxAge:   maxAge,
	}

	if err := writer.openFile(); err != nil {
		return nil, err
	}

	// Clean up old log files
	if err := writer.cleanupOldFiles(); err != nil {
		// Log the error but don't fail initialization
		fmt.Printf("Warning: failed to cleanup old log files: %v\n", err)
	}

	return writer, nil
}

// Write implements io.Writer interface
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	// Check if rotation is needed
	if w.needsRotation() {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	return w.file.Write(p)
}

// Close closes the current log file
func (w *RotatingFileWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// needsRotation checks if the log file needs to be rotated
func (w *RotatingFileWriter) needsRotation() bool {
	if w.file == nil {
		return false
	}

	stat, err := w.file.Stat()
	if err != nil {
		return false
	}

	return stat.Size() >= w.maxSize
}

// rotate rotates the current log file
func (w *RotatingFileWriter) rotate() error {
	// Close current file
	if w.file != nil {
		w.file.Close()
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(w.filename)
	base := strings.TrimSuffix(w.filename, ext)
	rotatedName := fmt.Sprintf("%s.%s%s", base, timestamp, ext)

	// Rename current file to rotated name
	if err := os.Rename(w.filename, rotatedName); err != nil {
		// If rename fails, try to continue with a new file
		fmt.Printf("Warning: failed to rename log file: %v\n", err)
	}

	// Open new file
	return w.openFile()
}

// openFile opens the log file for writing
func (w *RotatingFileWriter) openFile() error {
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	w.file = file
	return nil
}

// cleanupOldFiles removes log files older than maxAge days
func (w *RotatingFileWriter) cleanupOldFiles() error {
	if w.maxAge <= 0 {
		return nil // No cleanup if maxAge is not set
	}

	dir := filepath.Dir(w.filename)
	base := filepath.Base(w.filename)
	ext := filepath.Ext(base)
	namePrefix := strings.TrimSuffix(base, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -w.maxAge)
	var filesToDelete []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Check if this is a rotated log file
		if strings.HasPrefix(name, namePrefix+".") && strings.HasSuffix(name, ext) {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				filesToDelete = append(filesToDelete, filepath.Join(dir, name))
			}
		}
	}

	// Sort files to delete oldest first
	sort.Strings(filesToDelete)

	for _, file := range filesToDelete {
		if err := os.Remove(file); err != nil {
			fmt.Printf("Warning: failed to remove old log file %s: %v\n", file, err)
		}
	}

	return nil
}
