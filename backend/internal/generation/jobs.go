package generation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appdb "github.com/natet/honeygen/backend/internal/db"
)

var ErrJobNotFound = errors.New("generation job not found")

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type LogEntry struct {
	Time     time.Time `json:"time"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
	Path     string    `json:"path,omitempty"`
	Category string    `json:"category,omitempty"`
}

type Summary struct {
	ManifestCount int        `json:"manifest_count"`
	AssetCount    int        `json:"asset_count"`
	Categories    []string   `json:"categories,omitempty"`
	Logs          []LogEntry `json:"logs,omitempty"`
}

type Job struct {
	ID           string     `json:"id"`
	WorldModelID string     `json:"world_model_id"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Summary      Summary    `json:"summary"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ListOptions struct {
	WorldModelID string
	Limit        int
	Offset       int
}

type JobStore struct {
	db          *sql.DB
	idGenerator func() string
}

func NewJobStore(database *sql.DB) *JobStore {
	return &JobStore{
		db:          database,
		idGenerator: newJobID,
	}
}

func (s *JobStore) Create(ctx context.Context, worldModelID string) (Job, error) {
	id := s.idGenerator()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO generation_jobs (id, world_model_id, status, summary_json)
		VALUES (?, ?, ?, '{}')
	`, id, worldModelID, StatusPending); err != nil {
		return Job{}, fmt.Errorf("create generation job: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *JobStore) Get(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, world_model_id, status, started_at, summary_json, error_message, created_at, updated_at, completed_at
		FROM generation_jobs
		WHERE id = ?
	`, id)

	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get generation job %q: %w", id, err)
	}
	return job, nil
}

func (s *JobStore) List(ctx context.Context, options ListOptions) ([]Job, error) {
	query := `
		SELECT id, world_model_id, status, started_at, summary_json, error_message, created_at, updated_at, completed_at
		FROM generation_jobs
	`
	var (
		conditions []string
		args       []any
	)
	if options.WorldModelID != "" {
		conditions = append(conditions, "world_model_id = ?")
		args = append(args, options.WorldModelID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY datetime(created_at) DESC LIMIT %d OFFSET %d", normalizeJobLimit(options.Limit), normalizeJobOffset(options.Offset))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list generation jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate generation jobs: %w", err)
	}
	return jobs, nil
}

func (s *JobStore) SetRunning(ctx context.Context, id string, summary Summary) (Job, error) {
	return s.update(ctx, id, StatusRunning, summary, "", true, false)
}

func (s *JobStore) UpdateSummary(ctx context.Context, id string, summary Summary) (Job, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	return s.update(ctx, id, current.Status, summary, current.ErrorMessage, current.StartedAt == nil, false)
}

func (s *JobStore) SetCompleted(ctx context.Context, id string, summary Summary) (Job, error) {
	return s.update(ctx, id, StatusCompleted, summary, "", false, true)
}

func (s *JobStore) SetFailed(ctx context.Context, id string, summary Summary, message string) (Job, error) {
	return s.update(ctx, id, StatusFailed, summary, message, false, true)
}

func (s *JobStore) update(ctx context.Context, id, status string, summary Summary, errorMessage string, ensureStarted bool, complete bool) (Job, error) {
	summary.Logs = append([]LogEntry(nil), summary.Logs...)
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return Job{}, fmt.Errorf("encode generation job summary: %w", err)
	}

	startedAtClause := ""
	if ensureStarted {
		startedAtClause = ", started_at = COALESCE(started_at, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))"
	}
	completedAtClause := ""
	if complete {
		completedAtClause = ", completed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')"
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE generation_jobs
		SET status = ?, summary_json = ?, error_message = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`+startedAtClause+completedAtClause+`
		WHERE id = ?
	`, status, string(summaryJSON), errorMessage, id)
	if err != nil {
		return Job{}, fmt.Errorf("update generation job %q: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Job{}, fmt.Errorf("inspect generation job update %q: %w", id, err)
	}
	if rowsAffected == 0 {
		return Job{}, ErrJobNotFound
	}

	return s.Get(ctx, id)
}

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (Job, error) {
	var (
		job          Job
		startedAtRaw sql.NullString
		summaryJSON  string
		createdAtRaw string
		updatedAtRaw string
		completedRaw sql.NullString
	)

	err := scanner.Scan(
		&job.ID,
		&job.WorldModelID,
		&job.Status,
		&startedAtRaw,
		&summaryJSON,
		&job.ErrorMessage,
		&createdAtRaw,
		&updatedAtRaw,
		&completedRaw,
	)
	if err != nil {
		return Job{}, err
	}

	if startedAtRaw.Valid && startedAtRaw.String != "" {
		parsed, err := appdb.ParseTimestamp(startedAtRaw.String)
		if err != nil {
			return Job{}, fmt.Errorf("parse generation job started_at %q: %w", startedAtRaw.String, err)
		}
		job.StartedAt = &parsed
	}
	if completedRaw.Valid && completedRaw.String != "" {
		parsed, err := appdb.ParseTimestamp(completedRaw.String)
		if err != nil {
			return Job{}, fmt.Errorf("parse generation job completed_at %q: %w", completedRaw.String, err)
		}
		job.CompletedAt = &parsed
	}

	job.CreatedAt, err = appdb.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Job{}, fmt.Errorf("parse generation job created_at %q: %w", createdAtRaw, err)
	}
	job.UpdatedAt, err = appdb.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return Job{}, fmt.Errorf("parse generation job updated_at %q: %w", updatedAtRaw, err)
	}

	if summaryJSON != "" {
		if err := json.Unmarshal([]byte(summaryJSON), &job.Summary); err != nil {
			return Job{}, fmt.Errorf("decode generation job summary for %q: %w", job.ID, err)
		}
	}

	return job, nil
}

func newJobID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return "job_" + hex.EncodeToString(buf)
}

func normalizeJobLimit(limit int) int {
	if limit <= 0 || limit > 1000 {
		return 100
	}
	return limit
}

func normalizeJobOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
