-- Google sign-in and the daily download limit.

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,           -- "google:<sub>"
    email         TEXT        NOT NULL,
    name          TEXT,
    picture       TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per quota bucket per day: "user:<sub>", "dev:<device id>" or "ip:<addr>".
CREATE TABLE IF NOT EXISTS download_usage (
    key   TEXT NOT NULL,
    day   DATE NOT NULL,
    count INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (key, day)
);

-- Which buckets a job was charged to, so a failed job can give the count back
-- exactly once.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_keys     TEXT[];
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_day      DATE;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS quota_refunded BOOLEAN NOT NULL DEFAULT false;
