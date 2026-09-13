package queue

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const TypeSplit = "video:split"

type SplitPayload struct {
	JobID uuid.UUID `json:"job_id"`
}

func NewSplitTask(jobID uuid.UUID) (*asynq.Task, error) {
	b, err := json.Marshal(SplitPayload{JobID: jobID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSplit, b), nil
}

func ParseSplitPayload(t *asynq.Task) (SplitPayload, error) {
	var p SplitPayload
	err := json.Unmarshal(t.Payload(), &p)
	return p, err
}
