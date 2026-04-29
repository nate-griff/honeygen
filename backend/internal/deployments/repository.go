package deployments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	appdb "github.com/natet/honeygen/backend/internal/db"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrNotFound = errors.New("deployment not found")
var ErrPortConflict = errors.New("deployment port conflict")

const nfsMountPath = "/mount"

type Deployment struct {
	ID              string    `json:"id"`
	GenerationJobID string    `json:"generation_job_id"`
	WorldModelID    string    `json:"world_model_id"`
	Protocol        string    `json:"protocol"`
	Port            int       `json:"port"`
	RootPath        string    `json:"root_path"`
	Status          string    `json:"status"`
	ShareName       string    `json:"share_name,omitempty"`
	MountPath       string    `json:"mount_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Create(ctx context.Context, d Deployment) (Deployment, error) {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.Protocol == "" {
		d.Protocol = "http"
	}
	if d.RootPath == "" {
		d.RootPath = "/"
	}
	if d.Status == "" {
		d.Status = "stopped"
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO deployments (id, generation_job_id, world_model_id, protocol, port, root_path, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.GenerationJobID, d.WorldModelID, d.Protocol, d.Port, d.RootPath, d.Status); err != nil {
		if isDuplicateDeploymentPortError(err) {
			return Deployment{}, fmt.Errorf("create deployment %q: %w: %v", d.ID, ErrPortConflict, err)
		}
		return Deployment{}, fmt.Errorf("create deployment %q: %w", d.ID, err)
	}

	return r.Get(ctx, d.ID)
}

func (r *Repository) Get(ctx context.Context, id string) (Deployment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, generation_job_id, world_model_id, protocol, port, root_path, status, created_at, updated_at
		FROM deployments
		WHERE id = ?
	`, id)

	d, err := scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	if err != nil {
		return Deployment{}, fmt.Errorf("get deployment %q: %w", id, err)
	}
	return d, nil
}

func (r *Repository) List(ctx context.Context) ([]Deployment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, generation_job_id, world_model_id, protocol, port, root_path, status, created_at, updated_at
		FROM deployments
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()

	items := []Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deployments: %w", err)
	}
	return items, nil
}

func (r *Repository) ListByStatus(ctx context.Context, status string) ([]Deployment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, generation_job_id, world_model_id, protocol, port, root_path, status, created_at, updated_at
		FROM deployments
		WHERE status = ?
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("list deployments by status %q: %w", status, err)
	}
	defer rows.Close()

	items := []Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deployments: %w", err)
	}
	return items, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE deployments SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ?
	`, status, id)
	if err != nil {
		return fmt.Errorf("update deployment status %q: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM deployments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete deployment %q: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ExistsPort(ctx context.Context, port int) (bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deployments WHERE port = ?)`, port)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("check deployment port %d: %w", port, err)
	}

	return exists, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDeployment(scanner rowScanner) (Deployment, error) {
	var (
		d            Deployment
		createdAtRaw string
		updatedAtRaw string
	)

	err := scanner.Scan(
		&d.ID,
		&d.GenerationJobID,
		&d.WorldModelID,
		&d.Protocol,
		&d.Port,
		&d.RootPath,
		&d.Status,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if err != nil {
		return Deployment{}, err
	}

	d.CreatedAt, err = appdb.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Deployment{}, fmt.Errorf("parse deployment created_at %q: %w", createdAtRaw, err)
	}
	d.UpdatedAt, err = appdb.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return Deployment{}, fmt.Errorf("parse deployment updated_at %q: %w", updatedAtRaw, err)
	}

	switch d.Protocol {
	case "smb":
		d.ShareName = smbShareName
	case "nfs":
		d.MountPath = nfsMountPath
	}

	return d, nil
}

func isDuplicateDeploymentPortError(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	switch sqliteErr.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		message := strings.ToLower(sqliteErr.Error())
		return strings.Contains(message, "deployments.port") || strings.Contains(message, "idx_deployments_port_unique")
	default:
		return false
	}
}
