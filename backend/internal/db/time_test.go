package db

import (
	"testing"
	"time"
)

func TestParseTimestampAcceptsRFC3339(t *testing.T) {
	value, err := ParseTimestamp("2026-04-10T21:23:15Z")
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}

	if want := time.Date(2026, 4, 10, 21, 23, 15, 0, time.UTC); !value.Equal(want) {
		t.Fatalf("ParseTimestamp() = %s, want %s", value, want)
	}
}

func TestParseTimestampAcceptsLegacySQLiteFormat(t *testing.T) {
	value, err := ParseTimestamp("2026-04-10 21:23:15")
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}

	if want := time.Date(2026, 4, 10, 21, 23, 15, 0, time.UTC); !value.Equal(want) {
		t.Fatalf("ParseTimestamp() = %s, want %s", value, want)
	}
}
