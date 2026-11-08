-- Optional on-screen text and part counter overlay for split jobs.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS text_overlay TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS show_part_counter BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS text_position TEXT NOT NULL DEFAULT 'top';
