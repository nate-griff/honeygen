package db

import (
	"fmt"
	"time"
)

var timestampLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
}

func ParseTimestamp(raw string) (time.Time, error) {
	var lastErr error
	for _, layout := range timestampLayouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}

	return time.Time{}, fmt.Errorf("parse timestamp %q: %w", raw, lastErr)
}
