package queue

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TypeDownload = "video:download"
	TypeProcess  = "video:process"
	TypeSplit    = "video:split" // Alias for TypeProcess (backwards compatibility)

	QueueDownload = "downloads"
	QueueProcess  = "processing"
)

type JobPayload struct {
	JobID uuid.UUID `json:"job_id"`
}

// SplitPayload alias for backwards compatibility
type SplitPayload = JobPayload

func NewDownloadTask(jobID uuid.UUID) (*asynq.Task, error) {
	b, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDownload, b), nil
}

func NewProcessTask(jobID uuid.UUID) (*asynq.Task, error) {
	b, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProcess, b), nil
}

func NewSplitTask(jobID uuid.UUID) (*asynq.Task, error) {
	return NewProcessTask(jobID)
}

func ParsePayload(t *asynq.Task) (JobPayload, error) {
	var p JobPayload
	err := json.Unmarshal(t.Payload(), &p)
	return p, err
}

func ParseSplitPayload(t *asynq.Task) (SplitPayload, error) {
	return ParsePayload(t)
}

func ParseRedisOpt(addr string) (asynq.RedisConnOpt, error) {
	if addr == "" {
		return asynq.RedisClientOpt{Addr: "localhost:6379"}, nil
	}
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		return asynq.ParseRedisURI(addr)
	}
	return asynq.RedisClientOpt{Addr: addr}, nil
}
