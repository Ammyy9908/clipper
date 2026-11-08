package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UnfinishedJob is a job that hasn't reached completed or failed.
type UnfinishedJob struct {
	ID        uuid.UUID
	Status    string
	UserID    *string
	Title     *string
	SourceURL *string
	UpdatedAt time.Time
}

// ListUnfinished returns queued, downloading and processing jobs that haven't
// changed state for at least olderThan, oldest first. A non-empty userID limits
// it to that user or device.
func (s *Store) ListUnfinished(ctx context.Context, olderThan time.Duration, userID string) ([]UnfinishedJob, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, status, user_id, title, source_url, updated_at
		 FROM jobs
		 WHERE status IN ('queued', 'downloading', 'processing')
		   AND updated_at < now() - make_interval(secs => $1)
		   AND ($2 = '' OR user_id = $2)
		 ORDER BY updated_at`, olderThan.Seconds(), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UnfinishedJob
	for rows.Next() {
		var j UnfinishedJob
		if err := rows.Scan(&j.ID, &j.Status, &j.UserID, &j.Title, &j.SourceURL, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// JobStatuses maps each id to its status. Ids with no row are absent from the
// result, which is how a caller tells "deleted" from "still running".
func (s *Store) JobStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, status FROM jobs WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

// CancelUnfinished marks the given jobs failed with the "cancelled" code. It
// only touches jobs that are still unfinished, so a job that completed after it
// was listed is never overwritten.
func (s *Store) CancelUnfinished(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	rows, err := s.pool.Query(ctx,
		`UPDATE jobs
		 SET status = $2, error = 'cancelled by admin purge', error_code = 'cancelled', updated_at = now()
		 WHERE id = ANY($1) AND status IN ('queued', 'downloading', 'processing')
		 RETURNING id`,
		ids, StatusFailed)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var cancelled []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		cancelled = append(cancelled, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	// Cancelled work never produced a download, so it doesn't count against the
	// daily limit. Only the jobs actually cancelled are refunded, never one that
	// finished after it was listed.
	if err := s.refund(ctx, cancelled...); err != nil {
		return int64(len(cancelled)), err
	}
	return int64(len(cancelled)), nil
}
