package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/media"
	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/storage"
	"github.com/ammyy9908/clipper/internal/store"
	"golang.org/x/sync/singleflight"
)

type API struct {
	auth  authState
	lim   limits
	cfg   config.Config
	db    *store.Store
	q     *asynq.Client
	rdb   *redis.Client
	files *storage.Store
	res   *resolver.Resolver
	log   *slog.Logger
	sf    singleflight.Group
}

func New(cfg config.Config, db *store.Store, q *asynq.Client, rdb *redis.Client, files *storage.Store, res *resolver.Resolver, log *slog.Logger) *API {
	return &API{cfg: cfg, db: db, q: q, rdb: rdb, files: files, res: res, log: log, auth: newAuthState(cfg, log), lim: newLimits(cfg)}
}

func newLimits(cfg config.Config) limits {
	return limits{
		resolve: newLimiter(cfg.RateResolve),
		write:   newLimiter(cfg.RateWrite),
		auth:    newLimiter(cfg.RateAuth),
		read:    newLimiter(cfg.RateRead),
	}
}

// Routes. Everything that starts work or costs a lookup is rate limited per IP;
// /healthz and /v1/config are cheap and left open.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /v1/config", a.publicConfig)
	mux.HandleFunc("POST /v1/resolve", a.limited(a.lim.resolve, a.resolve))
	mux.HandleFunc("POST /v1/uploads", a.limited(a.lim.write, a.createUpload))
	mux.HandleFunc("POST /v1/jobs", a.limited(a.lim.write, a.createJob))
	mux.HandleFunc("GET /v1/jobs", a.limited(a.lim.read, a.listJobs))
	mux.HandleFunc("GET /v1/jobs/{id}", a.limited(a.lim.read, a.getJob))
	mux.HandleFunc("GET /v1/jobs/{id}/ws", a.limited(a.lim.read, a.jobWebSocket))
	mux.HandleFunc("POST /v1/auth/google", a.limited(a.lim.auth, a.authGoogle))
	mux.HandleFunc("GET /v1/me", a.limited(a.lim.read, a.me))
	mux.HandleFunc("POST /v1/playlists", a.limited(a.lim.write, a.createPlaylist))
	mux.HandleFunc("GET /v1/playlists/{id}", a.limited(a.lim.read, a.getPlaylist))
	return a.logging(a.cors(mux))
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createUploadReq struct {
	Filename string `json:"filename"`
}

type createUploadResp struct {
	ObjectKey string `json:"object_key"`
	UploadURL string `json:"upload_url"`
	ExpiresIn int    `json:"expires_in"`
	MaxBytes  int64  `json:"max_bytes"`
}

// createUpload hands back a presigned PUT so the client uploads straight to S3.
// Proxying multi-hundred-MB video through the API pods would be the first thing
// to fall over under any real load.
func (a *API) createUpload(w http.ResponseWriter, r *http.Request) {
	var req createUploadReq
	if err := decode(r, &req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}

	ext := strings.ToLower(path.Ext(req.Filename))
	if !allowedExt[ext] {
		badRequest(w, "unsupported file type; allowed: .mp4 .mov .m4v .webm .mkv")
		return
	}

	key := "uploads/" + uuid.NewString() + ext
	url, err := a.files.PresignPut(r.Context(), key, "video/mp4", a.cfg.PresignTTL)
	if err != nil {
		a.log.Error("presign put failed", "err", err)
		serverError(w)
		return
	}

	writeJSON(w, http.StatusCreated, createUploadResp{
		ObjectKey: key,
		UploadURL: url,
		ExpiresIn: int(a.cfg.PresignTTL.Seconds()),
		MaxBytes:  a.cfg.MaxUploadBytes,
	})
}

type createJobReq struct {
	ObjectKey       string `json:"object_key"`        // from POST /v1/uploads
	URL             string `json:"url"`               // or a remote link
	Format          string `json:"format"`            // variant selector from /v1/resolve
	AudioOnly       bool   `json:"audio_only"`        // extract to mp3 instead of the video pipeline
	SegmentSeconds  int    `json:"segment_seconds"`   // 0 = download only, no split
	Mode            string `json:"mode"`
	Aspect          string `json:"aspect"`
	TextOverlay     string `json:"text_overlay"`      // optional title/hook text overlay
	ShowPartCounter bool   `json:"show_part_counter"` // whether to overlay Part 1, Part 2...
	TextPosition    string `json:"text_position"`     // "top", "bottom", or "center"
	UserID          string `json:"user_id"`
}

func (a *API) createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobReq
	if err := decode(r, &req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if (req.ObjectKey == "") == (req.URL == "") {
		badRequest(w, "provide exactly one of object_key or url")
		return
	}
	if !validSelector(req.Format) {
		badRequest(w, "unsupported format")
		return
	}
	if req.SegmentSeconds != 0 && (req.SegmentSeconds < 5 || req.SegmentSeconds > 600) {
		badRequest(w, "segment_seconds must be 0 (no split) or between 5 and 600")
		return
	}
	if req.Mode == "" {
		req.Mode = media.ModeCopy
	}
	if req.Mode != media.ModeCopy && req.Mode != media.ModePrecise {
		badRequest(w, "mode must be 'copy' or 'precise'")
		return
	}

	if req.AudioOnly && req.SegmentSeconds != 0 {
		badRequest(w, "audio_only jobs can't be split")
		return
	}
	if !media.ValidAspect(req.Aspect) {
		badRequest(w, "aspect must be one of: "+strings.Join(aspectNames(), ", "))
		return
	}
	if req.AudioOnly && req.Aspect != media.AspectOriginal {
		badRequest(w, "audio_only jobs have no frame to reshape")
		return
	}

	if req.TextPosition == "" {
		req.TextPosition = "top"
	}
	if req.TextPosition != "top" && req.TextPosition != "bottom" && req.TextPosition != "center" {
		badRequest(w, "text_position must be 'top', 'bottom', or 'center'")
		return
	}
	if len(req.TextOverlay) > 120 {
		req.TextOverlay = req.TextOverlay[:120]
	}

	ident := a.identify(r, req.UserID)
	userID := ident.OwnerID

	job := store.Job{
		ID:              uuid.New(),
		SegmentSeconds:  req.SegmentSeconds,
		AudioOnly:       req.AudioOnly,
		Aspect:          req.Aspect,
		Mode:            req.Mode,
		TextOverlay:     strings.TrimSpace(req.TextOverlay),
		ShowPartCounter: req.ShowPartCounter,
		TextPosition:    req.TextPosition,
		Status:          store.StatusQueued,
	}
	if userID != "" {
		job.UserID = &userID
	}

	if req.ObjectKey != "" {
		if !strings.HasPrefix(req.ObjectKey, "uploads/") {
			badRequest(w, "unknown object_key")
			return
		}
		// Confirm the upload actually completed, and enforce the size cap here
		// rather than trusting the presigned PUT.
		size, err := a.files.HeadSize(r.Context(), req.ObjectKey)
		if err != nil {
			badRequest(w, "object not found; complete the upload first")
			return
		}
		if size > a.cfg.MaxUploadBytes {
			badRequest(w, "file exceeds maximum upload size")
			return
		}
		job.SourceType = store.SourceUpload
		job.SourceKey = &req.ObjectKey
	} else {
		req.URL = unshortenURL(r.Context(), req.URL)
		req.URL = normalizeURL(req.URL)
		if err := validateURL(req.URL); err != nil {
			badRequest(w, err.Error())
			return
		}

		var fmtPtr *string
		if req.Format != "" {
			fmtPtr = &req.Format
		}

		// Prevent redundant duplicate worker jobs if the same remote URL is already active
		if existing, err := a.db.FindActiveJob(r.Context(), req.URL, fmtPtr, req.AudioOnly, req.SegmentSeconds, req.Aspect, strings.TrimSpace(req.TextOverlay), req.ShowPartCounter, req.TextPosition); err == nil && existing != nil {
			a.log.Info("attaching client to existing active job", "job", existing.ID, "status", existing.Status, "url", req.URL)
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id":     existing.ID,
				"status": existing.Status,
			})
			return
		}

		job.SourceType = store.SourceRemote
		job.SourceURL = &req.URL
		if req.Format != "" {
			job.Format = &req.Format
		}
	}
	// Downloading a link is what the daily limit counts. Charging and creating
	// the job happen in one transaction, so a burst of requests can't each slip
	// under the limit. Uploaded files (splitting your own video) aren't limited.
	var createErr error
	if job.SourceType == store.SourceRemote {
		createErr = a.db.CreateJobCharged(r.Context(), job, a.chargeFor(ident))
	} else {
		createErr = a.db.CreateJob(r.Context(), job)
	}
	if errors.Is(createErr, store.ErrQuotaExceeded) {
		a.quotaExceeded(w, ident, 1, a.remainingFor(r.Context(), ident))
		return
	}
	if createErr != nil {
		a.log.Error("create job failed", "err", createErr)
		serverError(w)
		return
	}

	var (
		task  *asynq.Task
		qName string
		err   error
	)
	if job.SourceType == store.SourceUpload {
		task, err = queue.NewProcessTask(job.ID)
		qName = queue.QueueProcess
	} else {
		task, err = queue.NewDownloadTask(job.ID)
		qName = queue.QueueDownload
	}
	if err != nil {
		serverError(w)
		return
	}
	if _, err := a.q.EnqueueContext(r.Context(), task,
		asynq.Queue(qName),
		asynq.MaxRetry(2),
		asynq.Timeout(a.cfg.JobTimeout),
		asynq.Retention(24*time.Hour),
	); err != nil {
		a.log.Error("enqueue failed", "err", err, "job", job.ID)
		serverError(w)
		return
	}

	a.log.Info("job enqueued to redis queue", "job", job.ID, "queue", qName, "source_type", job.SourceType, "audio_only", job.AudioOnly)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     job.ID,
		"status": store.StatusQueued,
	})
}

func (a *API) listJobs(w http.ResponseWriter, r *http.Request) {
	ident := a.identify(r, "")
	var owners []string
	for _, o := range []string{ident.OwnerID, ident.DeviceID} {
		if o != "" {
			owners = append(owners, o)
		}
	}
	if len(owners) == 0 {
		badRequest(w, "missing user_id or X-Device-ID header")
		return
	}

	jobs, err := a.db.ListUserJobsFor(r.Context(), owners, 30)
	if err != nil {
		a.log.Error("list user jobs failed", "err", err, "user_id", ident.OwnerID)
		serverError(w)
		return
	}

	for i := range jobs {
		hideInternalError(&jobs[i])
		for j := range jobs[i].Clips {
			url, err := a.files.PresignGet(r.Context(), jobs[i].Clips[j].ObjectKey, a.cfg.OutputTTL)
			if err != nil {
				continue
			}
			jobs[i].Clips[j].URL = url
		}
		if jobs[i].Clips == nil {
			jobs[i].Clips = []store.Clip{}
		}
	}
	if jobs == nil {
		jobs = []store.Job{}
	}

	writeJSON(w, http.StatusOK, jobs)
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		badRequest(w, "invalid job id")
		return
	}

	job, err := a.db.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if err != nil {
		a.log.Error("get job failed", "err", err)
		serverError(w)
		return
	}
	hideInternalError(&job)

	// Sign clip URLs at read time so they expire on their own.
	for i := range job.Clips {
		url, err := a.files.PresignGet(r.Context(), job.Clips[i].ObjectKey, a.cfg.OutputTTL)
		if err != nil {
			a.log.Error("presign get failed", "err", err)
			continue
		}
		job.Clips[i].URL = url
	}
	if job.Clips == nil {
		job.Clips = []store.Clip{}
	}

	writeJSON(w, http.StatusOK, job)
}

// publicConfig tells clients the limits they can check up front, so a person
// isn't told "too large" only after waiting for an upload to finish. It needs
// nothing from the database, and holds nothing secret.
func (a *API) publicConfig(w http.ResponseWriter, r *http.Request) {
	exts := make([]string, 0, len(allowedExt))
	for e := range allowedExt {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	writeJSON(w, http.StatusOK, map[string]any{
		"max_upload_bytes":          a.cfg.MaxUploadBytes,
		"allowed_upload_extensions": exts,
		"aspects":                   aspectNames(),
	})
}

// publicError is what a caller may be told about a failed job. The stored text
// is raw tool output, which can carry server paths and proxy addresses, so
// clients get a fixed sentence per error code and the detail stays in the logs.
func publicError(code string) string {
	switch code {
	case "unavailable":
		return "this video is private, removed, or not available in this region"
	case "blocked":
		return "the site refused the request"
	case "extractor_stale":
		return "this site isn't working right now"
	case "unsupported":
		return "this site isn't supported"
	case "live_stream":
		return "live streams aren't supported"
	case "too_large":
		return "this video is too long or too large"
	default:
		return "we couldn't prepare this video"
	}
}

func hideInternalError(j *store.Job) {
	if j.Error == nil {
		return
	}
	msg := publicError(derefStr(j.ErrorCode))
	j.Error = &msg
}

// aspectNames are the frame shapes a job may ask for, in the order clients show them.
func aspectNames() []string {
	return []string{media.AspectVertical, media.AspectPortrait, media.AspectSquare, media.AspectHorizontal}
}

var allowedExt = map[string]bool{
	".mp4": true, ".mov": true, ".m4v": true, ".webm": true, ".mkv": true,
}

func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

// logging records each request with the caller's address as the API sees it,
// after trusted-proxy handling. That is the address limits are counted against,
// so it is what to check when confirming X-Forwarded-For is being honoured.
// The ResponseWriter is passed through untouched so WebSocket upgrades still work.
func (a *API) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.log.Info("request", "method", r.Method, "path", r.URL.Path, "ip", a.clientIP(r), "dur", time.Since(start))
	})
}
