package api

import "testing"

func TestWorldModelIDFromPathRejectsSlashContainingIDs(t *testing.T) {
	_, err := worldModelIDFromPath("/api/world-models/team/alpha")
	if err == nil {
		t.Fatal("worldModelIDFromPath() error = nil, want validation error")
	}
	if got, want := err.Error(), "world model id must not contain slashes"; got != want {
		t.Fatalf("worldModelIDFromPath() error = %q, want %q", got, want)
	}
}

func TestWorldModelIDFromPathRequiresID(t *testing.T) {
	_, err := worldModelIDFromPath("/api/world-models/")
	if err == nil {
		t.Fatal("worldModelIDFromPath() error = nil, want validation error")
	}
	if got, want := err.Error(), "world model id is required"; got != want {
		t.Fatalf("worldModelIDFromPath() error = %q, want %q", got, want)
	}
}
