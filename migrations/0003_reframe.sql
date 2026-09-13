-- Optional aspect-ratio conversion, applied before segmenting.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS aspect TEXT NOT NULL DEFAULT '';
