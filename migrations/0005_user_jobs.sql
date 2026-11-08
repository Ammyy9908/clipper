-- Add user/device identification to jobs
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id TEXT;

-- Index for quickly fetching active or recent jobs belonging to a user/device
CREATE INDEX IF NOT EXISTS jobs_user_id_created_idx ON jobs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS jobs_active_source_idx ON jobs (source_url, status) WHERE status IN ('queued', 'downloading', 'processing');
