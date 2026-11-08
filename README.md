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

**Playlists.** `POST /v1/resolve` also accepts a playlist link. The response then
has `"kind": "playlist"` and a `playlist` block; single videos return
`"kind": "video"` and are otherwise unchanged (older clients ignore the new
fields):
```json
{"kind": "playlist", "title": "…", "variants": [ …fixed quality ladder + MP3… ],
 "playlist": {"count": 5, "total": 116, "truncated": true,
              "entries": [{"index": 1, "title": "…", "url": "https://…",
                           "duration_seconds": 4296, "thumbnail": "https://…"}]}}
```
`index` is the 1-based position in the original playlist. Private and deleted
videos are left out, so indices can have gaps. Listings are capped at
`PLAYLIST_MAX_ITEMS` (default 50, max 200); `truncated` says there are more.
A `watch?v=…&list=…` link still resolves as that one video.

**POST /v1/playlists**
```json
{"url": "https://…/playlist?list=…", "format": "bestaudio/best",
 "audio_only": true, "items": [1, 3, 4]}
```
`items` are positions from `/v1/resolve`; omit it for every entry. Each chosen
video becomes an ordinary download job, so workers, clips, retention and
`GET /v1/jobs` history behave exactly as for single links. Returns `202` with
`{"id", "title", "total", "queued"}`, or `503` (`queue_unavailable`) if nothing
could be queued.

**GET /v1/playlists/{id}** returns the playlist with per-video jobs in playlist
order (each with `position`, `status`, and presigned `clips` once done) plus
`status` (`queued` / `processing` / `completed` / `failed`), `total`,
`completed` and `failed`. It settles as `completed` when nothing is pending and
at least one video succeeded, so a few unavailable videos don't hold it open.

**GET /v1/config** returns the limits a client can check before doing work:
`{"max_upload_bytes": 536870912, "allowed_upload_extensions": [".m4v", ".mkv", ".mov", ".mp4", ".webm"]}`.
It needs no auth and no database.

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

## Sign-in and daily limits

Downloads are limited per day so one person can't use up the egress. Anyone can download a few videos; signing in with Google raises the limit.

| Setting | Default | Meaning |
|---|---|---|
| `ANON_DAILY_LIMIT` | 3 | per device id, when not signed in |
| `ANON_IP_DAILY_LIMIT` | 9 | per IP, so rotating device ids doesn't help (higher, because networks are shared) |
| `USER_DAILY_LIMIT` | 20 | per Google account |
| `QUOTA_TZ` | UTC | the day rolls over at midnight in this time zone (e.g. `Asia/Kolkata`) |
| `GOOGLE_CLIENT_ID` | | a **Web application** OAuth client id; sign-in stays off without it |
| `SESSION_SECRET` | | 32+ random characters that sign session tokens; sign-in stays off without it |
| `SESSION_TTL_HOURS` | 168 | how long a session lasts |
| `TRUSTED_PROXY_CIDRS` | | proxies whose `X-Forwarded-For` is believed (see below) |

`0` means unlimited. What counts is a new job for a link (each video in a playlist counts) and it is charged when the job is created, in the same transaction, so a burst of requests can't slip past the limit. A job that fails, or that `cmd/purge` cancels, gives its count back. Uploading your own file to split isn't limited.

- `POST /v1/auth/google` `{"credential": "<Google ID token>"}` checks the token (signature against Google's keys, issuer, audience, expiry, verified email) and returns `{token, expires_at, user}`. Send the token as `Authorization: Bearer <token>`. A missing, invalid or expired token is simply treated as anonymous.
- `GET /v1/me` returns `{signed_in, auth_enabled, limit, used, remaining, resets_at, signed_in_limit, user}`.
- Over the limit, `POST /v1/jobs` and `POST /v1/playlists` return `429` with `{code: "daily_limit", error, limit, remaining, resets_at, signed_in}` and a `Retry-After` header. A playlist is charged all at once or not created.
- Signed-in people's jobs are filed under their account, and `GET /v1/jobs` also lists what their device downloaded before they signed in.

**Google setup:** in Google Cloud Console create an OAuth client of type *Web application*, add your site's origins (for example `http://localhost:3000` and your production URL) under Authorized JavaScript origins, set its id as `GOOGLE_CLIENT_ID` on the API and `NEXT_PUBLIC_GOOGLE_CLIENT_ID` on the web app. The iOS client id in `Info.plist` is a different, iOS-type client and can't be reused here.

**Behind a proxy (important):** anonymous limits rely on the caller's IP. `X-Forwarded-For` can be sent by anyone, so it is only believed when the request comes from an address in `TRUSTED_PROXY_CIDRS`, and then the right-most entry that isn't itself a trusted proxy is used. If the web app calls this API, list the web server's address as the API sees it. The web app must itself sit behind a reverse proxy or platform that sets `X-Forwarded-For` (nginx, Cloudflare, Cloud Run and Vercel all do); Next.js passes the header through but doesn't add the connecting address, so a Next server exposed directly can't tell visitors apart. With Cloudflare in front, also list Cloudflare's ranges so they are skipped over. If the list is empty the API logs a warning and identifies callers by the address that connects to it.

The iOS app keeps working without signing in and shares the anonymous allowance (per device and IP). Giving it the higher limit means adding Google sign-in there and sending the session token.

## Pruning the queue

`cmd/purge` clears stuck work out of the queue. It only looks unless you pass `--apply`, and it prints which database and Redis it is pointed at (never the credentials) before doing anything.

```bash
go run ./cmd/purge                      # dry run: what would be removed
go run ./cmd/purge --apply              # do it
go run ./cmd/purge --older-than 2h      # only jobs idle at least 2h (default 30m)
go run ./cmd/purge --user <device-id>   # only one user's or device's jobs
go run ./cmd/purge --clear-finished     # also drop completed/archived task history
```

It marks stuck queued/downloading/processing jobs failed (`error_code: cancelled`) and deletes only the queue tasks tied to them, plus tasks whose job is already finished or gone. It never touches a job a worker is running right now (a long download can be quiet and still healthy), never touches tasks it can't read, and if a task can't be removed it leaves that job alone. The old version wiped every Redis task and failed every unfinished job.

## Not done yet

Auth (every endpoint is open), rate limiting, per-user quotas, a resolve cache
(table exists, unused), metrics, and an S3 lifecycle policy.

## Before this goes public

Hosting a downloader for YouTube and Instagram content is against both
platforms' terms, and YouTube in particular has pursued download services on
DMCA §1201 circumvention grounds. App stores delist these regularly and payment
processors drop them. That's a business risk to price in, not a code problem —
but it is the reason to keep the splitter path independent of the resolver.
