package service

import (
	"testing"
	"time"
)

func TestFormatAPITimestampReturnsBrowserSafeUTC(t *testing.T) {
	t.Parallel()

	localOffset := time.FixedZone("UTC+05:30", 5*60*60+30*60)
	value := time.Date(2026, time.July, 16, 18, 45, 12, 0, localOffset)

	got := formatAPITimestamp(value)
	const want = "2026-07-16T13:15:12Z"
	if got != want {
		t.Fatalf("formatAPITimestamp() = %q, want %q", got, want)
	}

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("formatted timestamp is not RFC3339: %v", err)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("formatted timestamp location = %v, want UTC", parsed.Location())
	}
}
