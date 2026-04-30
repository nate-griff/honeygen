package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type StoredFile struct {
	Path      string
	SizeBytes int64
	Checksum  string
}

type Filesystem struct {
	root string
}

func NewFilesystem(root string) *Filesystem {
	return &Filesystem{root: root}
}

func (f *Filesystem) Root() string {
	if f == nil {
		return ""
	}
	return f.root
}

func (f *Filesystem) Write(_ context.Context, relativePath string, data []byte) (StoredFile, error) {
	normalized, fullPath, err := f.resolve(relativePath)
	if err != nil {
		return StoredFile{}, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return StoredFile{}, fmt.Errorf("create directory for %q: %w", normalized, err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return StoredFile{}, fmt.Errorf("write file %q: %w", normalized, err)
	}

	sum := sha256.Sum256(data)
	return StoredFile{
		Path:      normalized,
		SizeBytes: int64(len(data)),
		Checksum:  hex.EncodeToString(sum[:]),
	}, nil
}

func (f *Filesystem) Read(_ context.Context, relativePath string) ([]byte, error) {
	_, fullPath, err := f.resolve(relativePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", relativePath, err)
	}
	return data, nil
}

func (f *Filesystem) Delete(_ context.Context, relativePath string) error {
	normalized, fullPath, err := f.resolve(relativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete file %q: %w", normalized, err)
	}
	return nil
}

func (f *Filesystem) Move(_ context.Context, fromPath, toPath string) error {
	fromNormalized, fromFullPath, err := f.resolve(fromPath)
	if err != nil {
		return err
	}
	toNormalized, toFullPath, err := f.resolve(toPath)
	if err != nil {
		return err
	}
	if fromNormalized == toNormalized {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(toFullPath), 0o755); err != nil {
		return fmt.Errorf("create directory for %q: %w", toNormalized, err)
	}
	if err := os.Rename(fromFullPath, toFullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("move file %q to %q: %w", fromNormalized, toNormalized, err)
	}
	return nil
}

// DeleteFiles deletes each of the given relative paths. A missing file is not
// an error (consistent with Delete). Any other OS-level failure is returned
// immediately without attempting the remaining paths.
func (f *Filesystem) DeleteFiles(ctx context.Context, paths []string) error {
	for _, p := range paths {
		if err := f.Delete(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// DeleteDir removes the directory rooted at relativePath and all of its
// contents. If the directory does not exist the call is a no-op. The path is
// validated against the same traversal rules as all other Filesystem methods.
func (f *Filesystem) DeleteDir(_ context.Context, relativePath string) error {
	_, fullPath, err := f.resolve(relativePath)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("delete directory %q: %w", relativePath, err)
	}
	return nil
}

func (f *Filesystem) resolve(relativePath string) (string, string, error) {
	if f == nil || f.root == "" {
		return "", "", fmt.Errorf("storage root is not configured")
	}

	normalized, err := normalizeRelativePath(relativePath)
	if err != nil {
		return "", "", err
	}

	fullPath := filepath.Join(f.root, filepath.FromSlash(normalized))
	return normalized, fullPath, nil
}

func JoinRelative(parts ...string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("storage path parts are required")
	}

	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized, err := normalizeRelativePath(part)
		if err != nil {
			return "", err
		}
		segments = append(segments, strings.Split(normalized, "/")...)
	}

	return path.Join(segments...), nil
}

func normalizeRelativePath(relativePath string) (string, error) {
	normalized := path.Clean(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"))
	switch {
	case normalized == ".", normalized == "", strings.HasPrefix(normalized, "../"), normalized == "..", path.IsAbs(normalized):
		return "", fmt.Errorf("invalid storage path %q", relativePath)
	}
	return normalized, nil
}
