package agent

import (
	"context"
	"time"
)

// ResumableLoop continues a checkpointed loop state.
type ResumableLoop interface {
	Resume(ctx context.Context, state *LoopState, decision *LoopDecision) (*RunResult, error)
}

// Resume restores a checkpointed loop state and applies one human response.
func (r *AgentRuntime) Resume(ctx context.Context, runID string, input HumanInput) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.isStarted() {
		return nil, ErrRuntimeNotStarted
	}
	if r.checkpoints == nil {
		return nil, ErrCheckpointNotFound
	}
	if runID == "" {
		runID = input.RunID
	}

	checkpoint, err := r.checkpoints.Load(ctx, runID)
	if err != nil {
		return nil, err
	}
	state := checkpoint.ToLoopState()
	if input.RunID == "" {
		input.RunID = runID
	}

	decision, err := r.controller.OnHumanResponse(ctx, state, input)
	if err != nil {
		return nil, err
	}
	applyResumePatch(state, decision)

	loop := r.loopFactory.NewLoop(state.Agent)
	resumable, ok := loop.(ResumableLoop)
	if ok {
		return resumable.Resume(ctx, state, decision)
	}

	return &RunResult{
		RunID:       state.RunID,
		AgentID:     state.Agent.ID,
		TaskID:      state.Request.TaskID,
		SubTaskID:   state.Request.SubTaskID,
		Status:      string(state.Status),
		Steps:       cloneLoopSteps(state.StepResults),
		Error:       state.LastError,
		CompletedAt: time.Now(),
		Result: map[string]any{
			"phase":       string(state.Phase),
			"iteration":   state.Iteration,
			"step_cursor": state.StepCursor,
			"resumed":     decision != nil && decision.Resume,
		},
	}, nil
}

func applyResumePatch(state *LoopState, decision *LoopDecision) {
	if state == nil || decision == nil || len(decision.Patch) == 0 {
		return
	}
	metadata := ensureLoopMetadata(state)
	for key, value := range decision.Patch {
		metadata[key] = value
	}
}
