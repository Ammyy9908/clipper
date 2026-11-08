package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origin connections (web & mobile apps)
	},
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// jobWebSocket handles ephemeral real-time WebSocket connections for a single job (GET /v1/jobs/{id}/ws).
// The connection streams live progress / state change events and automatically closes with code 1000
// (Normal Closure) as soon as the job reaches a terminal state (completed or failed).
func (a *API) jobWebSocket(w http.ResponseWriter, r *http.Request) {
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
		a.log.Error("get job failed for ws", "err", err, "job", id)
		serverError(w)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.Warn("ws upgrade failed", "err", err, "job", id)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read loop to process control frames (Ping/Pong/Close) and detect client disconnect
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Helper to send signed final job payload
	sendCompletedSnapshot := func(j store.Job) {
		for i := range j.Clips {
			url, err := a.files.PresignGet(ctx, j.Clips[i].ObjectKey, a.cfg.OutputTTL)
			if err == nil {
				j.Clips[i].URL = url
			}
		}
		if j.Clips == nil {
			j.Clips = []store.Clip{}
		}
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		_ = conn.WriteJSON(queue.JobEvent{
			ID:     j.ID,
			Status: j.Status,
			Stage:  "completed",
			Title:  derefStr(j.Title),
			Clips:  j.Clips,
		})
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "job completed"),
			time.Now().Add(writeWait),
		)
	}

	sendFailedSnapshot := func(j store.Job) {
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		_ = conn.WriteJSON(queue.JobEvent{
			ID:        j.ID,
			Status:    store.StatusFailed,
			Stage:     "failed",
			Error:     publicError(derefStr(j.ErrorCode)),
			ErrorCode: derefStr(j.ErrorCode),
			Title:     derefStr(j.Title),
		})
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "job failed"),
			time.Now().Add(writeWait),
		)
	}

	// If the job is already terminal at connection time, send snapshot and close immediately
	if job.Status == store.StatusCompleted {
		sendCompletedSnapshot(job)
		return
	}
	if job.Status == store.StatusFailed {
		sendFailedSnapshot(job)
		return
	}

	// Send current active state snapshot
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = conn.WriteJSON(queue.JobEvent{
		ID:     job.ID,
		Status: job.Status,
		Stage:  job.Status,
		Title:  derefStr(job.Title),
	})

	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// If Redis Pub/Sub is available, subscribe to the job topic
	if a.rdb != nil {
		pubsub := a.rdb.Subscribe(ctx, queue.JobChannel(id.String()))
		defer pubsub.Close()

		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				return

			case <-pingTicker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}

			case msg, ok := <-ch:
				if !ok {
					return
				}

				var ev queue.JobEvent
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					continue
				}

				if ev.Status == store.StatusCompleted {
					// Fetch fresh job to ensure all clips are presigned and complete
					if fresh, err := a.db.GetJob(ctx, id); err == nil {
						sendCompletedSnapshot(fresh)
					} else {
						// Fallback using clips in event
						for i := range ev.Clips {
							url, err := a.files.PresignGet(ctx, ev.Clips[i].ObjectKey, a.cfg.OutputTTL)
							if err == nil {
								ev.Clips[i].URL = url
							}
						}
						_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
						_ = conn.WriteJSON(ev)
						_ = conn.WriteControl(
							websocket.CloseMessage,
							websocket.FormatCloseMessage(websocket.CloseNormalClosure, "job completed"),
							time.Now().Add(writeWait),
						)
					}
					return
				}

				if ev.Status == store.StatusFailed {
					ev.Error = publicError(ev.ErrorCode)
					_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
					_ = conn.WriteJSON(ev)
					_ = conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, "job failed"),
						time.Now().Add(writeWait),
					)
					return
				}

				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteJSON(ev); err != nil {
					return
				}
			}
		}
	} else {
		// Fallback polling loop if Redis is not configured
		pollTicker := time.NewTicker(1 * time.Second)
		defer pollTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-pingTicker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}

			case <-pollTicker.C:
				fresh, err := a.db.GetJob(ctx, id)
				if err != nil {
					continue
				}
				if fresh.Status == store.StatusCompleted {
					sendCompletedSnapshot(fresh)
					return
				}
				if fresh.Status == store.StatusFailed {
					sendFailedSnapshot(fresh)
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				_ = conn.WriteJSON(queue.JobEvent{
					ID:     fresh.ID,
					Status: fresh.Status,
					Stage:  fresh.Status,
					Title:  derefStr(fresh.Title),
				})
			}
		}
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
