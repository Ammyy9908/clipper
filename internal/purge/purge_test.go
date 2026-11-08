package purge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ammyy9908/clipper/internal/store"
)

type fakeQueues struct {
	waiting             map[string][]TaskRef
	active              map[string][]TaskRef
	completed, archived int
	failDelete          map[string]bool // task id -> Delete errors

	deleted []string
	cleared []string
}

func (f *fakeQueues) Names() ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string][]TaskRef{f.waiting, f.active} {
		for q := range m {
			if !seen[q] {
				seen[q] = true
				out = append(out, q)
			}
		}
	}
	return out, nil
}
func (f *fakeQueues) Waiting(q string) ([]TaskRef, error) { return f.waiting[q], nil }
func (f *fakeQueues) Active(q string) ([]TaskRef, error)  { return f.active[q], nil }
func (f *fakeQueues) Delete(q, id string) error {
	if f.failDelete[id] {
		return errors.New("task is active")
	}
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeQueues) Finished(string) (int, int, error) { return f.completed, f.archived, nil }
func (f *fakeQueues) ClearFinished(q string) error      { f.cleared = append(f.cleared, q); return nil }

type fakeJobs struct {
	jobs      map[uuid.UUID]store.UnfinishedJob // every job, any status
	status    map[uuid.UUID]string
	cancelled []uuid.UUID
}

func (f *fakeJobs) ListUnfinished(_ context.Context, olderThan time.Duration, userID string) ([]store.UnfinishedJob, error) {
	var out []store.UnfinishedJob
	for id, j := range f.jobs {
		s := f.status[id]
		if s != "queued" && s != "downloading" && s != "processing" {
			continue
		}
		if time.Since(j.UpdatedAt) < olderThan {
			continue
		}
		if userID != "" && (j.UserID == nil || *j.UserID != userID) {
			continue
		}
		j.Status = s
		out = append(out, j)
	}
	return out, nil
}
func (f *fakeJobs) JobStatuses(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if s, ok := f.status[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}
func (f *fakeJobs) CancelUnfinished(_ context.Context, ids []uuid.UUID) (int64, error) {
	f.cancelled = append(f.cancelled, ids...)
	return int64(len(ids)), nil
}

type world struct {
	q *fakeQueues
	j *fakeJobs
}

func newWorld() *world {
	return &world{
		q: &fakeQueues{waiting: map[string][]TaskRef{}, active: map[string][]TaskRef{}, failDelete: map[string]bool{}},
		j: &fakeJobs{jobs: map[uuid.UUID]store.UnfinishedJob{}, status: map[uuid.UUID]string{}},
	}
}

// job registers a job with the given status and idle time and returns its id.
func (w *world) job(status string, idle time.Duration, user string) uuid.UUID {
	id := uuid.New()
	j := store.UnfinishedJob{ID: id, UpdatedAt: time.Now().Add(-idle)}
	if user != "" {
		j.UserID = &user
	}
	w.j.jobs[id] = j
	w.j.status[id] = status
	return id
}

func (w *world) waiting(queue, taskID string, job uuid.UUID) {
	w.q.waiting[queue] = append(w.q.waiting[queue], TaskRef{Queue: queue, ID: taskID, State: "pending", JobID: job})
}

func plan(t *testing.T, w *world, opt Options) Plan {
	t.Helper()
	p, err := BuildPlan(context.Background(), w.q, w.j, opt)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var thirty = Options{OlderThan: 30 * time.Minute}

func TestStuckJobAndItsTaskArePlanned(t *testing.T) {
	w := newWorld()
	stuck := w.job("queued", 2*time.Hour, "")
	w.waiting("downloads", "t1", stuck)

	p := plan(t, w, thirty)
	if len(p.Cancel) != 1 || p.Cancel[0].ID != stuck {
		t.Fatalf("stuck job should be cancelled: %+v", p.Cancel)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].ID != "t1" || p.Tasks[0].Reason != ReasonStuck {
		t.Fatalf("its task should be deleted as stuck: %+v", p.Tasks)
	}
}

func TestRecentWorkIsNeverTouched(t *testing.T) {
	w := newWorld()
	fresh := w.job("queued", 2*time.Minute, "")
	running := w.job("downloading", 5*time.Minute, "")
	w.waiting("downloads", "t1", fresh)
	w.waiting("downloads", "t2", running)

	p := plan(t, w, thirty)
	if !p.Empty() {
		t.Fatalf("recent, healthy work must not be planned for removal: %+v", p)
	}
}

func TestPlanningNeverMutates(t *testing.T) {
	w := newWorld()
	w.waiting("downloads", "t1", w.job("queued", 2*time.Hour, ""))
	w.q.completed, w.q.archived = 5, 2

	plan(t, w, Options{OlderThan: time.Minute, ClearFinished: true})
	if len(w.q.deleted) != 0 || len(w.j.cancelled) != 0 || len(w.q.cleared) != 0 {
		t.Fatalf("a dry run must not change anything: deleted=%v cancelled=%v cleared=%v", w.q.deleted, w.j.cancelled, w.q.cleared)
	}
}

func TestJobRunningRightNowIsSkippedEvenIfQuiet(t *testing.T) {
	w := newWorld()
	long := w.job("downloading", 3*time.Hour, "") // a long download, quiet but healthy
	w.q.active["downloads"] = []TaskRef{{Queue: "downloads", ID: "a1", State: "active", JobID: long}}

	p := plan(t, w, thirty)
	if len(p.Cancel) != 0 {
		t.Fatalf("a job with an active task must not be cancelled: %+v", p.Cancel)
	}
	if len(p.SkippedActive) != 1 || p.SkippedActive[0].ID != long {
		t.Fatalf("it should be reported as skipped: %+v", p.SkippedActive)
	}
}

func TestOrphanTasksAreRemoved(t *testing.T) {
	w := newWorld()
	done := w.job("completed", time.Hour, "")
	failed := w.job("failed", time.Hour, "")
	live := w.job("queued", time.Minute, "")
	gone := uuid.New() // no job row at all
	w.waiting("downloads", "done", done)
	w.waiting("downloads", "failed", failed)
	w.waiting("downloads", "live", live)
	w.waiting("processing", "gone", gone)

	p := plan(t, w, thirty)
	got := map[string]string{}
	for _, task := range p.Tasks {
		got[task.ID] = task.Reason
	}
	if got["done"] != ReasonFinished || got["failed"] != ReasonFinished || got["gone"] != ReasonMissing {
		t.Errorf("orphans should be removed with the right reason: %v", got)
	}
	if _, ok := got["live"]; ok {
		t.Errorf("a task for a live job must be kept: %v", got)
	}
}

func TestUserFilterOnlyTouchesThatUser(t *testing.T) {
	w := newWorld()
	mine := w.job("queued", time.Hour, "me")
	theirs := w.job("queued", time.Hour, "them")
	done := w.job("completed", time.Hour, "them")
	w.waiting("downloads", "mine", mine)
	w.waiting("downloads", "theirs", theirs)
	w.waiting("downloads", "orphan", done)

	p := plan(t, w, Options{OlderThan: 30 * time.Minute, UserID: "me"})
	if len(p.Cancel) != 1 || p.Cancel[0].ID != mine {
		t.Fatalf("only my job should be cancelled: %+v", p.Cancel)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].ID != "mine" {
		t.Fatalf("only my task should be deleted, and no orphan cleanup: %+v", p.Tasks)
	}
}

func TestUnreadablePayloadIsReportedNotDeleted(t *testing.T) {
	w := newWorld()
	w.waiting("downloads", "weird", uuid.UUID{})
	p := plan(t, w, thirty)
	if p.Unreadable != 1 || len(p.Tasks) != 0 {
		t.Fatalf("unreadable tasks must be left alone: %+v", p)
	}
}

func TestClearFinishedCountsOnlyWithoutUserFilter(t *testing.T) {
	w := newWorld()
	w.waiting("downloads", "x", uuid.New())
	w.q.completed, w.q.archived = 32, 3

	p := plan(t, w, Options{OlderThan: time.Hour, ClearFinished: true})
	if p.FinishedCompleted != 32 || p.FinishedArchived != 3 {
		t.Errorf("finished counts = %d/%d", p.FinishedCompleted, p.FinishedArchived)
	}
	p = plan(t, w, Options{OlderThan: time.Hour, ClearFinished: true, UserID: "me"})
	if p.FinishedCompleted != 0 {
		t.Errorf("history is shared, so a per-user purge must not clear it")
	}
}

func TestApplyDeletesTasksThenCancelsJobs(t *testing.T) {
	w := newWorld()
	stuck := w.job("queued", 2*time.Hour, "")
	orphan := w.job("completed", 2*time.Hour, "")
	w.waiting("downloads", "t1", stuck)
	w.waiting("downloads", "t2", orphan)
	opt := Options{OlderThan: 30 * time.Minute, ClearFinished: true}

	res, err := Apply(context.Background(), w.q, w.j, plan(t, w, opt), opt)
	if err != nil {
		t.Fatal(err)
	}
	if res.TasksDeleted != 2 || res.JobsCancelled != 1 || len(w.j.cancelled) != 1 || w.j.cancelled[0] != stuck {
		t.Fatalf("unexpected result %+v cancelled=%v", res, w.j.cancelled)
	}
	if len(w.q.cleared) == 0 {
		t.Error("finished history should have been cleared")
	}
}

func TestJobIsKeptWhenItsTaskCannotBeDeleted(t *testing.T) {
	w := newWorld()
	stuck := w.job("queued", 2*time.Hour, "")
	w.waiting("downloads", "t1", stuck)
	w.q.failDelete["t1"] = true // a worker grabbed it between plan and apply

	res, err := Apply(context.Background(), w.q, w.j, plan(t, w, thirty), thirty)
	if err != nil {
		t.Fatal(err)
	}
	if res.JobsCancelled != 0 || res.JobsKept != 1 || res.TasksSkipped != 1 || len(w.j.cancelled) != 0 {
		t.Fatalf("a job whose task couldn't be removed must not be cancelled: %+v", res)
	}
}
