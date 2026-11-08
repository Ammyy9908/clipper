package store

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

const today, tomorrow = "2026-09-20", "2026-09-21"

func bucket(limit int) QuotaKey { return QuotaKey{Key: "test:" + uuid.NewString(), Limit: limit} }

func charge(day string, keys ...QuotaKey) QuotaCharge { return QuotaCharge{Day: day, Keys: keys} }

func used(t *testing.T, s *Store, day string, k QuotaKey) int {
	t.Helper()
	left, err := s.Remaining(context.Background(), day, []QuotaKey{k})
	if err != nil {
		t.Fatal(err)
	}
	return k.Limit - left
}

func TestLimitIsEnforcedAndIsPerDay(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(3)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	for i := 1; i <= 3; i++ {
		if err := s.CreateJobCharged(ctx, remoteJob("https://example.com/"+uuid.NewString(), "j"), charge(today, k)); err != nil {
			t.Fatalf("download %d of 3 should be allowed: %v", i, err)
		}
	}
	err := s.CreateJobCharged(ctx, remoteJob("https://example.com/x", "over"), charge(today, k))
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("the 4th download must be refused, got %v", err)
	}
	if got := used(t, s, today, k); got != 3 {
		t.Errorf("a refused download must not be counted: used=%d", got)
	}

	// The next day is a fresh allowance.
	if err := s.CreateJobCharged(ctx, remoteJob("https://example.com/tomorrow", "t"), charge(tomorrow, k)); err != nil {
		t.Fatalf("a new day starts fresh: %v", err)
	}
}

func TestRefusedDownloadCreatesNoJob(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(1)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	first := remoteJob("https://example.com/first", "first")
	second := remoteJob("https://example.com/second", "second")
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{first.ID, second.ID}) })

	if err := s.CreateJobCharged(ctx, first, charge(today, k)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateJobCharged(ctx, second, charge(today, k)); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("want ErrQuotaExceeded, got %v", err)
	}
	var n int
	s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE id = $1`, second.ID).Scan(&n)
	if n != 0 {
		t.Error("a job refused by the limit must not exist")
	}
}

func TestAnonymousMustFitInBothBuckets(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	device, ip := bucket(3), bucket(1) // this IP has used its one
	t.Cleanup(func() {
		s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = ANY($1)`, []string{device.Key, ip.Key})
	})

	if err := s.CreateJobCharged(ctx, remoteJob("https://example.com/a", "a"), charge(today, device, ip)); err != nil {
		t.Fatal(err)
	}
	// The device still has room, but the IP is full: refused, and the device is not charged.
	err := s.CreateJobCharged(ctx, remoteJob("https://example.com/b", "b"), charge(today, device, ip))
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("a full IP bucket must refuse the download: %v", err)
	}
	if got := used(t, s, today, device); got != 1 {
		t.Errorf("the device bucket must not be charged when the IP bucket refuses: used=%d", got)
	}
}

func TestConcurrentRequestsCannotExceedTheLimit(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(5)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	var ok, refused atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.CreateJobCharged(ctx, remoteJob("https://example.com/"+uuid.NewString(), "race"), charge(today, k))
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrQuotaExceeded):
				refused.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 5 || refused.Load() != 25 {
		t.Fatalf("30 simultaneous requests against a limit of 5: %d allowed, %d refused", ok.Load(), refused.Load())
	}
	if got := used(t, s, today, k); got != 5 {
		t.Errorf("usage should be exactly 5, got %d", got)
	}
}

func TestFailedJobsAreRefundedOnceAndFinishedOnesAreNot(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(5)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	failing, succeeding := remoteJob("https://example.com/f", "f"), remoteJob("https://example.com/s", "s")
	for _, j := range []Job{failing, succeeding} {
		if err := s.CreateJobCharged(ctx, j, charge(today, k)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{failing.ID, succeeding.ID})
	})
	if got := used(t, s, today, k); got != 2 {
		t.Fatalf("setup: used=%d", got)
	}

	msg, code := "private video", "unavailable"
	for i := 0; i < 3; i++ { // retries call SetStatus(failed) again; it must refund only once
		if err := s.SetStatus(ctx, failing.ID, StatusFailed, &msg, &code); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Complete(ctx, succeeding.ID, 1000, nil); err != nil {
		t.Fatal(err)
	}
	if got := used(t, s, today, k); got != 1 {
		t.Errorf("only the failed download is given back, once: used=%d want 1", got)
	}
}

func TestPurgeRefundsOnlyWhatItCancelled(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(5)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	stuck, done := remoteJob("https://example.com/stuck", "s"), remoteJob("https://example.com/done", "d")
	for _, j := range []Job{stuck, done} {
		if err := s.CreateJobCharged(ctx, j, charge(today, k)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{stuck.ID, done.ID}) })
	if err := s.Complete(ctx, done.ID, 1000, nil); err != nil { // finished after being listed
		t.Fatal(err)
	}

	if n, err := s.CancelUnfinished(ctx, []uuid.UUID{stuck.ID, done.ID}); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := used(t, s, today, k); got != 1 {
		t.Errorf("cancelling refunds the cancelled job only, not the one that completed: used=%d want 1", got)
	}
}

func TestPlaylistIsChargedAllOrNothing(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(3)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	mk := func(n int) (Playlist, []PlaylistJob) {
		p := Playlist{ID: uuid.New(), SourceURL: "https://example.com/pl", Title: "pl", Total: n}
		var jobs []PlaylistJob
		for i := 1; i <= n; i++ {
			jobs = append(jobs, PlaylistJob{Job: remoteJob("https://example.com/"+uuid.NewString(), "v"), Position: i})
		}
		return p, jobs
	}
	c := charge(today, k)

	big, bigJobs := mk(4)
	if err := s.CreatePlaylist(ctx, big, bigJobs, &c); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("a 4 video playlist doesn't fit a limit of 3: %v", err)
	}
	if _, _, err := s.GetPlaylist(ctx, big.ID); err != ErrNotFound {
		t.Errorf("a refused playlist must not exist: %v", err)
	}
	if got := used(t, s, today, k); got != 0 {
		t.Errorf("a refused playlist must not use any allowance: used=%d", got)
	}

	fits, fitsJobs := mk(3)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, fits.ID) })
	if err := s.CreatePlaylist(ctx, fits, fitsJobs, &c); err != nil {
		t.Fatal(err)
	}
	if got := used(t, s, today, k); got != 3 {
		t.Errorf("each video in the playlist counts: used=%d want 3", got)
	}
}

func TestUnlimitedBucketsAreNotCounted(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(0) // limit 0 = unlimited
	if err := s.CreateJobCharged(ctx, remoteJob("https://example.com/u", "u"), charge(today, k)); err != nil {
		t.Fatal(err)
	}
	left, err := s.Remaining(ctx, today, []QuotaKey{k})
	if err != nil || left != -1 {
		t.Errorf("an unlimited bucket reports -1 remaining: %d %v", left, err)
	}
}

func TestSignedInSeesDeviceHistoryToo(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	device, account := "dev-"+uuid.NewString(), "google:"+uuid.NewString()

	before, after := remoteJob("https://example.com/before", "before"), remoteJob("https://example.com/after", "after")
	before.UserID, after.UserID = &device, &account
	for _, j := range []Job{before, after} {
		if err := s.CreateJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{before.ID, after.ID}) })

	got, err := s.ListUserJobsFor(ctx, []string{account, device}, 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("signing in shouldn't hide what the device downloaded before: %d %v", len(got), err)
	}
}

func TestUpsertUser(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	id := "google:" + uuid.NewString()
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	for _, name := range []string{"First", "Second"} {
		if err := s.UpsertUser(ctx, User{ID: id, Email: "a@b.c", Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	var name string
	s.pool.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, id).Scan(&name)
	if name != "Second" {
		t.Errorf("a repeat login updates the profile: %q", name)
	}
}

// Regression: refunding several jobs that share a bucket in one batch used to
// give back only one, because UPDATE ... FROM applies a single join row per target row.
func TestBatchRefundCountsEveryJobInTheBucket(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	k := bucket(10)
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM download_usage WHERE key = $1`, k.Key) })

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		j := remoteJob("https://example.com/"+uuid.NewString(), "batch")
		if err := s.CreateJobCharged(ctx, j, charge(today, k)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, j.ID)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM jobs WHERE id = ANY($1)`, ids) })
	if got := used(t, s, today, k); got != 5 {
		t.Fatalf("setup: used=%d", got)
	}

	if n, err := s.CancelUnfinished(ctx, ids); err != nil || n != 5 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := used(t, s, today, k); got != 0 {
		t.Errorf("all 5 cancelled jobs must be refunded: used=%d want 0", got)
	}
	// And a second pass must not refund again or go below zero.
	if _, err := s.CancelUnfinished(ctx, ids); err != nil {
		t.Fatal(err)
	}
	if got := used(t, s, today, k); got != 0 {
		t.Errorf("refunds are once-only: used=%d", got)
	}
}
