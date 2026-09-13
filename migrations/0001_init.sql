CREATE TABLE IF NOT EXISTS jobs (
    id              UUID PRIMARY KEY,
    source_key      TEXT        NOT NULL,
    segment_seconds INT         NOT NULL,
    mode            TEXT        NOT NULL DEFAULT 'copy',
    status          TEXT        NOT NULL DEFAULT 'queued',
    error           TEXT,
    duration_ms     BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS jobs_status_created_idx ON jobs (status, created_at DESC);

CREATE TABLE IF NOT EXISTS clips (
    id          UUID PRIMARY KEY,
    job_id      UUID   NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    idx         INT    NOT NULL,
    object_key  TEXT   NOT NULL,
    duration_ms BIGINT NOT NULL,
    size_bytes  BIGINT NOT NULL,
    UNIQUE (job_id, idx)
);
