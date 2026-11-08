package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ammyy9908/clipper/internal/media"
	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/store"
)

var errEmptyPlaylist = errors.New("playlist has no downloadable entries")

type playlistEntry struct {
	// Index is the 1-based position in the original playlist. Positions of
	// private or deleted videos are skipped, so indices can have gaps.
	Index           int     `json:"index"`
	Title           string  `json:"title"`
	URL             string  `json:"url"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Thumbnail       string  `json:"thumbnail,omitempty"`
}

type playlistInfo struct {
	// Count is how many downloadable entries are listed in Entries.
	Count int `json:"count"`
	// Total is the size the site reports for the whole playlist, when known.
	Total int `json:"total,omitempty"`
	// Truncated means the playlist is longer than the entries returned here.
	Truncated bool            `json:"truncated"`
	Entries   []playlistEntry `json:"entries"`
}

// buildPlaylistResp shapes a flat yt-dlp listing for clients. Per-video formats
// aren't known without resolving every entry, so the quality choices are a
// fixed ladder; each video falls back to its best format at or below the choice.
func buildPlaylistResp(info resolver.Info) resolveResp {
	entries := info.PlaylistEntries()

	out := make([]playlistEntry, 0, len(entries))
	var total float64
	for _, e := range entries {
		total += e.Duration
		out = append(out, playlistEntry{
			Index:           e.Index,
			Title:           e.Title,
			URL:             e.URL,
			DurationSeconds: e.Duration,
			Thumbnail:       e.Thumbnail,
		})
	}

	title := info.Title
	if title == "" {
		title = "Playlist"
	}
	uploader := info.Uploader
	if uploader == "" {
		uploader = info.Channel
	}
	thumb := info.Thumbnail
	if thumb == "" && len(out) > 0 {
		thumb = out[0].Thumbnail
	}

	return resolveResp{
		Kind:      "playlist",
		Title:     title,
		Uploader:  uploader,
		Platform:  info.Extractor,
		Duration:  total,
		Thumbnail: thumb,
		Variants:  playlistVariants(),
		Playlist: &playlistInfo{
			Count:     len(out),
			Total:     info.PlaylistCount,
			Truncated: info.PlaylistCount > len(info.Entries),
			Entries:   out,
		},
	}
}

func playlistVariants() []variant {
	out := make([]variant, 0, 5)
	for _, h := range []int{1080, 720, 480, 360} {
		out = append(out, variant{
			Selector: videoSelector(h),
			Label:    itoa(h) + "p",
			Height:   h,
			Ext:      "mp4",
		})
	}
	return append(out, variant{Selector: "bestaudio/best", Label: "MP3", Ext: "mp3", Audio: true})
}

// looksLikePlaylistURL is a cheap check for links that are unambiguously a
// playlist, used only to keep the single-video oEmbed fallback away from them.
func looksLikePlaylistURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.HasSuffix(host, "youtube.com"):
		return u.Path == "/playlist"
	case strings.HasSuffix(host, "soundcloud.com"):
		return strings.Contains(u.Path, "/sets/")
	}
	return false
}

// selectEntries picks the entries whose positions were requested. No positions
// means every entry. Results keep playlist order whatever order was asked for,
// and repeats collapse, so a client can't fan one video out into many jobs.
func selectEntries(entries []playlistEntry, items []int) ([]playlistEntry, error) {
	if len(items) == 0 {
		return entries, nil
	}
	want := make(map[int]bool, len(items))
	for _, n := range items {
		want[n] = true
	}
	out := make([]playlistEntry, 0, len(want))
	for _, e := range entries {
		if want[e.Index] {
			out = append(out, e)
			delete(want, e.Index)
		}
	}
	if len(want) > 0 {
		missing := make([]int, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		sort.Ints(missing)
		return nil, fmt.Errorf("item %d is not a downloadable video in this playlist", missing[0])
	}
	return out, nil
}

type createPlaylistReq struct {
	URL       string `json:"url"`
	Format    string `json:"format"`     // variant selector from /v1/resolve
	AudioOnly bool   `json:"audio_only"` // save every video as mp3
	Items     []int  `json:"items"`      // 1-based positions from /v1/resolve; empty = all
	UserID    string `json:"user_id"`
}

// createPlaylist fans a playlist out into one ordinary download job per video.
// Everything after this (downloading, processing, clips, expiry) is the same
// pipeline a single link uses, so workers need no playlist awareness.
func (a *API) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var req createPlaylistReq
	if err := decode(r, &req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	req.URL = normalizeURL(req.URL)
	if err := validateURL(req.URL); err != nil {
		badRequest(w, err.Error())
		return
	}
	if !validSelector(req.Format) {
		badRequest(w, "unsupported format")
		return
	}
	if len(req.Items) > 500 {
		badRequest(w, "too many items")
		return
	}

	resp, err := a.resolveCached(r.Context(), req.URL)
	if err != nil {
		a.resolverError(w, err, req.URL)
		return
	}
	if resp.Playlist == nil {
		badRequest(w, "this link is a single video; create a job for it instead")
		return
	}

	chosen, err := selectEntries(resp.Playlist.Entries, req.Items)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	if len(chosen) == 0 {
		badRequest(w, "no videos selected")
		return
	}
	if max := a.cfg.PlaylistMaxItems; max > 0 && len(chosen) > max {
		badRequest(w, fmt.Sprintf("select at most %d videos at once", max))
		return
	}

	ident := a.identify(r, req.UserID)
	userID := ident.OwnerID
	var format *string
	if req.Format != "" {
		format = &req.Format
	}

	pl := store.Playlist{ID: uuid.New(), SourceURL: req.URL, Title: resp.Title, Total: len(chosen)}
	if userID != "" {
		pl.UserID = &userID
	}

	jobs := make([]store.PlaylistJob, 0, len(chosen))
	for _, e := range chosen {
		entryURL := normalizeURL(e.URL)
		if validateURL(entryURL) != nil {
			continue
		}
		title := e.Title
		j := store.Job{
			ID:         uuid.New(),
			SourceType: store.SourceRemote,
			SourceURL:  &entryURL,
			Format:     format,
			AudioOnly:  req.AudioOnly,
			Mode:       media.ModeCopy,
			Status:     store.StatusQueued,
			Title:      &title,
		}
		jobs = append(jobs, store.PlaylistJob{Job: j, Position: e.Index})
	}
	if len(jobs) == 0 {
		badRequest(w, "no downloadable videos in the selection")
		return
	}
	pl.Total = len(jobs)

	// Each video is a download, so the whole playlist is charged up front: it
	// fits in what's left of today's allowance or nothing is created.
	charge := a.chargeFor(ident)
	err = a.db.CreatePlaylist(r.Context(), pl, jobs, &charge)
	if errors.Is(err, store.ErrQuotaExceeded) {
		a.quotaExceeded(w, ident, len(jobs), a.remainingFor(r.Context(), ident))
		return
	}
	if err != nil {
		a.log.Error("create playlist failed", "err", err)
		serverError(w)
		return
	}

	queued := 0
	for _, pj := range jobs {
		if err := a.enqueueDownload(r.Context(), pj.Job.ID); err != nil {
			// Keep going: one bad enqueue shouldn't strand the rest. The job is
			// marked failed so the playlist can still settle instead of waiting forever.
			a.log.Error("enqueue playlist job failed", "err", err, "job", pj.Job.ID, "playlist", pl.ID)
			msg, code := "could not queue this video", "enqueue_failed"
			_ = a.db.SetStatus(context.WithoutCancel(r.Context()), pj.Job.ID, store.StatusFailed, &msg, &code)
			continue
		}
		queued++
	}
	a.log.Info("playlist enqueued", "playlist", pl.ID, "videos", len(jobs), "queued", queued, "audio_only", req.AudioOnly)

	if queued == 0 {
		// Nothing will ever run. Say so instead of handing back an id that only
		// ever reports failure.
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "couldn't queue these downloads right now, try again shortly",
			"code":  "queue_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     pl.ID,
		"title":  pl.Title,
		"total":  pl.Total,
		"queued": queued,
		"status": store.StatusQueued,
	})
}

func (a *API) enqueueDownload(ctx context.Context, id uuid.UUID) error {
	task, err := queue.NewDownloadTask(id)
	if err != nil {
		return err
	}
	_, err = a.q.EnqueueContext(ctx, task,
		asynq.Queue(queue.QueueDownload),
		asynq.MaxRetry(2),
		asynq.Timeout(a.cfg.JobTimeout),
		asynq.Retention(24*time.Hour),
	)
	return err
}

type playlistItemView struct {
	Position int `json:"position"`
	store.Job
}

type playlistView struct {
	ID        uuid.UUID          `json:"id"`
	Title     string             `json:"title"`
	SourceURL string             `json:"source_url"`
	Status    string             `json:"status"`
	Total     int                `json:"total"`
	Completed int                `json:"completed"`
	Failed    int                `json:"failed"`
	CreatedAt time.Time          `json:"created_at"`
	Items     []playlistItemView `json:"items"`
}

// summarizePlaylist folds the per-video statuses into one. It settles as
// "completed" once nothing is pending and at least one video succeeded, so a
// few unavailable videos don't hold a whole playlist in limbo, and as "failed"
// only when every video failed.
func summarizePlaylist(jobs []store.PlaylistJob) (status string, completed, failed int) {
	pending, started := 0, false
	for _, pj := range jobs {
		switch pj.Job.Status {
		case store.StatusCompleted:
			completed++
		case store.StatusFailed:
			failed++
		default:
			pending++
			if pj.Job.Status != store.StatusQueued {
				started = true
			}
		}
	}
	switch {
	case pending > 0 && !started && completed == 0 && failed == 0:
		return store.StatusQueued, completed, failed
	case pending > 0:
		return store.StatusProcessing, completed, failed
	case completed == 0:
		return store.StatusFailed, completed, failed
	default:
		return store.StatusCompleted, completed, failed
	}
}

func (a *API) getPlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		badRequest(w, "invalid playlist id")
		return
	}

	pl, jobs, err := a.db.GetPlaylist(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "playlist not found"})
		return
	}
	if err != nil {
		a.log.Error("get playlist failed", "err", err)
		serverError(w)
		return
	}

	items := make([]playlistItemView, 0, len(jobs))
	for _, pj := range jobs {
		j := pj.Job
		hideInternalError(&j)
		// Sign clip URLs at read time so they expire on their own.
		for i := range j.Clips {
			u, err := a.files.PresignGet(r.Context(), j.Clips[i].ObjectKey, a.cfg.OutputTTL)
			if err != nil {
				a.log.Error("presign get failed", "err", err, "job", j.ID)
				continue
			}
			j.Clips[i].URL = u
		}
		if j.Clips == nil {
			j.Clips = []store.Clip{}
		}
		items = append(items, playlistItemView{Position: pj.Position, Job: j})
	}

	status, completed, failed := summarizePlaylist(jobs)
	writeJSON(w, http.StatusOK, playlistView{
		ID:        pl.ID,
		Title:     pl.Title,
		SourceURL: pl.SourceURL,
		Status:    status,
		Total:     pl.Total,
		Completed: completed,
		Failed:    failed,
		CreatedAt: pl.CreatedAt,
		Items:     items,
	})
}

// requestUserID reads the caller's identity the same way job creation does:
// body first, then X-User-ID, X-Device-ID, and finally ?user_id=.
func requestUserID(r *http.Request, fromBody string) string {
	for _, v := range []string{fromBody, r.Header.Get("X-User-ID"), r.Header.Get("X-Device-ID"), r.URL.Query().Get("user_id")} {
		if v != "" {
			return v
		}
	}
	return ""
}
