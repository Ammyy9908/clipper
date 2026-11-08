package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Playlist groups the jobs created from one playlist URL.
type Playlist struct {
	ID        uuid.UUID `json:"id"`
	UserID    *string   `json:"user_id,omitempty"`
	SourceURL string    `json:"source_url"`
	Title     string    `json:"title"`
	Total     int       `json:"total"`
	CreatedAt time.Time `json:"created_at"`
}

// PlaylistJob is a job to create as part of a playlist, with its 1-based
// position in the original listing (used for ordering and for the client to
// map results back to entries).
type PlaylistJob struct {
	Job      Job
	Position int
}

// CreatePlaylist inserts the playlist and all its jobs in one transaction, so a
// playlist never exists with only some of its jobs. The jobs are owned by the
// playlist's user, whatever their own UserID says, so they always appear in that
// user's history.
func (s *Store) CreatePlaylist(ctx context.Context, p Playlist, jobs []PlaylistJob, charge *QuotaCharge) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Every video in a playlist is a download. The whole playlist is charged at
	// once, so it either fits in the day's allowance or isn't created at all.
	var (
		qKeys []string
		qDay  *string
	)
	if charge != nil {
		if err := consume(ctx, tx, *charge, len(jobs)); err != nil {
			return err
		}
		if keys := charge.chargedKeys(); len(keys) > 0 {
			qKeys, qDay = keys, &charge.Day
		}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO playlists (id, user_id, source_url, title, total)
		 VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.UserID, p.SourceURL, p.Title, p.Total); err != nil {
		return err
	}

	for _, pj := range jobs {
		j := pj.Job
		if j.TextPosition == "" {
			j.TextPosition = "top"
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO jobs (id, source_type, source_url, format, audio_only, title,
			                   segment_seconds, aspect, mode, status, user_id,
			                   text_overlay, show_part_counter, text_position,
			                   playlist_id, playlist_pos, quota_keys, quota_day)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18::date)`,
			j.ID, j.SourceType, j.SourceURL, j.Format, j.AudioOnly, j.Title,
			j.SegmentSeconds, j.Aspect, j.Mode, StatusQueued, p.UserID,
			j.TextOverlay, j.ShowPartCounter, j.TextPosition,
			p.ID, pj.Position, qKeys, qDay); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// GetPlaylist returns the playlist and its jobs in playlist order, each with
// its clips. Clips are loaded in one query rather than one per job.
func (s *Store) GetPlaylist(ctx context.Context, id uuid.UUID) (Playlist, []PlaylistJob, error) {
	var p Playlist
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, source_url, title, total, created_at FROM playlists WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.SourceURL, &p.Title, &p.Total, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil, ErrNotFound
	}
	if err != nil {
		return p, nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, source_type, source_key, source_url, format, audio_only, title,
		        segment_seconds, aspect, mode, status, error, error_code, duration_ms,
		        created_at, updated_at, user_id, playlist_pos,
		        text_overlay, show_part_counter, text_position
		 FROM jobs WHERE playlist_id = $1 ORDER BY playlist_pos`, id)
	if err != nil {
		return p, nil, err
	}
	defer rows.Close()

	var jobs []PlaylistJob
	byID := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var pj PlaylistJob
		j := &pj.Job
		if err := rows.Scan(&j.ID, &j.SourceType, &j.SourceKey, &j.SourceURL, &j.Format, &j.AudioOnly, &j.Title,
			&j.SegmentSeconds, &j.Aspect, &j.Mode, &j.Status, &j.Error, &j.ErrorCode,
			&j.DurationMS, &j.CreatedAt, &j.UpdatedAt, &j.UserID, &pj.Position,
			&j.TextOverlay, &j.ShowPartCounter, &j.TextPosition); err != nil {
			return p, nil, err
		}
		byID[j.ID] = len(jobs)
		ids = append(ids, j.ID)
		jobs = append(jobs, pj)
	}
	if err := rows.Err(); err != nil {
		return p, nil, err
	}
	if len(ids) == 0 {
		return p, jobs, nil
	}

	clipRows, err := s.pool.Query(ctx,
		`SELECT job_id, idx, object_key, duration_ms, size_bytes
		 FROM clips WHERE job_id = ANY($1) ORDER BY job_id, idx`, ids)
	if err != nil {
		return p, nil, err
	}
	defer clipRows.Close()
	for clipRows.Next() {
		var jobID uuid.UUID
		var c Clip
		if err := clipRows.Scan(&jobID, &c.Index, &c.ObjectKey, &c.DurationMS, &c.SizeBytes); err != nil {
			return p, nil, err
		}
		i := byID[jobID]
		jobs[i].Job.Clips = append(jobs[i].Job.Clips, c)
	}
	return p, jobs, clipRows.Err()
}
