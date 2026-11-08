package purge

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Asynq adapts an asynq.Inspector to Queues.
type Asynq struct{ In *asynq.Inspector }

const pageSize = 200

func (a Asynq) Names() ([]string, error) { return a.In.Queues() }

func (a Asynq) Waiting(queue string) ([]TaskRef, error) {
	var out []TaskRef
	for _, list := range []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
		a.In.ListPendingTasks, a.In.ListScheduledTasks, a.In.ListRetryTasks,
	} {
		refs, err := collect(queue, list)
		if err != nil {
			return nil, err
		}
		out = append(out, refs...)
	}
	return out, nil
}

func (a Asynq) Active(queue string) ([]TaskRef, error) {
	return collect(queue, a.In.ListActiveTasks)
}

func (a Asynq) Delete(queue, taskID string) error { return a.In.DeleteTask(queue, taskID) }

func (a Asynq) Finished(queue string) (int, int, error) {
	info, err := a.In.GetQueueInfo(queue)
	if err != nil {
		return 0, 0, err
	}
	return info.Completed, info.Archived, nil
}

func (a Asynq) ClearFinished(queue string) error {
	if _, err := a.In.DeleteAllCompletedTasks(queue); err != nil {
		return err
	}
	_, err := a.In.DeleteAllArchivedTasks(queue)
	return err
}

// collect pages through one of the inspector's list calls.
func collect(queue string, list func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)) ([]TaskRef, error) {
	var out []TaskRef
	for page := 1; ; page++ {
		tasks, err := list(queue, asynq.PageSize(pageSize), asynq.Page(page))
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			var p struct {
				JobID uuid.UUID `json:"job_id"`
			}
			// An unreadable payload leaves JobID as the zero UUID, which the
			// planner reports and never touches.
			_ = json.Unmarshal(t.Payload, &p)
			out = append(out, TaskRef{Queue: t.Queue, ID: t.ID, State: t.State.String(), JobID: p.JobID})
		}
		if len(tasks) < pageSize {
			return out, nil
		}
	}
}
