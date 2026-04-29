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
