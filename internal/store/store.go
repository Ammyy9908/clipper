package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

const (
	SourceUpload = "upload"
	SourceRemote = "remote"
)

type Job struct {
	ID             uuid.UUID `json:"id"`
	SourceType     string    `json:"source_type"`
	SourceKey      *string   `json:"-"`
	SourceURL      *string   `json:"source_url,omitempty"`
	Format         *string   `json:"format,omitempty"`
	Title          *string   `json:"title,omitempty"`
	SegmentSeconds int       `json:"segment_seconds"`
	Aspect         string    `json:"aspect,omitempty"`
	Mode           string    `json:"mode"`
	Status         string    `json:"status"`
	Error          *string   `json:"error,omitempty"`
	ErrorCode      *string   `json:"error_code,omitempty"`
	DurationMS     *int64    `json:"duration_ms,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Clips          []Clip    `json:"clips"`
}

type Clip struct {
	Index      int    `json:"index"`
	ObjectKey  string `json:"-"`
	URL        string `json:"url,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes"`
}

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) CreateJob(ctx context.Context, j Job) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO jobs (id, source_type, source_key, source_url, format,
		                   segment_seconds, aspect, mode, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		j.ID, j.SourceType, j.SourceKey, j.SourceURL, j.Format,
		j.SegmentSeconds, j.Aspect, j.Mode, StatusQueued)
	return err
}

func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (Job, error) {
	var j Job
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_type, source_key, source_url, format, title,
		        segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		        created_at, updated_at
		 FROM jobs WHERE id = $1`, id).
		Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	if err != nil {
		return j, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT idx, object_key, duration_ms, size_bytes
		 FROM clips WHERE job_id = $1 ORDER BY idx`, id)
	if err != nil {
		return j, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Clip
		if err := rows.Scan(&c.Index, &c.ObjectKey, &c.DurationMS, &c.SizeBytes); err != nil {
			return j, err
		}
		j.Clips = append(j.Clips, c)
	}
	return j, rows.Err()
}

func (s *Store) SetStatus(ctx context.Context, id uuid.UUID, status string, errMsg, errCode *string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, error = $3, error_code = $4, updated_at = now()
		 WHERE id = $1`, id, status, errMsg, errCode)
	return err
}

// SetTitle records the resolved title once the worker has metadata in hand.
func (s *Store) SetTitle(ctx context.Context, id uuid.UUID, title string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET title = $2, updated_at = now() WHERE id = $1`, id, title)
	return err
}

// Complete writes the clip rows and flips the job to completed in one transaction,
// so a job is never "completed" with a partial clip list.
func (s *Store) Complete(ctx context.Context, id uuid.UUID, durationMS int64, clips []Clip) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM clips WHERE job_id = $1`, id); err != nil {
		return err
	}
	for _, c := range clips {
		if _, err := tx.Exec(ctx,
			`INSERT INTO clips (id, job_id, idx, object_key, duration_ms, size_bytes)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.New(), id, c.Index, c.ObjectKey, c.DurationMS, c.SizeBytes); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE jobs SET status = $2, duration_ms = $3, error = NULL, updated_at = now()
		 WHERE id = $1`, id, StatusCompleted, durationMS); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
