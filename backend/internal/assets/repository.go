package assets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	appdb "github.com/natet/honeygen/backend/internal/db"
)

var ErrNotFound = errors.New("asset not found")

type Asset struct {
	ID              string    `json:"id"`
	GenerationJobID string    `json:"generation_job_id"`
	WorldModelID    string    `json:"world_model_id"`
	SourceType      string    `json:"source_type"`
	RenderedType    string    `json:"rendered_type"`
	Path            string    `json:"path"`
	MIMEType        string    `json:"mime_type"`
	SizeBytes       int64     `json:"size_bytes"`
	Tags            []string  `json:"tags"`
	Previewable     bool      `json:"previewable"`
	Checksum        string    `json:"checksum"`
	CreatedAt       time.Time `json:"created_at"`
}

type ListOptions struct {
	WorldModelID    string
	GenerationJobID string
	Limit           int
	Offset          int
}

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Kind     string      `json:"kind"`
	AssetID  string      `json:"asset_id,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Create(ctx context.Context, asset Asset) (Asset, error) {
	tagsJSON, err := json.Marshal(asset.Tags)
	if err != nil {
		return Asset{}, fmt.Errorf("encode asset tags: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO assets (
			id, generation_job_id, world_model_id, source_type, rendered_type, path,
			mime_type, size_bytes, tags_json, previewable, checksum
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, asset.ID, asset.GenerationJobID, asset.WorldModelID, asset.SourceType, asset.RenderedType, asset.Path, asset.MIMEType, asset.SizeBytes, string(tagsJSON), boolToInt(asset.Previewable), asset.Checksum); err != nil {
		return Asset{}, fmt.Errorf("create asset %q: %w", asset.ID, err)
	}

	return r.Get(ctx, asset.ID)
}

func (r *Repository) Get(ctx context.Context, id string) (Asset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, generation_job_id, world_model_id, source_type, rendered_type, path,
		       mime_type, size_bytes, tags_json, previewable, checksum, created_at
		FROM assets
		WHERE id = ?
	`, id)

	asset, err := scanAsset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("get asset %q: %w", id, err)
	}

	return asset, nil
}

func (r *Repository) FindByPath(ctx context.Context, path string) (Asset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, generation_job_id, world_model_id, source_type, rendered_type, path,
		       mime_type, size_bytes, tags_json, previewable, checksum, created_at
		FROM assets
		WHERE path = ?
		LIMIT 1
	`, path)

	asset, err := scanAsset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("find asset by path %q: %w", path, err)
	}

	return asset, nil
}

func (r *Repository) List(ctx context.Context, options ListOptions) ([]Asset, error) {
	query := `
		SELECT id, generation_job_id, world_model_id, source_type, rendered_type, path,
		       mime_type, size_bytes, tags_json, previewable, checksum, created_at
		FROM assets
	`
	var (
		conditions []string
		args       []any
	)
	if options.WorldModelID != "" {
		conditions = append(conditions, "world_model_id = ?")
		args = append(args, options.WorldModelID)
	}
	if options.GenerationJobID != "" {
		conditions = append(conditions, "generation_job_id = ?")
		args = append(args, options.GenerationJobID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY path ASC, datetime(created_at) DESC"
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", normalizeLimit(options.Limit), normalizeOffset(options.Offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var items []Asset
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assets: %w", err)
	}

	return items, nil
}

func (r *Repository) Tree(ctx context.Context, options ListOptions) ([]*TreeNode, error) {
	items, err := r.listAllForTree(ctx, options)
	if err != nil {
		return nil, err
	}

	root := []*TreeNode{}
	for _, item := range items {
		treePath := displayPath(item, options)
		segments := strings.Split(strings.Trim(treePath, "/"), "/")
		if len(segments) == 0 {
			continue
		}

		root = insertTreeNode(root, segments, item.ID, "")
	}

	for _, node := range root {
		sortTree(node)
	}
	sort.Slice(root, func(i, j int) bool {
		return root[i].Name < root[j].Name
	})

	return root, nil
}

func (r *Repository) listAllForTree(ctx context.Context, options ListOptions) ([]Asset, error) {
	const pageSize = 1000

	var (
		allItems []Asset
		offset   int
	)
	for {
		items, err := r.List(ctx, ListOptions{
			WorldModelID:    options.WorldModelID,
			GenerationJobID: options.GenerationJobID,
			Limit:           pageSize,
			Offset:          offset,
		})
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		if len(items) < pageSize {
			return allItems, nil
		}
		offset += len(items)
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(scanner rowScanner) (Asset, error) {
	var (
		item         Asset
		tagsJSON     string
		previewable  int
		createdAtRaw string
	)

	err := scanner.Scan(
		&item.ID,
		&item.GenerationJobID,
		&item.WorldModelID,
		&item.SourceType,
		&item.RenderedType,
		&item.Path,
		&item.MIMEType,
		&item.SizeBytes,
		&tagsJSON,
		&previewable,
		&item.Checksum,
		&createdAtRaw,
	)
	if err != nil {
		return Asset{}, err
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
			return Asset{}, fmt.Errorf("decode asset tags for %q: %w", item.ID, err)
		}
	}
	item.Previewable = previewable == 1
	item.CreatedAt, err = appdb.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Asset{}, fmt.Errorf("parse asset created_at %q: %w", createdAtRaw, err)
	}

	return item, nil
}

func displayPath(item Asset, options ListOptions) string {
	trimmed := strings.Trim(item.Path, "/")
	if options.GenerationJobID != "" {
		if relative, ok := trimTreePrefix(trimmed, joinDisplayPrefix("generated", item.WorldModelID, options.GenerationJobID)); ok {
			return relative
		}
	}
	if options.WorldModelID != "" {
		if relative, ok := trimTreePrefix(trimmed, joinDisplayPrefix("generated", options.WorldModelID)); ok {
			return relative
		}
		if relative, ok := trimTreePrefix(trimmed, options.WorldModelID); ok {
			return relative
		}
	}
	return trimmed
}

func trimTreePrefix(value string, prefix string) (string, bool) {
	value = strings.Trim(value, "/")
	prefix = strings.Trim(prefix, "/")
	switch {
	case value == "", prefix == "":
		return value, false
	case value == prefix:
		return value, false
	case strings.HasPrefix(value, prefix+"/"):
		return strings.TrimPrefix(value, prefix+"/"), true
	default:
		return value, false
	}
}

func joinDisplayPrefix(parts ...string) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	if len(segments) == 0 {
		return ""
	}
	return path.Join(segments...)
}

func sortTree(node *TreeNode) {
	for _, child := range node.Children {
		sortTree(child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})
}

func insertTreeNode(nodes []*TreeNode, segments []string, assetID string, prefix string) []*TreeNode {
	name := segments[0]
	currentPath := name
	if prefix != "" {
		currentPath = prefix + "/" + name
	}

	var node *TreeNode
	for _, existing := range nodes {
		if existing.Name == name {
			node = existing
			break
		}
	}
	if node == nil {
		node = &TreeNode{Name: name, Path: currentPath, Kind: "directory"}
		nodes = append(nodes, node)
	}

	if len(segments) == 1 {
		node.Kind = "file"
		node.AssetID = assetID
		return nodes
	}

	node.Children = insertTreeNode(node.Children, segments[1:], assetID, currentPath)
	return nodes
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 1000 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
