package scanner

import (
	"fmt"
	"sync"
	"time"

	"github.com/kinyelo/redis-valkey-migration/internal/client"
	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
)

// KeyScanner defines the interface for key scanning operations
type KeyScanner interface {
	ScanAllKeys(client client.DatabaseClient) ([]string, error)
}

// KeyInfo represents information about a discovered key
type KeyInfo struct {
	Name string
	Type string
	Size int64
	TTL  time.Duration
}

// ScanProgress tracks the progress of key scanning
type ScanProgress struct {
	TotalKeys     int
	ScannedKeys   int
	StartTime     time.Time
	CurrentKey    string
	KeysPerSecond float64
	mu            sync.RWMutex
}

// Scanner handles key discovery and type detection
type Scanner struct {
	client   client.DatabaseClient
	progress *ScanProgress
	logger   logger.Logger
}

// keyScanner implements KeyScanner interface
type keyScanner struct {
	logger logger.Logger
}

// NewKeyScanner creates a new KeyScanner instance
func NewKeyScanner(logger logger.Logger) KeyScanner {
	return &keyScanner{
		logger: logger,
	}
}

// ScanAllKeys implements KeyScanner interface
func (ks *keyScanner) ScanAllKeys(client client.DatabaseClient) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("database client is nil")
	}

	keys, err := client.GetAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}

	ks.logger.Infof("Scanned %d keys from database", len(keys))
	return keys, nil
}

// NewScanner creates a new Scanner instance
func NewScanner(client client.DatabaseClient) *Scanner {
	return &Scanner{
		client: client,
		progress: &ScanProgress{
			StartTime: time.Now(),
		},
	}
}

// ScanAllKeys discovers all keys in the database and returns their metadata
func (s *Scanner) ScanAllKeys() ([]KeyInfo, error) {
	if s.client == nil {
		return nil, fmt.Errorf("database client is nil")
	}

	// Reset progress tracking
	s.progress.mu.Lock()
	s.progress.StartTime = time.Now()
	s.progress.ScannedKeys = 0
	s.progress.TotalKeys = 0
	s.progress.CurrentKey = ""
	s.progress.KeysPerSecond = 0
	s.progress.mu.Unlock()

	// Get all keys from the database
	keys, err := s.client.GetAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}

	// Update total count
	s.progress.mu.Lock()
	s.progress.TotalKeys = len(keys)
	s.progress.mu.Unlock()

	var keyInfos []KeyInfo

	// Process each key to get its metadata
	for i, key := range keys {
		// Update current progress
		s.progress.mu.Lock()
		s.progress.CurrentKey = key
		s.progress.ScannedKeys = i + 1
		elapsed := time.Since(s.progress.StartTime).Seconds()
		if elapsed > 0 {
			s.progress.KeysPerSecond = float64(s.progress.ScannedKeys) / elapsed
		}
		s.progress.mu.Unlock()

		keyInfo, err := s.getKeyInfo(key)
		if err != nil {
			// Log error but continue with other keys
			continue
		}

		keyInfos = append(keyInfos, keyInfo)
	}

	return keyInfos, nil
}

// getKeyInfo retrieves detailed information about a specific key
func (s *Scanner) getKeyInfo(key string) (KeyInfo, error) {
	keyInfo := KeyInfo{
		Name: key,
	}

	// Get key type
	keyType, err := s.client.GetKeyType(key)
	if err != nil {
		return keyInfo, fmt.Errorf("failed to get key type for %s: %w", key, err)
	}
	keyInfo.Type = keyType

	// Get TTL
	ttl, err := s.client.GetTTL(key)
	if err != nil {
		// TTL error is not critical, continue without it
		keyInfo.TTL = -1
	} else {
		keyInfo.TTL = ttl
	}

	// Estimate size based on key type (simplified approach)
	keyInfo.Size = s.estimateKeySize(key, keyType)

	return keyInfo, nil
}

// estimateKeySize provides a rough estimate of key size
// This is a simplified implementation for progress tracking
func (s *Scanner) estimateKeySize(key, keyType string) int64 {
	// Base size for the key name
	size := int64(len(key))

	// Add estimated size based on type
	// These are rough estimates for progress tracking purposes
	switch keyType {
	case "string":
		size += 50 // Average string value size
	case "hash":
		size += 200 // Average hash size
	case "list":
		size += 100 // Average list size
	case "set":
		size += 100 // Average set size
	case "zset":
		size += 150 // Average sorted set size
	default:
		size += 50 // Default estimate
	}

	return size
}

// GetProgress returns the current scanning progress
func (s *Scanner) GetProgress() ScanProgress {
	s.progress.mu.RLock()
	defer s.progress.mu.RUnlock()

	// Return a copy to avoid race conditions
	return ScanProgress{
		TotalKeys:     s.progress.TotalKeys,
		ScannedKeys:   s.progress.ScannedKeys,
		StartTime:     s.progress.StartTime,
		CurrentKey:    s.progress.CurrentKey,
		KeysPerSecond: s.progress.KeysPerSecond,
	}
}

// GetProgressPercent returns the scanning progress as a percentage
func (s *Scanner) GetProgressPercent() float64 {
	progress := s.GetProgress()
	if progress.TotalKeys == 0 {
		return 0
	}
	return float64(progress.ScannedKeys) / float64(progress.TotalKeys) * 100
}

// IsComplete returns true if scanning is complete
func (s *Scanner) IsComplete() bool {
	progress := s.GetProgress()
	return progress.ScannedKeys >= progress.TotalKeys && progress.TotalKeys > 0
}

// GetKeysByType returns keys grouped by their data type
func (s *Scanner) GetKeysByType(keyInfos []KeyInfo) map[string][]KeyInfo {
	keysByType := make(map[string][]KeyInfo)

	for _, keyInfo := range keyInfos {
		keysByType[keyInfo.Type] = append(keysByType[keyInfo.Type], keyInfo)
	}

	return keysByType
}

// GetScanSummary returns a summary of the scanning results
func (s *Scanner) GetScanSummary(keyInfos []KeyInfo) ScanSummary {
	summary := ScanSummary{
		TotalKeys:    len(keyInfos),
		TypeCounts:   make(map[string]int),
		TotalSize:    0,
		ScanDuration: time.Since(s.progress.StartTime),
	}

	for _, keyInfo := range keyInfos {
		summary.TypeCounts[keyInfo.Type]++
		summary.TotalSize += keyInfo.Size
	}

	return summary
}

// ScanSummary provides a summary of scanning results
type ScanSummary struct {
	TotalKeys    int
	TypeCounts   map[string]int
	TotalSize    int64
	ScanDuration time.Duration
}
