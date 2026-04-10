package worldmodels

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	appdb "github.com/natet/honeygen/backend/internal/db"
)

func TestSQLiteRepositoryCreateListGetAndUpdate(t *testing.T) {
	database := newTestDatabase(t)
	repository := NewRepository(database)

	created, err := repository.Create(context.Background(), StoredWorldModel{
		ID:          "world-1",
		Name:        "Acme Advisory",
		Description: "Initial description",
		JSONBlob:    `{"organization":{"name":"Acme Advisory","industry":"Financial Services","size":"mid-size","region":"United States","domain_theme":"acmeadvisory.local"},"branding":{"tone":"professional"},"departments":[],"employees":[],"projects":[],"document_themes":[]}`,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("created.CreatedAt is zero")
	}
	if created.UpdatedAt.IsZero() {
		t.Fatal("created.UpdatedAt is zero")
	}

	got, err := repository.Get(context.Background(), "world-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "Acme Advisory" || got.Description != "Initial description" {
		t.Fatalf("Get() = %+v, want name and description to match", got)
	}

	items, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(List()) = %d, want %d", len(items), 1)
	}
	if items[0].ID != "world-1" {
		t.Fatalf("List()[0].ID = %q, want %q", items[0].ID, "world-1")
	}

	updated, err := repository.Update(context.Background(), StoredWorldModel{
		ID:          "world-1",
		Name:        "Acme Advisory Updated",
		Description: "Updated description",
		JSONBlob:    `{"organization":{"name":"Acme Advisory Updated","industry":"Financial Services","size":"mid-size","region":"Canada","domain_theme":"acmeadvisory.ca"},"branding":{"tone":"professional"},"departments":["Finance"],"employees":[],"projects":[],"document_themes":[]}`,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Acme Advisory Updated" {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, "Acme Advisory Updated")
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("updated.UpdatedAt = %s, want same or after %s", updated.UpdatedAt, created.UpdatedAt)
	}
}

func TestSQLiteRepositoryGetReturnsNotFound(t *testing.T) {
	database := newTestDatabase(t)
	repository := NewRepository(database)

	_, err := repository.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}
}

func newTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := appdb.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return database
}
