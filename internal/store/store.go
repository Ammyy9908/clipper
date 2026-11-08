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
	StatusQueued      = "queued"
	StatusDownloading = "downloading"
	StatusProcessing  = "processing"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

const (
	SourceUpload = "upload"
	SourceRemote = "remote"
)

type Job struct {
	ID              uuid.UUID `json:"id"`
	UserID          *string   `json:"user_id,omitempty"`
	SourceType      string    `json:"source_type"`
	SourceKey       *string   `json:"-"`
	SourceURL       *string   `json:"source_url,omitempty"`
	Format          *string   `json:"format,omitempty"`
	AudioOnly       bool      `json:"audio_only"`
	Title           *string   `json:"title,omitempty"`
	SegmentSeconds  int       `json:"segment_seconds"`
	Aspect          string    `json:"aspect,omitempty"`
	Mode            string    `json:"mode"`
	TextOverlay     string    `json:"text_overlay,omitempty"`
	ShowPartCounter bool      `json:"show_part_counter,omitempty"`
	TextPosition    string    `json:"text_position,omitempty"`
	Status          string    `json:"status"`
	Error           *string   `json:"error,omitempty"`
	ErrorCode       *string   `json:"error_code,omitempty"`
	DurationMS      *int64    `json:"duration_ms,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Clips           []Clip    `json:"clips"`
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
	s := &Store{pool: pool}
	if err := s.EnsureSchema(ctx); err != nil {
		// Log warning or return error if schema migration fails
		return s, nil
	}
	return s, nil
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	queries := []string{
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id TEXT;`,
		`CREATE INDEX IF NOT EXISTS jobs_user_id_created_idx ON jobs (user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS jobs_active_source_idx ON jobs (source_url, status) WHERE status IN ('queued', 'downloading', 'processing');`,
		// Playlists (migrations/0006_playlists.sql), mirrored here because
		// production doesn't run the migrations directory automatically.
		`CREATE TABLE IF NOT EXISTS playlists (
			id UUID PRIMARY KEY,
			user_id TEXT,
			source_url TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			total INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE INDEX IF NOT EXISTS playlists_user_created_idx ON playlists (user_id, created_at DESC);`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS playlist_id UUID REFERENCES playlists (id) ON DELETE CASCADE;`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS playlist_pos INT;`,
		`CREATE INDEX IF NOT EXISTS jobs_playlist_idx ON jobs (playlist_id, playlist_pos) WHERE playlist_id IS NOT NULL;`,
		// Sign-in and daily limits (migrations/0007_auth_quota.sql).
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT,
			picture TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_login_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE TABLE IF NOT EXISTS download_usage (
			key TEXT NOT NULL,
			day DATE NOT NULL,
			count INT NOT NULL DEFAULT 0,
			PRIMARY KEY (key, day)
		);`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_keys TEXT[];`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_day DATE;`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_refunded BOOLEAN NOT NULL DEFAULT false;`,
		// On-screen text & part counter (migrations/0008_text_overlay.sql).
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS text_overlay TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS show_part_counter BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE jobs ADD COLUMN IF NOT EXISTS text_position TEXT NOT NULL DEFAULT 'top';`,
		// Credits and Razorpay payment orders (migrations/0009_credits.sql).
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS credits_balance INT NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS daily_free_credits INT NOT NULL DEFAULT 10;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_refill_date DATE NOT NULL DEFAULT CURRENT_DATE;`,
		`CREATE TABLE IF NOT EXISTS credit_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			amount INT NOT NULL,
			balance_after INT NOT NULL,
			task_type TEXT NOT NULL,
			job_id UUID,
			payment_id TEXT,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE INDEX IF NOT EXISTS credit_transactions_user_created_idx ON credit_transactions (user_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS payment_orders (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			razorpay_order_id TEXT UNIQUE NOT NULL,
			razorpay_payment_id TEXT,
			pack_id TEXT NOT NULL,
			credits INT NOT NULL,
			amount INT NOT NULL,
			currency TEXT NOT NULL DEFAULT 'INR',
			status TEXT NOT NULL DEFAULT 'created',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			paid_at TIMESTAMPTZ
		);`,
		`CREATE INDEX IF NOT EXISTS payment_orders_user_idx ON payment_orders (user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS payment_orders_rzp_idx ON payment_orders (razorpay_order_id);`,
	}
	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) CreateJob(ctx context.Context, j Job) error {
	if j.TextPosition == "" {
		j.TextPosition = "top"
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO jobs (id, source_type, source_key, source_url, format, audio_only,
		                   segment_seconds, aspect, mode, status, user_id,
		                   text_overlay, show_part_counter, text_position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		j.ID, j.SourceType, j.SourceKey, j.SourceURL, j.Format, j.AudioOnly,
		j.SegmentSeconds, j.Aspect, j.Mode, StatusQueued, j.UserID,
		j.TextOverlay, j.ShowPartCounter, j.TextPosition)
	return err
}

func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (Job, error) {
	var j Job
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_type, source_key, source_url, format, audio_only, title,
		        segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		        created_at, updated_at, user_id,
		        text_overlay, show_part_counter, text_position
		 FROM jobs WHERE id = $1`, id).
		Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.AudioOnly, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt, &j.UserID,
			&j.TextOverlay, &j.ShowPartCounter, &j.TextPosition)
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

// FindActiveJob looks for an in-flight job for the same source URL, format, and split settings.
func (s *Store) FindActiveJob(ctx context.Context, sourceURL string, format *string, audioOnly bool, segmentSeconds int, aspect string, textOverlay string, showPartCounter bool, textPosition string) (*Job, error) {
	var j Job
	query := `
		SELECT id, source_type, source_key, source_url, format, audio_only, title,
		       segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		       created_at, updated_at, user_id,
		       text_overlay, show_part_counter, text_position
		FROM jobs
		WHERE source_url = $1
		  AND (
		      (status IN ('queued', 'downloading', 'processing') AND updated_at > now() - interval '90 minutes')
		      OR (status = 'completed' AND created_at > now() - interval '24 hours')
		  )
		  AND audio_only = $2
		  AND segment_seconds = $3
		  AND aspect = $4
		  AND text_overlay = $5
		  AND show_part_counter = $6
		  AND text_position = $7
		  AND ($8::text IS NULL OR format = $8 OR format IS NULL)
		ORDER BY created_at DESC
		LIMIT 1`

	err := s.pool.QueryRow(ctx, query, sourceURL, audioOnly, segmentSeconds, aspect, textOverlay, showPartCounter, textPosition, format).
		Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.AudioOnly, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt, &j.UserID,
			&j.TextOverlay, &j.ShowPartCounter, &j.TextPosition)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ListUserJobs retrieves recent jobs created by a given user or device.
func (s *Store) ListUserJobs(ctx context.Context, userID string, limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_type, source_key, source_url, format, audio_only, title,
		        segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		        created_at, updated_at, user_id,
		        text_overlay, show_part_counter, text_position
		 FROM jobs
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.AudioOnly, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt, &j.UserID,
			&j.TextOverlay, &j.ShowPartCounter, &j.TextPosition); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *Store) SetStatus(ctx context.Context, id uuid.UUID, status string, errMsg, errCode *string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, error = $3, error_code = $4, updated_at = now()
		 WHERE id = $1`, id, status, errMsg, errCode)
	if err != nil {
		return err
	}
	// A download that failed didn't happen, so it doesn't count against the daily limit.
	if status == StatusFailed {
		return s.refund(ctx, id)
	}
	return nil
}

// SetTitle records the resolved title once the worker has metadata in hand.
func (s *Store) SetTitle(ctx context.Context, id uuid.UUID, title string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET title = $2, updated_at = now() WHERE id = $1`, id, title)
	return err
}

// SetSourceKey records the uploaded/stored source object key in R2.
func (s *Store) SetSourceKey(ctx context.Context, id uuid.UUID, sourceKey string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET source_key = $2, updated_at = now() WHERE id = $1`, id, sourceKey)
	return err
}

// SetSourceKeyAndTitle records both the source key and the title.
func (s *Store) SetSourceKeyAndTitle(ctx context.Context, id uuid.UUID, sourceKey, title string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET source_key = $2, title = $3, updated_at = now() WHERE id = $1`, id, sourceKey, title)
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

// PurgeStuck cancels any queued or processing jobs in the database.
func (s *Store) PurgeStuck(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE jobs SET status = $1, error = 'purged by admin', error_code = 'cancelled', updated_at = now() WHERE status = $2 OR status = $3 OR status = $4`,
		StatusFailed, StatusQueued, StatusDownloading, StatusProcessing)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// PruneExpired deletes jobs older than ttl, along with expired playlists (whose
// jobs go with them via ON DELETE CASCADE).
func (s *Store) PruneExpired(ctx context.Context, ttl time.Duration) (int64, error) {
	// Old usage rows can't be refunded into any more, so they can go.
	_, _ = s.PruneUsage(ctx, 7)
	pl, err := s.pool.Exec(ctx, `DELETE FROM playlists WHERE created_at < now() - $1::interval`, ttl.String())
	if err != nil {
		return 0, err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM jobs WHERE created_at < now() - $1::interval`, ttl.String())
	if err != nil {
		return pl.RowsAffected(), err
	}
	return pl.RowsAffected() + tag.RowsAffected(), nil
}
