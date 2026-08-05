package pathutil

import (
	"os"
	"strings"
)

// NormalizePath normalizes a path by ensuring it starts with "/" and optionally ends with "/".
// If value is empty or whitespace, returns the fallback value.
// If requireTrailingSlash is true, ensures the path ends with "/" (unless it's exactly "/").
func NormalizePath(value, fallback string, requireTrailingSlash bool) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = fallback
	}
	if trimmed == "" {
		return ""
	}
	// Ensure leading slash
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	// Handle trailing slash
	if requireTrailingSlash && trimmed != "/" && !strings.HasSuffix(trimmed, "/") {
		trimmed += "/"
	}
	return trimmed
}

// JoinPaths combines a prefix and base path, handling slashes properly.
// Reads prefix from the given environment variable if set.
func JoinPaths(envVar, defaultPrefix, basePath string) string {
	prefix := defaultPrefix
	if envVar != "" {
		if configuredPrefix, ok := os.LookupEnv(envVar); ok {
			prefix = strings.TrimSpace(configuredPrefix)
		}
	}
	// Trim trailing slash from prefix
	prefix = strings.TrimSuffix(prefix, "/")
	// Ensure basePath starts with /
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return prefix + basePath
}
