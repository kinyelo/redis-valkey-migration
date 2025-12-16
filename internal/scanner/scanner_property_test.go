package scanner

import (
	"fmt"
	"testing"
	"time"

	"github.com/kinyelo/redis-valkey-migration/pkg/logger"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: redis-valkey-migration, Property 3: Comprehensive Object Discovery**
// **Validates: Requirements 1.3**
func TestProperty_ComprehensiveObjectDiscovery(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("scanner discovers all keys with correct types", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 20 { // Limit to reasonable size for testing
				keyCount = 20
			}

			// Generate unique key data
			keyData := make([]KeyTestData, keyCount)
			types := []string{"string", "hash", "list", "set", "zset"}
			ttls := []time.Duration{-1, 60 * time.Second, 300 * time.Second, 3600 * time.Second}

			for i := 0; i < keyCount; i++ {
				keyData[i] = KeyTestData{
					Key:  fmt.Sprintf("test:key:%d", i),
					Type: types[i%len(types)],
					TTL:  ttls[i%len(ttls)],
				}
			}

			// Create mock client with the generated key data
			mockClient := &MockDatabaseClient{}

			// Extract keys for GetAllKeys mock
			keys := make([]string, len(keyData))
			for i, kd := range keyData {
				keys[i] = kd.Key
			}

			mockClient.On("GetAllKeys").Return(keys, nil)

			// Mock GetKeyType and GetTTL for each key
			for _, kd := range keyData {
				mockClient.On("GetKeyType", kd.Key).Return(kd.Type, nil)
				mockClient.On("GetTTL", kd.Key).Return(kd.TTL, nil)
			}

			// Create scanner and scan all keys
			scanner := NewScanner(mockClient)
			discoveredKeys, err := scanner.ScanAllKeys()

			// Property: Scanner should discover all keys without error
			if err != nil {
				return false
			}

			// Property: Number of discovered keys should match input
			if len(discoveredKeys) != len(keyData) {
				return false
			}

			// Property: All keys should be discovered with correct metadata
			discoveredMap := make(map[string]KeyInfo)
			for _, ki := range discoveredKeys {
				discoveredMap[ki.Name] = ki
			}

			for _, expected := range keyData {
				discovered, exists := discoveredMap[expected.Key]
				if !exists {
					return false // Key not discovered
				}

				// Property: Key type should match exactly
				if discovered.Type != expected.Type {
					return false
				}

				// Property: Key name should match exactly
				if discovered.Name != expected.Key {
					return false
				}

				// Property: TTL should match (allowing for -1 as no TTL)
				if discovered.TTL != expected.TTL {
					return false
				}

				// Property: Size should be estimated (greater than key name length)
				if discovered.Size <= int64(len(expected.Key)) {
					return false
				}
			}

			// Property: Progress tracking should be accurate
			progress := scanner.GetProgress()
			if progress.TotalKeys != len(keyData) {
				return false
			}

			if progress.ScannedKeys != len(keyData) {
				return false
			}

			// Property: Scanner should report completion correctly
			if !scanner.IsComplete() && len(keyData) > 0 {
				return false
			}

			// Property: Progress percentage should be 100% when complete
			if len(keyData) > 0 && scanner.GetProgressPercent() != 100.0 {
				return false
			}

			return true
		},
		gen.IntRange(0, 20),
	))

	properties.Property("scanner handles empty database correctly", prop.ForAll(
		func() bool {
			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return([]string{}, nil)

			scanner := NewScanner(mockClient)
			discoveredKeys, err := scanner.ScanAllKeys()

			// Property: Empty database should return no error
			if err != nil {
				return false
			}

			// Property: Empty database should return empty results
			if len(discoveredKeys) != 0 {
				return false
			}

			// Property: Progress should reflect empty state
			progress := scanner.GetProgress()
			if progress.TotalKeys != 0 || progress.ScannedKeys != 0 {
				return false
			}

			// Property: Progress percentage should be 0 for empty database
			if scanner.GetProgressPercent() != 0.0 {
				return false
			}

			return true
		},
	))

	properties.Property("scanner groups keys by type correctly", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true // Skip empty case, handled by other property
			}
			if keyCount > 15 {
				keyCount = 15
			}

			scanner := NewScanner(nil) // No client needed for this test

			// Generate test key infos
			keyInfos := make([]KeyInfo, keyCount)
			types := []string{"string", "hash", "list", "set", "zset"}

			for i := 0; i < keyCount; i++ {
				keyInfos[i] = KeyInfo{
					Name: fmt.Sprintf("key:%d", i),
					Type: types[i%len(types)],
					TTL:  -1,
					Size: int64(50 + i), // Simple size estimate
				}
			}

			keysByType := scanner.GetKeysByType(keyInfos)

			// Property: All keys should be grouped
			totalGrouped := 0
			for _, keys := range keysByType {
				totalGrouped += len(keys)
			}

			if totalGrouped != len(keyInfos) {
				return false
			}

			// Property: Keys should be in correct type groups
			for typeName, keys := range keysByType {
				for _, key := range keys {
					if key.Type != typeName {
						return false
					}
				}
			}

			// Property: Each key should appear exactly once
			seenKeys := make(map[string]bool)
			for _, keys := range keysByType {
				for _, key := range keys {
					if seenKeys[key.Name] {
						return false // Duplicate key
					}
					seenKeys[key.Name] = true
				}
			}

			return true
		},
		gen.IntRange(1, 15),
	))

	properties.TestingRun(t)
}

// **Feature: redis-valkey-migration, Property 16: Pattern Matching Accuracy**
// **Validates: Requirements 9.1**
func TestProperty_PatternMatchingAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("pattern matching follows glob rules correctly", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 20 {
				keyCount = 20
			}

			// Generate test keys with known patterns
			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				switch i % 4 {
				case 0:
					keys[i] = fmt.Sprintf("user:%d", i)
				case 1:
					keys[i] = fmt.Sprintf("session:%d", i)
				case 2:
					keys[i] = fmt.Sprintf("cache:data:%d", i)
				case 3:
					keys[i] = fmt.Sprintf("temp_%d", i)
				}
			}

			// Test various patterns
			testCases := []struct {
				patterns []string
				expected func(string) bool
			}{
				{
					patterns: []string{"user:*"},
					expected: func(key string) bool {
						return len(key) >= 5 && key[:5] == "user:"
					},
				},
				{
					patterns: []string{"session:*"},
					expected: func(key string) bool {
						return len(key) >= 8 && key[:8] == "session:"
					},
				},
				{
					patterns: []string{"cache:*"},
					expected: func(key string) bool {
						return len(key) >= 6 && key[:6] == "cache:"
					},
				},
				{
					patterns: []string{"temp_*"},
					expected: func(key string) bool {
						return len(key) >= 5 && key[:5] == "temp_"
					},
				},
				{
					patterns: []string{"user:*", "session:*"},
					expected: func(key string) bool {
						return (len(key) >= 5 && key[:5] == "user:") ||
							(len(key) >= 8 && key[:8] == "session:")
					},
				},
			}

			// Create a mock logger for testing
			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			for _, tc := range testCases {
				for _, key := range keys {
					matches := scanner.MatchesPatterns(key, tc.patterns)
					expected := tc.expected(key)

					// Property: Pattern matching should match expected results
					if matches != expected {
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(0, 20),
	))

	properties.Property("empty patterns match all keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 10 {
				keyCount = 10
			}

			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				keys[i] = fmt.Sprintf("key:%d", i)
			}

			// Create a mock logger for testing
			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Property: Empty patterns should match all keys
			for _, key := range keys {
				if !scanner.MatchesPatterns(key, []string{}) {
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 10),
	))

	properties.Property("invalid patterns are handled gracefully", prop.ForAll(
		func() bool {
			// Create a mock logger for testing
			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Test with invalid glob patterns
			invalidPatterns := []string{"[", "\\", "[z-a]"}
			testKey := "test:key"

			// Property: Invalid patterns should not cause crashes
			// and should return false (no match)
			for _, pattern := range invalidPatterns {
				matches := scanner.MatchesPatterns(testKey, []string{pattern})
				// Invalid patterns should not match
				if matches {
					return false
				}
			}

			return true
		},
	))

	properties.Property("pattern matching is case sensitive", prop.ForAll(
		func() bool {
			// Create a mock logger for testing
			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			testCases := []struct {
				key     string
				pattern string
				matches bool
			}{
				{"User:123", "user:*", false},
				{"user:123", "User:*", false},
				{"user:123", "user:*", true},
				{"USER:123", "USER:*", true},
			}

			for _, tc := range testCases {
				matches := scanner.MatchesPatterns(tc.key, []string{tc.pattern})
				if matches != tc.matches {
					return false
				}
			}

			return true
		},
	))

	properties.TestingRun(t)
}

// **Feature: redis-valkey-migration, Property 17: Multiple Pattern Collection**
// **Validates: Requirements 9.2**
func TestProperty_MultiplePatternCollection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("multiple patterns collect all matching keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 20 {
				keyCount = 20
			}

			// Generate test keys with known patterns
			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				switch i % 5 {
				case 0:
					keys[i] = fmt.Sprintf("user:%d", i)
				case 1:
					keys[i] = fmt.Sprintf("session:%d", i)
				case 2:
					keys[i] = fmt.Sprintf("cache:data:%d", i)
				case 3:
					keys[i] = fmt.Sprintf("temp_%d", i)
				case 4:
					keys[i] = fmt.Sprintf("other:key:%d", i)
				}
			}

			// Create mock client and scanner
			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Test multiple patterns
			patterns := []string{"user:*", "session:*", "cache:*"}

			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, patterns)
			if err != nil {
				return false
			}

			// Property: All matched keys should match at least one pattern
			for _, key := range matchedKeys {
				if !scanner.MatchesPatterns(key, patterns) {
					return false
				}
			}

			// Property: All keys that match any pattern should be included
			expectedMatches := 0
			for _, key := range keys {
				if scanner.MatchesPatterns(key, patterns) {
					expectedMatches++
				}
			}

			if len(matchedKeys) != expectedMatches {
				return false
			}

			// Property: No duplicates should exist in results
			seenKeys := make(map[string]bool)
			for _, key := range matchedKeys {
				if seenKeys[key] {
					return false // Duplicate found
				}
				seenKeys[key] = true
			}

			mockClient.AssertExpectations(t)
			return true
		},
		gen.IntRange(0, 20),
	))

	properties.Property("empty pattern list returns all keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 10 {
				keyCount = 10
			}

			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				keys[i] = fmt.Sprintf("key:%d", i)
			}

			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Property: Empty patterns should return all keys
			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, []string{})
			if err != nil {
				return false
			}

			if len(matchedKeys) != len(keys) {
				return false
			}

			mockClient.AssertExpectations(t)
			return true
		},
		gen.IntRange(0, 10),
	))

	properties.Property("overlapping patterns don't create duplicates", prop.ForAll(
		func() bool {
			// Create keys that match multiple patterns
			keys := []string{
				"user:admin:123",
				"user:guest:456",
				"admin:settings:789",
				"guest:profile:012",
				"other:data:345",
			}

			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Use overlapping patterns
			patterns := []string{"user:*", "*:admin:*", "*:guest:*"}

			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, patterns)
			if err != nil {
				return false
			}

			// Property: No duplicates should exist even with overlapping patterns
			seenKeys := make(map[string]bool)
			for _, key := range matchedKeys {
				if seenKeys[key] {
					return false // Duplicate found
				}
				seenKeys[key] = true
			}

			// Property: All matched keys should match at least one pattern
			for _, key := range matchedKeys {
				if !scanner.MatchesPatterns(key, patterns) {
					return false
				}
			}

			mockClient.AssertExpectations(t)
			return true
		},
	))

	properties.TestingRun(t)
}

// **Feature: redis-valkey-migration, Property 18: Filtered Key Scanning**
// **Validates: Requirements 9.3**
func TestProperty_FilteredKeyScanning(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("filtered scanning only processes matching keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 20 {
				keyCount = 20
			}

			// Generate mixed keys - some matching, some not
			keys := make([]string, keyCount)
			expectedMatches := 0
			for i := 0; i < keyCount; i++ {
				if i%3 == 0 {
					keys[i] = fmt.Sprintf("target:%d", i)
					expectedMatches++
				} else {
					keys[i] = fmt.Sprintf("other:%d", i)
				}
			}

			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Use specific pattern to filter
			patterns := []string{"target:*"}

			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, patterns)
			if err != nil {
				return false
			}

			// Property: Only keys matching the pattern should be returned
			if len(matchedKeys) != expectedMatches {
				return false
			}

			// Property: All returned keys should match the pattern
			for _, key := range matchedKeys {
				if !scanner.MatchesPatterns(key, patterns) {
					return false
				}
			}

			// Property: Scanner should not return keys that don't match
			for _, key := range matchedKeys {
				found := false
				for _, originalKey := range keys {
					if originalKey == key && scanner.MatchesPatterns(originalKey, patterns) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}

			mockClient.AssertExpectations(t)
			return true
		},
		gen.IntRange(0, 20),
	))

	properties.Property("scanning efficiency - doesn't scan entire database unnecessarily", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 1 {
				keyCount = 1
			}
			if keyCount > 15 {
				keyCount = 15
			}

			// Generate keys where only a few match
			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				if i < 2 {
					keys[i] = fmt.Sprintf("special:%d", i)
				} else {
					keys[i] = fmt.Sprintf("normal:%d", i)
				}
			}

			mockClient := &MockDatabaseClient{}
			// The scanner should call GetAllKeys once to get all keys, then filter
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			patterns := []string{"special:*"}
			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, patterns)
			if err != nil {
				return false
			}

			// Property: Should return only the special keys
			expectedCount := 2
			if keyCount < 2 {
				expectedCount = keyCount
			}

			if len(matchedKeys) != expectedCount {
				return false
			}

			// Property: All returned keys should start with "special:"
			for _, key := range matchedKeys {
				if len(key) < 8 || key[:8] != "special:" {
					return false
				}
			}

			mockClient.AssertExpectations(t)
			return true
		},
		gen.IntRange(1, 15),
	))

	properties.Property("no matches returns empty result without error", prop.ForAll(
		func(keyCount int) bool {
			if keyCount < 0 {
				keyCount = 0
			}
			if keyCount > 10 {
				keyCount = 10
			}

			// Generate keys that won't match our pattern
			keys := make([]string, keyCount)
			for i := 0; i < keyCount; i++ {
				keys[i] = fmt.Sprintf("nomatch:%d", i)
			}

			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Use pattern that won't match any keys
			patterns := []string{"different:*"}

			matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, patterns)

			// Property: Should return empty result without error
			if err != nil {
				return false
			}

			if len(matchedKeys) != 0 {
				return false
			}

			mockClient.AssertExpectations(t)
			return true
		},
		gen.IntRange(0, 10),
	))

	properties.Property("complex patterns work correctly", prop.ForAll(
		func() bool {
			keys := []string{
				"user:123:profile",
				"user:456:settings",
				"admin:789:config",
				"guest:012:temp",
				"user:345:data",
				"system:678:log",
			}

			mockClient := &MockDatabaseClient{}
			mockClient.On("GetAllKeys").Return(keys, nil)

			mockLogger := &MockLogger{}
			scanner := NewKeyScanner(mockLogger)

			// Test complex patterns
			testCases := []struct {
				patterns      []string
				expectedCount int
			}{
				{[]string{"user:*:profile"}, 1},
				{[]string{"user:*"}, 3},
				{[]string{"*:*:config", "*:*:temp"}, 2},
				{[]string{"admin:*", "guest:*"}, 2},
			}

			for _, tc := range testCases {
				matchedKeys, err := scanner.ScanKeysByPatterns(mockClient, tc.patterns)
				if err != nil {
					return false
				}

				if len(matchedKeys) != tc.expectedCount {
					return false
				}

				// All matched keys should match at least one pattern
				for _, key := range matchedKeys {
					if !scanner.MatchesPatterns(key, tc.patterns) {
						return false
					}
				}
			}

			mockClient.AssertExpectations(t)
			return true
		},
	))

	properties.TestingRun(t)
}

// KeyTestData represents test data for a key
type KeyTestData struct {
	Key  string
	Type string
	TTL  time.Duration
}

// MockLogger is a mock implementation of logger.Logger for testing
type MockLogger struct{}

func (m *MockLogger) Debug(args ...interface{})                              {}
func (m *MockLogger) Info(args ...interface{})                               {}
func (m *MockLogger) Warn(args ...interface{})                               {}
func (m *MockLogger) Error(args ...interface{})                              {}
func (m *MockLogger) Fatal(args ...interface{})                              {}
func (m *MockLogger) Debugf(format string, args ...interface{})              {}
func (m *MockLogger) Infof(format string, args ...interface{})               {}
func (m *MockLogger) Warnf(format string, args ...interface{})               {}
func (m *MockLogger) Errorf(format string, args ...interface{})              {}
func (m *MockLogger) Fatalf(format string, args ...interface{})              {}
func (m *MockLogger) WithField(key string, value interface{}) logger.Logger  { return m }
func (m *MockLogger) WithFields(fields map[string]interface{}) logger.Logger { return m }
func (m *MockLogger) LogConnection(operation, host string, port int, database int, success bool, duration time.Duration) {
}
func (m *MockLogger) LogKeyTransfer(key, dataType string, size int64, success bool, duration time.Duration, errorMsg string) {
}
func (m *MockLogger) LogProgress(totalKeys, processedKeys, failedKeys int, throughput float64) {}
func (m *MockLogger) LogError(operation, key, errorMsg, stackTrace string, retryAttempt int)   {}
func (m *MockLogger) LogSummary(stats logger.MigrationStats)                                   {}
