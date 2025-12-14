package scanner

import (
	"fmt"
	"testing"
	"time"

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

// KeyTestData represents test data for a key
type KeyTestData struct {
	Key  string
	Type string
	TTL  time.Duration
}
