-- Jobs can now originate from a remote URL instead of an uploaded object.
ALTER TABLE jobs ALTER COLUMN source_key DROP NOT NULL;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'upload';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_url  TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS format      TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS title       TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS error_code  TEXT;

-- segment_seconds = 0 means "download only, do not split".
ALTER TABLE jobs ALTER COLUMN segment_seconds SET DEFAULT 0;

CREATE TABLE IF NOT EXISTS resolve_cache (
    url_hash   TEXT PRIMARY KEY,
    payload    JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS resolve_cache_created_idx ON resolve_cache (created_at);
