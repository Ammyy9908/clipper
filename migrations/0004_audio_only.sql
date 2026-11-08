-- Audio-only jobs (save as mp3) skip the video pipeline entirely.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS audio_only BOOLEAN NOT NULL DEFAULT false;
