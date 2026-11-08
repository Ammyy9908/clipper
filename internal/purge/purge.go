// Package purge prunes stuck work out of the download queue.
//
// It is built around two rules. Nothing changes unless the caller applies the
// plan, so the default is to look. And it only ever removes things it can tie
// to a stuck or already-finished job, never "everything in Redis", so work that
// is genuinely in flight is left alone.
package purge

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ammyy9908/clipper/internal/store"
)

// TaskRef is one queued task and the job it belongs to. JobID is the zero UUID
// when the payload couldn't be read.
type TaskRef struct {
	Queue string
	ID    string
	State string
	JobID uuid.UUID
}

// Queues is the slice of the task queue this package needs.
type Queues interface {
	Names() ([]string, error)
	// Waiting lists tasks that haven't started: pending, scheduled and retrying.
	Waiting(queue string) ([]TaskRef, error)
	// Active lists tasks a worker is running right now.
	Active(queue string) ([]TaskRef, error)
	Delete(queue, taskID string) error
	// Finished counts tasks kept only as history: completed and archived.
	Finished(queue string) (completed, archived int, err error)
	ClearFinished(queue string) error
}

// Jobs is the slice of the job store this package needs.
type Jobs interface {
	ListUnfinished(ctx context.Context, olderThan time.Duration, userID string) ([]store.UnfinishedJob, error)
	JobStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
	CancelUnfinished(ctx context.Context, ids []uuid.UUID) (int64, error)
}

type Options struct {
	// OlderThan is how long a job must have been idle to count as stuck.
	OlderThan time.Duration
	// UserID limits the purge to one user or device. Orphan cleanup is skipped
	// in that case, since orphans have no owner to match.
	UserID string
	// ClearFinished also drops completed and archived task history.
	ClearFinished bool
}

const (
	ReasonStuck    = "job is stuck"
	ReasonFinished = "job already finished"
	ReasonMissing  = "job no longer exists"
)

type PlannedTask struct {
	TaskRef
	Reason string
}

type Plan struct {
	// Cancel are stuck jobs to mark failed.
	Cancel []store.UnfinishedJob
	// SkippedActive are stuck-looking jobs a worker is running right now. They
	// are left alone: a long download can be quiet for a while and still be healthy.
	SkippedActive []store.UnfinishedJob
	// Tasks are queue entries to delete.
	Tasks []PlannedTask
	// Unreadable counts waiting tasks whose payload couldn't be parsed. They are never touched.
	Unreadable int

	FinishedCompleted, FinishedArchived int
}

// Empty reports whether applying the plan would do nothing.
func (p Plan) Empty() bool {
	return len(p.Cancel) == 0 && len(p.Tasks) == 0 && p.FinishedCompleted == 0 && p.FinishedArchived == 0
}

// BuildPlan works out what a purge would do, without changing anything.
func BuildPlan(ctx context.Context, q Queues, j Jobs, opt Options) (Plan, error) {
	var plan Plan

	names, err := q.Names()
	if err != nil {
		return plan, fmt.Errorf("list queues: %w", err)
	}

	active := map[uuid.UUID]bool{}
	var waiting []TaskRef
	for _, name := range names {
		act, err := q.Active(name)
		if err != nil {
			return plan, fmt.Errorf("list active tasks in %s: %w", name, err)
		}
		for _, t := range act {
			active[t.JobID] = true
		}
		w, err := q.Waiting(name)
		if err != nil {
			return plan, fmt.Errorf("list waiting tasks in %s: %w", name, err)
		}
		waiting = append(waiting, w...)
	}

	stuck, err := j.ListUnfinished(ctx, opt.OlderThan, opt.UserID)
	if err != nil {
		return plan, fmt.Errorf("list unfinished jobs: %w", err)
	}
	cancel := map[uuid.UUID]bool{}
	for _, job := range stuck {
		if active[job.ID] {
			plan.SkippedActive = append(plan.SkippedActive, job)
			continue
		}
		plan.Cancel = append(plan.Cancel, job)
		cancel[job.ID] = true
	}

	// Waiting tasks that need a status lookup: not tied to a job being cancelled.
	var unknown []uuid.UUID
	for _, t := range waiting {
		if t.JobID == (uuid.UUID{}) {
			plan.Unreadable++
			continue
		}
		if cancel[t.JobID] {
			plan.Tasks = append(plan.Tasks, PlannedTask{TaskRef: t, Reason: ReasonStuck})
		} else if opt.UserID == "" {
			unknown = append(unknown, t.JobID)
		}
	}

	if len(unknown) > 0 {
		statuses, err := j.JobStatuses(ctx, unknown)
		if err != nil {
			return plan, fmt.Errorf("look up job statuses: %w", err)
		}
		for _, t := range waiting {
			if t.JobID == (uuid.UUID{}) || cancel[t.JobID] {
				continue
			}
			status, ok := statuses[t.JobID]
			switch {
			case !ok:
				plan.Tasks = append(plan.Tasks, PlannedTask{TaskRef: t, Reason: ReasonMissing})
			case status == store.StatusCompleted || status == store.StatusFailed:
				// The worker would skip this task anyway; it only clutters the queue.
				plan.Tasks = append(plan.Tasks, PlannedTask{TaskRef: t, Reason: ReasonFinished})
			}
		}
	}

	if opt.ClearFinished && opt.UserID == "" {
		for _, name := range names {
			completed, archived, err := q.Finished(name)
			if err != nil {
				return plan, fmt.Errorf("count finished tasks in %s: %w", name, err)
			}
			plan.FinishedCompleted += completed
			plan.FinishedArchived += archived
		}
	}
	return plan, nil
}

type Result struct {
	TasksDeleted  int
	TasksSkipped  int // could not be deleted, e.g. a worker picked them up meanwhile
	JobsCancelled int64
	JobsKept      int // stuck jobs left alone because their task could not be removed
}

// Apply carries out a plan. Tasks go first so no worker can pick up work that
// is about to be cancelled. A job whose task couldn't be removed is not
// cancelled, because a worker may be running it and would just overwrite the result.
func Apply(ctx context.Context, q Queues, j Jobs, plan Plan, opt Options) (Result, error) {
	var res Result
	blocked := map[uuid.UUID]bool{}

	for _, t := range plan.Tasks {
		if err := q.Delete(t.Queue, t.ID); err != nil {
			res.TasksSkipped++
			blocked[t.JobID] = true
			continue
		}
		res.TasksDeleted++
	}

	var ids []uuid.UUID
	for _, job := range plan.Cancel {
		if blocked[job.ID] {
			res.JobsKept++
			continue
		}
		ids = append(ids, job.ID)
	}
	n, err := j.CancelUnfinished(ctx, ids)
	if err != nil {
		return res, fmt.Errorf("cancel jobs: %w", err)
	}
	res.JobsCancelled = n

	if opt.ClearFinished && opt.UserID == "" {
		names, err := q.Names()
		if err != nil {
			return res, fmt.Errorf("list queues: %w", err)
		}
		for _, name := range names {
			if err := q.ClearFinished(name); err != nil {
				return res, fmt.Errorf("clear finished tasks in %s: %w", name, err)
			}
		}
	}
	return res, nil
}
