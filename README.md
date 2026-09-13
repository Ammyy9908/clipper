# clipper

Backend for a video downloader + splitter. Two ways in:

- **upload** a file and get it split into clips
- **paste a link** (YouTube, Instagram, and the ~1800 sites yt-dlp supports)
  and get the video back, optionally split

Both paths converge on the same job pipeline, so the splitter works whether or
not the resolver is healthy.

## Shape

```
client  --(1) POST /v1/uploads-->  api      presigned PUT
client  --(2) PUT to S3--------->  S3
client  --(3) POST /v1/jobs----->  api  --> redis (asynq)
                                            |
worker  <-----------------------------------+
worker  -- download -> ffprobe -> ffmpeg segment -> upload clips -> postgres
client  --(4) GET /v1/jobs/{id}->  api      presigned GET per clip
```

Uploads go straight to object storage. The API never touches video bytes, so
API pods stay small and stateless while workers scale on CPU and scratch disk.

## Endpoints

**POST /v1/resolve** — `{"url": "https://..."}`
Synchronous metadata call. Returns title, duration, thumbnail and a collapsed
list of variants (one per resolution rung, with a yt-dlp `selector` string and
approximate size). This is the call that breaks when a platform changes; it
fails before any bandwidth is spent.

**POST /v1/uploads** — `{"filename": "trip.mp4"}`
Returns `object_key`, a presigned `upload_url`, and the size cap. PUT the file
to that URL yourself.

**POST /v1/jobs** — exactly one of `object_key` or `url`:
```json
{"url": "https://...", "format": "bv*[height<=1080]+ba/b", "segment_seconds": 30}
{"object_key": "uploads/...", "segment_seconds": 30, "mode": "copy"}
```
`segment_seconds: 0` means download only, no split. `mode` is `copy` (default)
or `precise`. Returns `202` with a job id.

**GET /v1/jobs/{id}**
Returns status (`queued` / `processing` / `completed` / `failed`) and, once
complete, the clip list with presigned URLs.

## copy vs precise

`copy` remuxes without re-encoding: instant, lossless, but cuts snap to the
nearest keyframe, so boundaries land up to a GOP (often 1–2s) early. Good
enough for status clips.

`precise` re-encodes first with `-force_key_frames` on every boundary, then
segments by copy. Exact cuts, roughly 50x the CPU. Keep it opt-in.

## Local

```bash
docker compose up --build
# create the bucket once
docker compose exec minio mc alias set local http://localhost:9000 minioadmin minioadmin
docker compose exec minio mc mb local/clipper-dev
```

Migrations in `migrations/` are applied automatically by the Postgres image on
first boot. In production run them with your own migration tool.

## Failure taxonomy

Extraction failures are classified rather than lumped together, because the
right response differs and retrying the wrong one wastes worker slots:

| code | meaning | fix | retried? |
|---|---|---|---|
| `unavailable` | private, deleted, geo-blocked, age-gated | nothing | no |
| `blocked` | egress IP or session rejected | different proxy | yes |
| `extractor_stale` | platform changed, yt-dlp can't parse | bump yt-dlp | no |
| `unsupported` | site has no extractor | nothing | no |
| `live_stream` | live content | nothing | no |

`extractor_stale` is the one to alert on. It means ship a new image, and it
will fire without warning on someone else's release schedule.

## Operational notes

- **Disk, not CPU, is the binding constraint.** A 4K source plus its clips can
  be several GB of scratch. `WORKER_CONCURRENCY` defaults to 2; size it against
  the ephemeral volume, and give worker pods a real `emptyDir` sizeLimit.
- **Every job has a hard timeout** (`JOB_TIMEOUT_MIN`, default 20). Without it
  one wedged ffmpeg pins a worker slot forever.
- **Scratch dirs are removed in a defer** on every path, success or failure.
- **Outputs are presigned at read time**, so links expire without a cleanup job.
  Add an S3 lifecycle rule on `outputs/` and `uploads/` to actually delete them.
- Retries are capped at 2, and terminal failures skip retry entirely.
- **yt-dlp is a live dependency, not a pinned library.** Rebuild the worker
  image on a schedule. An image that is a month old will have broken YouTube
  in it. Log the version on boot (the worker does) so the first diagnostic
  question is already answered.
- **Egress reputation is the real cost line.** Datacenter IPs get the
  "confirm you're not a bot" wall on YouTube quickly. `PROXY_POOL` takes a
  comma-separated list and the resolver picks per-request; residential
  bandwidth runs roughly $3-15/GB, which for video dominates every other cost
  in this system. Model that before pricing anything.
- **You are proxying the full stream.** Every download is egress twice —
  in from the platform, out to the client. Budget accordingly.

## Not done yet

Auth (every endpoint is open), rate limiting, per-user quotas, a resolve cache
(table exists, unused), metrics, and an S3 lifecycle policy.

## Before this goes public

Hosting a downloader for YouTube and Instagram content is against both
platforms' terms, and YouTube in particular has pursued download services on
DMCA §1201 circumvention grounds. App stores delist these regularly and payment
processors drop them. That's a business risk to price in, not a code problem —
but it is the reason to keep the splitter path independent of the resolver.
