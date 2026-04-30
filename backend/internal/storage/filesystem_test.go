package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemDeleteFilesRemovesAllPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	paths := []string{
		"generated/world-1/job-1/public/index.html",
		"generated/world-1/job-1/public/about.html",
		"generated/world-1/job-1/shared/report.csv",
	}
	for _, p := range paths {
		if _, err := fs.Write(ctx, p, []byte("content")); err != nil {
			t.Fatalf("Write() %q error = %v", p, err)
		}
	}

	if err := fs.DeleteFiles(ctx, paths); err != nil {
		t.Fatalf("DeleteFiles() error = %v", err)
	}

	for _, p := range paths {
		fullPath := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("file %q still exists after DeleteFiles(), stat error = %v", p, err)
		}
	}
}

func TestFilesystemDeleteFilesReturnsErrorOnPermissionFailure(t *testing.T) {
	t.Parallel()

	// Verify that a non-existent-file-not-found case is OK (consistent with Delete).
	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	// Deleting a path that was never written should succeed (not-found is not an error).
	if err := fs.DeleteFiles(ctx, []string{"generated/world-1/job-1/public/missing.html"}); err != nil {
		t.Fatalf("DeleteFiles() with missing file error = %v, want nil", err)
	}
}

func TestFilesystemDeleteFilesRejectsTraversalPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	if err := fs.DeleteFiles(ctx, []string{"../escape"}); err == nil {
		t.Fatal("DeleteFiles() with traversal path error = nil, want error")
	}
}

func TestFilesystemDeleteFilesIsEmptyNoOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	if err := fs.DeleteFiles(ctx, nil); err != nil {
		t.Fatalf("DeleteFiles(nil) error = %v, want nil", err)
	}
	if err := fs.DeleteFiles(ctx, []string{}); err != nil {
		t.Fatalf("DeleteFiles([]) error = %v, want nil", err)
	}
}

func TestFilesystemMoveRelocatesFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	const fromPath = "generated/world-1/job-1/public/file.txt"
	const toPath = ".deletions/assets/asset-1"

	if _, err := fs.Write(ctx, fromPath, []byte("content")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := fs.Move(ctx, fromPath, toPath); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(fromPath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source file still exists after Move(), stat error = %v", err)
	}

	data, err := fs.Read(ctx, toPath)
	if err != nil {
		t.Fatalf("Read(destination) error = %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("destination content = %q, want %q", string(data), "content")
	}
}

func TestFilesystemMoveIsNoOpWhenSourceMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)

	if err := fs.Move(context.Background(), "generated/world-1/job-1/public/missing.txt", ".deletions/assets/asset-1"); err != nil {
		t.Fatalf("Move() missing source error = %v, want nil", err)
	}
}

func TestFilesystemDeleteDirRemovesSubtree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	paths := []string{
		"generated/world-1/job-1/public/index.html",
		"generated/world-1/job-1/public/about.html",
		"generated/world-1/job-1/intranet/doc.md",
	}
	for _, p := range paths {
		if _, err := fs.Write(ctx, p, []byte("content")); err != nil {
			t.Fatalf("Write() %q error = %v", p, err)
		}
	}

	if err := fs.DeleteDir(ctx, "generated/world-1/job-1"); err != nil {
		t.Fatalf("DeleteDir() error = %v", err)
	}

	// All files inside the job directory must be gone.
	for _, p := range paths {
		fullPath := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("file %q still exists after DeleteDir(), stat error = %v", p, err)
		}
	}

	// The job directory itself must be gone.
	jobDir := filepath.Join(root, "generated", "world-1", "job-1")
	if _, err := os.Stat(jobDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("job dir %q still exists after DeleteDir()", jobDir)
	}

	// Sibling directories must be untouched (parent directory still exists).
	worldDir := filepath.Join(root, "generated", "world-1")
	if _, err := os.Stat(worldDir); err != nil {
		t.Fatalf("world dir %q unexpectedly missing after DeleteDir(): %v", worldDir, err)
	}
}

func TestFilesystemDeleteDirIsNoOpWhenDirectoryAbsent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	// Deleting a directory that doesn't exist must not error.
	if err := fs.DeleteDir(ctx, "generated/world-1/job-missing"); err != nil {
		t.Fatalf("DeleteDir() nonexistent dir error = %v, want nil", err)
	}
}

func TestFilesystemDeleteDirRejectsTraversalPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := NewFilesystem(root)
	ctx := context.Background()

	if err := fs.DeleteDir(ctx, "../escape"); err == nil {
		t.Fatal("DeleteDir() with traversal path error = nil, want error")
	}
}
