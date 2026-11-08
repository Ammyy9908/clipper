-- A playlist request fans out into one ordinary job per video. The playlists
-- row groups them so a client can poll a single id for overall progress.
CREATE TABLE IF NOT EXISTS playlists (
    id         UUID PRIMARY KEY,
    user_id    TEXT,
    source_url TEXT        NOT NULL,
    title      TEXT        NOT NULL DEFAULT '',
    total      INT         NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS playlists_user_created_idx ON playlists (user_id, created_at DESC);

-- Deleting a playlist (retention pruning) removes its jobs and, through the
-- existing clips foreign key, their clip rows.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS playlist_id  UUID REFERENCES playlists (id) ON DELETE CASCADE;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS playlist_pos INT;

CREATE INDEX IF NOT EXISTS jobs_playlist_idx ON jobs (playlist_id, playlist_pos) WHERE playlist_id IS NOT NULL;
