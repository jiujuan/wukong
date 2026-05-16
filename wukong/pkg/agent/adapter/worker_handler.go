package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/queue"
)

// Runtime is the minimal Agent Runtime dependency consumed by WorkerHandler.
type Runtime interface {
	Run(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error)
}

// WorkerHandler adapts WorkerPool queue tasks to Agent Runtime runs.
type WorkerHandler struct {
	runtime Runtime
	mapper  SubTaskMapper
}

// NewWorkerHandler creates a WorkerPool handler backed by Agent Runtime.
func NewWorkerHandler(runtime Runtime, mapper SubTaskMapper) *WorkerHandler {
	if mapper == nil {
		mapper = NewDefaultSubTaskMapper()
	}
	return &WorkerHandler{runtime: runtime, mapper: mapper}
}

// Handle converts a queue task into a run request and writes the result back to the subtask.
func (h *WorkerHandler) Handle(ctx context.Context, task *queue.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if h == nil || h.runtime == nil {
		return fmt.Errorf("agent runtime is not configured")
	}
	mapper := h.mapper
	if mapper == nil {
		mapper = NewDefaultSubTaskMapper()
	}

	req, holder, err := mapper.FromQueueTask(task)
	if err != nil {
		return err
	}

	result, err := h.runtime.Run(ctx, req)
	if err != nil {
		holder.SetError(err.Error())
		holder.SetUpdatedAt(time.Now())
		return err
	}

	holder.SetResult(mapper.ToSubTaskResult(result))
	holder.SetError("")
	holder.SetUpdatedAt(time.Now())
	return nil
}
