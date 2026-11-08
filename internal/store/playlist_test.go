package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Runs against a real Postgres with migrations/*.sql applied:
//
//	TEST_DATABASE_URL=postgres://... go test ./internal/store
//
// Skipped otherwise, so plain `go test ./...` stays hermetic.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func remoteJob(url, title string) Job {
	return Job{ID: uuid.New(), SourceType: SourceRemote, SourceURL: &url, Title: &title, Mode: "copy"}
}

func TestPlaylistRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	user := "test-" + uuid.NewString()
	p := Playlist{ID: uuid.New(), UserID: &user, SourceURL: "https://example.com/pl", Title: "Mix", Total: 3}
	// Positions have a gap (2 is a private video) and are inserted out of order.
	jobs := []PlaylistJob{
		{Job: remoteJob("https://example.com/c", "third"), Position: 5},
		{Job: remoteJob("https://example.com/a", "first"), Position: 1},
		{Job: remoteJob("https://example.com/b", "second"), Position: 3},
	}
	if err := s.CreatePlaylist(ctx, p, jobs, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, p.ID) })

	// Finish one job with two clips so clip loading is exercised.
	done := jobs[1].Job.ID // position 1
	if err := s.Complete(ctx, done, 5000, []Clip{
		{Index: 0, ObjectKey: "outputs/a-0.mp4", DurationMS: 2500, SizeBytes: 10},
		{Index: 1, ObjectKey: "outputs/a-1.mp4", DurationMS: 2500, SizeBytes: 20},
	}); err != nil {
		t.Fatal(err)
	}

	got, items, err := s.GetPlaylist(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Mix" || got.Total != 3 || got.UserID == nil || *got.UserID != user {
		t.Errorf("playlist row wrong: %+v", got)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	wantOrder := []int{1, 3, 5}
	for i, it := range items {
		if it.Position != wantOrder[i] {
			t.Errorf("item %d position = %d, want %d (must be ordered by playlist position)", i, it.Position, wantOrder[i])
		}
	}
	if items[0].Job.Status != StatusCompleted || len(items[0].Job.Clips) != 2 || items[0].Job.Clips[1].Index != 1 {
		t.Errorf("completed job should carry its ordered clips: %+v", items[0].Job)
	}
	if items[1].Job.Status != StatusQueued || len(items[1].Job.Clips) != 0 {
		t.Errorf("queued job should have no clips: %+v", items[1].Job)
	}
	if items[1].Job.Title == nil || *items[1].Job.Title != "second" {
		t.Errorf("preset title should be stored: %+v", items[1].Job.Title)
	}

	// Playlist children are ordinary jobs, so they show up in the user's history.
	history, err := s.ListUserJobs(ctx, user, 10)
	if err != nil || len(history) != 3 {
		t.Errorf("children should appear in ListUserJobs: %d %v", len(history), err)
	}
}

func TestCreatePlaylistIsAtomic(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	dup := remoteJob("https://example.com/a", "dup")
	p := Playlist{ID: uuid.New(), SourceURL: "https://example.com/pl", Title: "Broken", Total: 2}
	// The same job id twice: the second insert violates the primary key.
	err := s.CreatePlaylist(ctx, p, []PlaylistJob{{Job: dup, Position: 1}, {Job: dup, Position: 2}}, nil)
	if err == nil {
		t.Fatal("expected the duplicate job id to fail")
	}
	if _, _, err := s.GetPlaylist(ctx, p.ID); err != ErrNotFound {
		t.Errorf("a failed create must leave no playlist behind, got %v", err)
	}
	var n int
	s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE id = $1`, dup.ID).Scan(&n)
	if n != 0 {
		t.Errorf("a failed create must leave no jobs behind, found %d", n)
	}
}

func TestGetPlaylistNotFound(t *testing.T) {
	s := testStore(t)
	if _, _, err := s.GetPlaylist(context.Background(), uuid.New()); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPruneExpiredRemovesPlaylistJobsAndClips(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	p := Playlist{ID: uuid.New(), SourceURL: "https://example.com/pl", Title: "Old", Total: 1}
	j := remoteJob("https://example.com/a", "old")
	if err := s.CreatePlaylist(ctx, p, []PlaylistJob{{Job: j, Position: 1}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Complete(ctx, j.ID, 1000, []Clip{{Index: 0, ObjectKey: "outputs/x.mp4", DurationMS: 1000, SizeBytes: 1}}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	s.pool.Exec(ctx, `UPDATE playlists SET created_at = $2 WHERE id = $1`, p.ID, old)
	s.pool.Exec(ctx, `UPDATE jobs SET created_at = $2 WHERE id = $1`, j.ID, old)

	if _, err := s.PruneExpired(ctx, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	var jobs, clips int
	s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE id = $1`, j.ID).Scan(&jobs)
	s.pool.QueryRow(ctx, `SELECT count(*) FROM clips WHERE job_id = $1`, j.ID).Scan(&clips)
	if _, _, err := s.GetPlaylist(ctx, p.ID); err != ErrNotFound || jobs != 0 || clips != 0 {
		t.Errorf("expired playlist should be gone with its jobs and clips: err=%v jobs=%d clips=%d", err, jobs, clips)
	}
}

// EnsureSchema runs on every Open, so it must be safe to repeat.
func TestEnsureSchemaIsIdempotent(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 2; i++ {
		if err := s.EnsureSchema(context.Background()); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}

func TestCancelUnfinishedNeverOverwritesFinishedJobs(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	stuck := remoteJob("https://example.com/stuck", "stuck")
	done := remoteJob("https://example.com/done", "done")
	for _, j := range []Job{stuck, done} {
		if err := s.CreateJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{stuck.ID, done.ID}) })
	// "done" finishes between being listed and being cancelled.
	if err := s.Complete(ctx, done.ID, 1000, nil); err != nil {
		t.Fatal(err)
	}
	s.pool.Exec(ctx, `UPDATE jobs SET updated_at = now() - interval '2 hours' WHERE id = $1`, stuck.ID)

	listed, err := s.ListUnfinished(ctx, 30*time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, j := range listed {
		found = found || j.ID == stuck.ID
		if j.ID == done.ID {
			t.Error("a completed job must never be listed as unfinished")
		}
	}
	if !found {
		t.Error("the idle queued job should be listed")
	}

	n, err := s.CancelUnfinished(ctx, []uuid.UUID{stuck.ID, done.ID})
	if err != nil || n != 1 {
		t.Fatalf("only the still-unfinished job may be cancelled: n=%d err=%v", n, err)
	}
	statuses, _ := s.JobStatuses(ctx, []uuid.UUID{stuck.ID, done.ID, uuid.New()})
	if statuses[stuck.ID] != StatusFailed || statuses[done.ID] != StatusCompleted || len(statuses) != 2 {
		t.Errorf("statuses wrong: %v", statuses)
	}
}
