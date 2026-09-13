package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/media"
	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/storage"
	"github.com/ammyy9908/clipper/internal/store"
)

type API struct {
	cfg   config.Config
	db    *store.Store
	q     *asynq.Client
	files *storage.Store
	res   *resolver.Resolver
	log   *slog.Logger
}

func New(cfg config.Config, db *store.Store, q *asynq.Client, files *storage.Store, res *resolver.Resolver, log *slog.Logger) *API {
	return &API{cfg: cfg, db: db, q: q, files: files, res: res, log: log}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /v1/resolve", a.resolve)
	mux.HandleFunc("POST /v1/uploads", a.createUpload)
	mux.HandleFunc("POST /v1/jobs", a.createJob)
	mux.HandleFunc("GET /v1/jobs/{id}", a.getJob)
	return logging(a.log, mux)
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
	ObjectKey      string `json:"object_key"`      // from POST /v1/uploads
	URL            string `json:"url"`             // or a remote link
	Format         string `json:"format"`          // variant selector from /v1/resolve
	SegmentSeconds int    `json:"segment_seconds"` // 0 = download only, no split
	Mode           string `json:"mode"`
	Aspect         string `json:"aspect"`
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

	if !media.ValidAspect(req.Aspect) {
		badRequest(w, "aspect must be omitted, '9:16', or '1:1'")
		return
	}

	job := store.Job{
		ID:             uuid.New(),
		SegmentSeconds: req.SegmentSeconds,
		Aspect:         req.Aspect,
		Mode:           req.Mode,
		Status:         store.StatusQueued,
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
		if err := validateURL(req.URL); err != nil {
			badRequest(w, err.Error())
			return
		}
		job.SourceType = store.SourceRemote
		job.SourceURL = &req.URL
		if req.Format != "" {
			job.Format = &req.Format
		}
	}
	if err := a.db.CreateJob(r.Context(), job); err != nil {
		a.log.Error("create job failed", "err", err)
		serverError(w)
		return
	}

	task, err := queue.NewSplitTask(job.ID)
	if err != nil {
		serverError(w)
		return
	}
	if _, err := a.q.EnqueueContext(r.Context(), task,
		asynq.MaxRetry(2),
		asynq.Timeout(a.cfg.JobTimeout),
		asynq.Retention(24*time.Hour),
	); err != nil {
		a.log.Error("enqueue failed", "err", err, "job", job.ID)
		serverError(w)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     job.ID,
		"status": store.StatusQueued,
	})
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

func logging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
