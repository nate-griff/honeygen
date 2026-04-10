package worldmodels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type Repository interface {
	List(context.Context) ([]StoredWorldModel, error)
	Get(context.Context, string) (StoredWorldModel, error)
	Create(context.Context, StoredWorldModel) (StoredWorldModel, error)
	Update(context.Context, StoredWorldModel) (StoredWorldModel, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: database}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]StoredWorldModel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, json_blob, created_at, updated_at
		FROM world_models
		ORDER BY datetime(updated_at) DESC, datetime(created_at) DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list world models: %w", err)
	}
	defer rows.Close()

	var items []StoredWorldModel
	for rows.Next() {
		item, err := scanStoredWorldModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate world models: %w", err)
	}

	return items, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (StoredWorldModel, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, json_blob, created_at, updated_at
		FROM world_models
		WHERE id = ?
	`, id)

	item, err := scanStoredWorldModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredWorldModel{}, ErrNotFound
	}
	if err != nil {
		return StoredWorldModel{}, fmt.Errorf("get world model %q: %w", id, err)
	}
	return item, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, item StoredWorldModel) (StoredWorldModel, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO world_models (id, name, description, json_blob)
		VALUES (?, ?, ?, ?)
	`, item.ID, item.Name, item.Description, item.JSONBlob); err != nil {
		if isDuplicateWorldModelIDError(err) {
			return StoredWorldModel{}, ErrAlreadyExists
		}
		return StoredWorldModel{}, fmt.Errorf("create world model %q: %w", item.ID, err)
	}

	return r.Get(ctx, item.ID)
}

func (r *SQLiteRepository) Update(ctx context.Context, item StoredWorldModel) (StoredWorldModel, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE world_models
		SET name = ?, description = ?, json_blob = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ?
	`, item.Name, item.Description, item.JSONBlob, item.ID)
	if err != nil {
		return StoredWorldModel{}, fmt.Errorf("update world model %q: %w", item.ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return StoredWorldModel{}, fmt.Errorf("read rows affected for world model %q: %w", item.ID, err)
	}
	if rowsAffected == 0 {
		return StoredWorldModel{}, ErrNotFound
	}

	return r.Get(ctx, item.ID)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanStoredWorldModel(scanner rowScanner) (StoredWorldModel, error) {
	var (
		item         StoredWorldModel
		createdAtRaw string
		updatedAtRaw string
	)

	err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.JSONBlob,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if err != nil {
		return StoredWorldModel{}, err
	}

	item.CreatedAt, err = time.Parse(time.RFC3339, createdAtRaw)
	if err != nil {
		return StoredWorldModel{}, fmt.Errorf("parse created_at %q: %w", createdAtRaw, err)
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtRaw)
	if err != nil {
		return StoredWorldModel{}, fmt.Errorf("parse updated_at %q: %w", updatedAtRaw, err)
	}

	return item, nil
}

func isDuplicateWorldModelIDError(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	switch sqliteErr.Code() {
	case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return strings.Contains(strings.ToLower(sqliteErr.Error()), "world_models.id")
	default:
		return false
	}
}
