package mcpadapter

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// equalStringSlices compares two string slices for equality.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// equalStringMaps compares two string maps for equality.
func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// sanitizePrefix removes any characters that are not letters, numbers, or hyphens.
func sanitizePrefix(prefix string) string {
	// Replace underscores with hyphens first.
	sanitized := strings.ReplaceAll(prefix, "_", "-")

	// Now, remove any remaining characters that are not letters, numbers, or hyphens.
	reg1 := regexp.MustCompile("[^a-zA-Z0-9-]+")
	sanitized = reg1.ReplaceAllString(sanitized, "")

	// Replace multiple consecutive hyphens with a single hyphen
	reg2 := regexp.MustCompile("-+")
	sanitized = reg2.ReplaceAllString(sanitized, "-")

	// Trim leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

// Helper functions

func writeConfig(path string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
