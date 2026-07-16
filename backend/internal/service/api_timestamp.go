package service

import "time"

// formatAPITimestamp keeps hand-written API mappings browser-safe and
// independent of the server's local timezone.
func formatAPITimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
