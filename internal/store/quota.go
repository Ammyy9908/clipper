package store

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrQuotaExceeded means a charge would push a bucket past its daily limit.
var ErrQuotaExceeded = errors.New("daily download limit reached")

// QuotaKey is one bucket a download is charged to, with that bucket's limit.
// Anonymous requests are charged to two buckets (device and IP) and must fit in both.
type QuotaKey struct {
	Key   string
	Limit int
}

// QuotaCharge is what a new job (or playlist) is billed against.
type QuotaCharge struct {
	Day  string // YYYY-MM-DD in the quota time zone
	Keys []QuotaKey
}

// consume adds n to every bucket, all or nothing. It runs inside the caller's
// transaction, so charging and creating the job commit or roll back together.
//
// The increment is a single INSERT ... ON CONFLICT DO UPDATE ... WHERE, which
// Postgres evaluates under the row lock: two concurrent requests can never both
// take the last slot. Buckets are locked in sorted order so concurrent charges
// can't deadlock each other.
func consume(ctx context.Context, tx pgx.Tx, c QuotaCharge, n int) error {
	keys := append([]QuotaKey(nil), c.Keys...)
	sort.Slice(keys, func(i, j int) bool { return keys[i].Key < keys[j].Key })

	for _, k := range keys {
		if k.Limit <= 0 { // unlimited bucket
			continue
		}
		if n > k.Limit {
			return ErrQuotaExceeded
		}
		var count int
		err := tx.QueryRow(ctx,
			`INSERT INTO download_usage (key, day, count) VALUES ($1, $2::date, $3)
			 ON CONFLICT (key, day) DO UPDATE SET count = download_usage.count + $3
			 WHERE download_usage.count + $3 <= $4
			 RETURNING count`, k.Key, c.Day, n, k.Limit).Scan(&count)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrQuotaExceeded
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// quotaKeys is the list stored on a job so a refund knows what to give back.
func (c QuotaCharge) chargedKeys() []string {
	var out []string
	for _, k := range c.Keys {
		if k.Limit > 0 {
			out = append(out, k.Key)
		}
	}
	return out
}

// CreateJobCharged inserts a remote job and charges it to its quota buckets in
// one transaction. It returns ErrQuotaExceeded, creating nothing, when any bucket is full.
func (s *Store) CreateJobCharged(ctx context.Context, j Job, c QuotaCharge) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := consume(ctx, tx, c, 1); err != nil {
		return err
	}
	keys := c.chargedKeys()
	var (
		qKeys []string
		qDay  *string
	)
	if len(keys) > 0 {
		qKeys, qDay = keys, &c.Day
	}
	if j.TextPosition == "" {
		j.TextPosition = "top"
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO jobs (id, source_type, source_key, source_url, format, audio_only,
		                   segment_seconds, aspect, mode, status, user_id,
		                   text_overlay, show_part_counter, text_position,
		                   quota_keys, quota_day)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::date)`,
		j.ID, j.SourceType, j.SourceKey, j.SourceURL, j.Format, j.AudioOnly,
		j.SegmentSeconds, j.Aspect, j.Mode, StatusQueued, j.UserID,
		j.TextOverlay, j.ShowPartCounter, j.TextPosition,
		qKeys, qDay); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Remaining is how many more downloads the tightest bucket allows today.
// It returns -1 when none of the buckets has a limit.
func (s *Store) Remaining(ctx context.Context, day string, keys []QuotaKey) (int, error) {
	remaining := -1
	for _, k := range keys {
		if k.Limit <= 0 {
			continue
		}
		var used int
		err := s.pool.QueryRow(ctx,
			`SELECT count FROM download_usage WHERE key = $1 AND day = $2::date`, k.Key, day).Scan(&used)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
		left := k.Limit - used
		if left < 0 {
			left = 0
		}
		if remaining < 0 || left < remaining {
			remaining = left
		}
	}
	return remaining, nil
}

// refund gives back the quota charged to jobs that didn't produce a download.
// A job is refunded at most once, however many times this is called for it.
func (s *Store) refund(ctx context.Context, ids ...uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		_ = s.RefundJobCredits(ctx, id)
	}

	// Several jobs in one batch usually share a bucket. UPDATE ... FROM applies
	// only one matching row to each target row, so the jobs are counted per
	// bucket first and each bucket is decremented by its total.
	_, err := s.pool.Exec(ctx,
		`WITH j AS (
		     UPDATE jobs SET quota_refunded = true
		     WHERE id = ANY($1) AND quota_keys IS NOT NULL AND NOT quota_refunded
		     RETURNING quota_keys, quota_day
		 ), owed AS (
		     SELECT unnest(quota_keys) AS key, quota_day AS day, count(*) AS n
		     FROM j GROUP BY 1, 2
		 )
		 UPDATE download_usage u SET count = GREATEST(u.count - owed.n, 0)
		 FROM owed
		 WHERE u.key = owed.key AND u.day = owed.day`, ids)
	return err
}

// PruneUsage drops usage rows old enough that no refund can still point at them.
func (s *Store) PruneUsage(ctx context.Context, keepDays int) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM download_usage WHERE day < (now() - make_interval(days => $1))::date`, keepDays)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// User is a signed-in Google account.
type User struct {
	ID      string
	Email   string
	Name    string
	Picture string
}

// UpsertUser records a login, creating the user on first sight.
func (s *Store) UpsertUser(ctx context.Context, u User) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, email, name, picture) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE
		 SET email = EXCLUDED.email, name = EXCLUDED.name, picture = EXCLUDED.picture, last_login_at = now()`,
		u.ID, u.Email, u.Name, u.Picture)
	return err
}

// ListUserJobsFor is ListUserJobs across several identities. A signed-in person
// sees the jobs made under their account and the ones their device made before signing in.
func (s *Store) ListUserJobsFor(ctx context.Context, userIDs []string, limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_type, source_key, source_url, format, audio_only, title,
		        segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		        created_at, updated_at, user_id
		 FROM jobs
		 WHERE user_id = ANY($1)
		 ORDER BY created_at DESC
		 LIMIT $2`, userIDs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.AudioOnly, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt, &j.UserID); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
