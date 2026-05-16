package agent

import (
	"context"
	"fmt"
	"time"
)

// PlanStrategy produces and revises plans for AgentLoop without importing reasoning subpackages.
type PlanStrategy interface {
	Name() string
	Plan(ctx context.Context, agentCtx AgentContext) (*AgentPlan, error)
	Revise(ctx context.Context, agentCtx AgentContext, previous *AgentPlan, eval *Evaluation) (*AgentPlan, error)
}

// PlanActionExecutor executes one plan step without importing action subpackages.
type PlanActionExecutor interface {
	Name() string
	CanExecute(ctx context.Context, agentCtx AgentContext, step PlanStep) bool
	Execute(ctx context.Context, agentCtx AgentContext, step PlanStep) (*StepResult, error)
}

// PlanActionRunner executes one complete plan.
type PlanActionRunner interface {
	RunPlan(ctx context.Context, agentCtx AgentContext, plan *AgentPlan) (*ActionResult, error)
}

// Reflector evaluates plan execution results.
type Reflector interface {
	Reflect(ctx context.Context, agentCtx AgentContext, plan *AgentPlan, result *ActionResult, execErr error) (*Evaluation, error)
}

// LoopMemoryProvider loads memory into AgentContext and writes run experience back.
type LoopMemoryProvider interface {
	Load(ctx context.Context, req RunRequest, profile AgentProfile) (*MemorySnapshot, error)
	AppendEvent(ctx context.Context, event AgentEvent) error
	WriteRun(ctx context.Context, agentCtx AgentContext, result *ActionResult, eval *Evaluation) error
}

// AgentLoopOption configures an AgentLoop.
type AgentLoopOption func(*AgentLoop)

// AgentLoop runs a minimal perceive-plan-act-reflect-learn loop for one profile.
type AgentLoop struct {
	profile     AgentProfile
	controller  LoopController
	checkpoints CheckpointStore
	strategy    PlanStrategy
	actionRun   PlanActionRunner
	reflector   Reflector
	memory      LoopMemoryProvider
}

// NewAgentLoop creates a minimal Agent Loop.
func NewAgentLoop(profile AgentProfile, options ...AgentLoopOption) *AgentLoop {
	loop := &AgentLoop{
		profile:     cloneAgentProfile(profile),
		controller:  NewDefaultLoopController(),
		checkpoints: NewInMemoryCheckpointStore(),
		strategy:    DirectPlanStrategy{},
		actionRun:   NewSequentialActionRunner(),
		reflector:   NoopReflector{},
		memory:      NoopLoopMemoryProvider{},
	}
	for _, option := range options {
		if option != nil {
			option(loop)
		}
	}
	return loop
}

// WithAgentLoopController configures loop control.
func WithAgentLoopController(controller LoopController) AgentLoopOption {
	return func(loop *AgentLoop) {
		if controller != nil {
			loop.controller = controller
		}
	}
}

// WithAgentLoopCheckpointStore configures loop checkpointing.
func WithAgentLoopCheckpointStore(store CheckpointStore) AgentLoopOption {
	return func(loop *AgentLoop) {
		if store != nil {
			loop.checkpoints = store
		}
	}
}

// WithAgentLoopStrategy configures planning.
func WithAgentLoopStrategy(strategy PlanStrategy) AgentLoopOption {
	return func(loop *AgentLoop) {
		if strategy != nil {
			loop.strategy = strategy
		}
	}
}

// WithAgentLoopActionRunner configures action execution.
func WithAgentLoopActionRunner(runner PlanActionRunner) AgentLoopOption {
	return func(loop *AgentLoop) {
		if runner != nil {
			loop.actionRun = runner
		}
	}
}

// WithAgentLoopReflector configures result reflection.
func WithAgentLoopReflector(reflector Reflector) AgentLoopOption {
	return func(loop *AgentLoop) {
		if reflector != nil {
			loop.reflector = reflector
		}
	}
}

// WithAgentLoopMemoryProvider configures memory loading and write-back.
func WithAgentLoopMemoryProvider(provider LoopMemoryProvider) AgentLoopOption {
	return func(loop *AgentLoop) {
		if provider != nil {
			loop.memory = provider
		}
	}
}

// Run executes a minimal Agent Loop.
func (l *AgentLoop) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	agentState := AgentState{
		AgentID:       l.profile.ID,
		Status:        AgentStatusRunning,
		CurrentTaskID: req.TaskID,
		Goal:          req.Goal,
		UpdatedAt:     time.Now(),
	}
	agentCtx := AgentContext{
		Request: req.Clone(),
		Agent:   cloneAgentProfile(l.profile),
		State:   cloneAgentState(agentState),
	}
	if err := l.loadMemory(ctx, req, &agentCtx); err != nil {
		return nil, err
	}
	state := NewLoopState(req, l.profile, agentState, agentCtx)
	if err := l.appendMemoryEvent(ctx, state, "run_started", "agent loop started"); err != nil {
		return nil, err
	}

	if decision, err := l.controller.BeforeRun(ctx, state); err != nil {
		return nil, err
	} else if decision != nil && decision.Stop {
		return buildRunResult(state), nil
	}

	state.Phase = LoopPhasePlan
	plan, err := l.strategy.Plan(ctx, state.AgentContext)
	if err != nil {
		return l.handleLoopError(ctx, state, err)
	}
	state.Plan = planToMap(plan)
	state.AgentPlan = cloneAgentPlanPtr(plan)

	retries := 0
	maxRetries := l.maxReflectRetries()
	for {
		decision, err := l.controller.BeforeIteration(ctx, state)
		if err != nil {
			return nil, err
		}
		if decision != nil && decision.Stop {
			return buildRunResult(state), nil
		}

		state.Phase = LoopPhaseAct
		actionResult, err := l.actionRun.RunPlan(ctx, state.AgentContext, plan)
		state.ActionResult = actionResult
		if actionResult != nil {
			state.StepResults = stepResultsToLoopSteps(actionResult.StepResults)
			state.StepCursor = stepCursorFromStepResults(actionResult.StepResults)
		}
		if err != nil {
			return l.handleLoopError(ctx, state, err)
		}

		state.Phase = LoopPhaseReflect
		evaluation, err := l.reflector.Reflect(ctx, state.AgentContext, plan, actionResult, nil)
		if err != nil {
			return l.handleLoopError(ctx, state, err)
		}
		state.Evaluation = evaluation

		if evaluation == nil || !evaluation.Retry {
			break
		}
		if retries >= maxRetries {
			state.Status = LoopStatusFailed
			state.LastError = "reflect retry limit exceeded"
			if _, err := l.controller.AfterIteration(ctx, state); err != nil {
				return nil, err
			}
			if err := l.writeMemoryRun(ctx, state); err != nil {
				return nil, err
			}
			if err := l.saveCheckpoint(ctx, state); err != nil {
				return nil, err
			}
			return buildRunResult(state), nil
		}
		retries++
		ensureLoopMetadata(state)["reflect_retry_count"] = retries
		if _, err := l.controller.AfterIteration(ctx, state); err != nil {
			return nil, err
		}

		state.Phase = LoopPhasePlan
		revised, err := l.strategy.Revise(ctx, state.AgentContext, plan, evaluation)
		if err != nil {
			return l.handleLoopError(ctx, state, err)
		}
		plan = revised
		state.Plan = planToMap(plan)
		state.AgentPlan = cloneAgentPlanPtr(plan)
	}

	state.Phase = LoopPhaseLearn
	state.Status = LoopStatusCompleted
	if _, err := l.controller.AfterIteration(ctx, state); err != nil {
		return nil, err
	}
	if err := l.writeMemoryRun(ctx, state); err != nil {
		return nil, err
	}
	if err := l.appendMemoryEvent(ctx, state, "run_completed", "agent loop completed"); err != nil {
		return nil, err
	}
	if err := l.saveCheckpoint(ctx, state); err != nil {
		return nil, err
	}
	return buildRunResult(state), nil
}

// Resume continues a checkpointed Agent Loop from its step cursor.
func (l *AgentLoop) Resume(ctx context.Context, state *LoopState, decision *LoopDecision) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("loop state is nil")
	}
	plan := cloneAgentPlanPtr(state.AgentPlan)
	if plan == nil {
		return nil, fmt.Errorf("checkpoint missing agent plan")
	}
	applyPatchToRemainingSteps(plan, state.StepCursor, decision)
	state.AgentPlan = cloneAgentPlanPtr(plan)
	state.Plan = planToMap(plan)

	if state.StepCursor >= len(plan.Steps) {
		state.Status = LoopStatusCompleted
		if err := l.saveCheckpoint(ctx, state); err != nil {
			return nil, err
		}
		return buildRunResult(state), nil
	}

	resumePlan := plan.Clone()
	resumePlan.Steps = clonePlanSteps(plan.Steps[state.StepCursor:])
	state.Phase = LoopPhaseAct
	actionResult, err := l.actionRun.RunPlan(ctx, state.AgentContext, &resumePlan)
	if actionResult != nil {
		state.ActionResult = actionResult
		for _, step := range stepResultsToLoopSteps(actionResult.StepResults) {
			step.Index += state.StepCursor
			state.StepResults = append(state.StepResults, step)
		}
	}
	if err != nil {
		return l.handleLoopError(ctx, state, err)
	}
	state.StepCursor = len(plan.Steps)

	state.Phase = LoopPhaseReflect
	evaluation, err := l.reflector.Reflect(ctx, state.AgentContext, plan, actionResult, nil)
	if err != nil {
		return l.handleLoopError(ctx, state, err)
	}
	state.Evaluation = evaluation
	state.Phase = LoopPhaseLearn
	state.Status = LoopStatusCompleted
	if err := l.writeMemoryRun(ctx, state); err != nil {
		return nil, err
	}
	if err := l.appendMemoryEvent(ctx, state, "run_completed", "agent loop completed"); err != nil {
		return nil, err
	}
	if err := l.saveCheckpoint(ctx, state); err != nil {
		return nil, err
	}
	return buildRunResult(state), nil
}

func (l *AgentLoop) maxReflectRetries() int {
	if l.profile.Reflection.MaxRetries > 0 {
		return l.profile.Reflection.MaxRetries
	}
	if l.profile.Reasoning.MaxRetries > 0 {
		return l.profile.Reasoning.MaxRetries
	}
	return 1
}

func applyPatchToRemainingSteps(plan *AgentPlan, cursor int, decision *LoopDecision) {
	if plan == nil || decision == nil || len(decision.Patch) == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	for i := cursor; i < len(plan.Steps); i++ {
		if plan.Steps[i].Params == nil {
			plan.Steps[i].Params = make(map[string]any)
		}
		for key, value := range decision.Patch {
			plan.Steps[i].Params[key] = value
		}
	}
}

func (l *AgentLoop) handleLoopError(ctx context.Context, state *LoopState, err error) (*RunResult, error) {
	state.LastError = err.Error()
	_, controllerErr := l.controller.OnError(ctx, state, err)
	if controllerErr != nil {
		return nil, controllerErr
	}
	_ = l.writeMemoryRun(ctx, state)
	_ = l.appendMemoryEvent(ctx, state, "run_failed", err.Error())
	_ = l.saveCheckpoint(ctx, state)
	return buildRunResult(state), err
}

func (l *AgentLoop) saveCheckpoint(ctx context.Context, state *LoopState) error {
	if l.checkpoints == nil {
		return nil
	}
	return l.checkpoints.Save(ctx, NewLoopCheckpoint(state))
}

func (l *AgentLoop) loadMemory(ctx context.Context, req RunRequest, agentCtx *AgentContext) error {
	if l.memory == nil || agentCtx == nil {
		return nil
	}
	snapshot, err := l.memory.Load(ctx, req, l.profile)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	agentCtx.WorkingMemory = cloneMemoryItems(snapshot.Working)
	agentCtx.LongMemory = cloneMemoryItems(snapshot.Long)
	agentCtx.SharedMemory = cloneMap(snapshot.Shared)
	return nil
}

func (l *AgentLoop) writeMemoryRun(ctx context.Context, state *LoopState) error {
	if l.memory == nil || state == nil {
		return nil
	}
	return l.memory.WriteRun(ctx, state.AgentContext, state.ActionResult, state.Evaluation)
}

func (l *AgentLoop) appendMemoryEvent(ctx context.Context, state *LoopState, eventType, message string) error {
	if l.memory == nil || state == nil {
		return nil
	}
	return l.memory.AppendEvent(ctx, AgentEvent{
		RunID:     state.RunID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"phase":  string(state.Phase),
			"status": string(state.Status),
		},
	})
}

// NoopReflector returns a successful evaluation without additional work.
type NoopReflector struct{}

// Reflect implements Reflector.
func (NoopReflector) Reflect(ctx context.Context, _ AgentContext, _ *AgentPlan, result *ActionResult, execErr error) (*Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if execErr != nil {
		return &Evaluation{Success: false, Score: 0, Reason: execErr.Error(), Retry: true}, nil
	}
	if result != nil && result.Error != "" {
		return &Evaluation{Success: false, Score: 0, Reason: result.Error, Retry: true}, nil
	}
	return &Evaluation{Success: true, Score: 1, Reason: "noop reflector accepted result"}, nil
}

// NoopLoopMemoryProvider is a safe default memory provider for AgentLoop.
type NoopLoopMemoryProvider struct{}

// Load returns an empty memory snapshot.
func (NoopLoopMemoryProvider) Load(ctx context.Context, _ RunRequest, _ AgentProfile) (*MemorySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &MemorySnapshot{}, nil
}

// AppendEvent ignores memory events.
func (NoopLoopMemoryProvider) AppendEvent(ctx context.Context, _ AgentEvent) error {
	return ctx.Err()
}

// WriteRun ignores memory write-back.
func (NoopLoopMemoryProvider) WriteRun(ctx context.Context, _ AgentContext, _ *ActionResult, _ *Evaluation) error {
	return ctx.Err()
}

// DirectPlanStrategy is a local direct strategy used by the minimal AgentLoop default.
type DirectPlanStrategy struct{}

// Name returns the strategy name.
func (DirectPlanStrategy) Name() string {
	return "direct"
}

// Plan converts the request into a single-step plan.
func (s DirectPlanStrategy) Plan(ctx context.Context, agentCtx AgentContext) (*AgentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req := agentCtx.Request.Clone()
	step := PlanStep{
		StepID:   "step-1",
		Params:   cloneMap(req.Params),
		Context:  cloneMap(req.Context),
		Expected: "complete request",
	}
	switch {
	case req.SkillName != "":
		step.Type = StepTypeSkill
		step.SkillName = req.SkillName
		step.Target = req.SkillName
	case req.Action != "":
		step.Type = StepTypeTool
		step.Action = req.Action
		step.Target = req.Action
	default:
		step.Type = StepTypeLLM
		step.Action = "respond"
		step.Target = "llm"
	}
	return &AgentPlan{
		PlanID:   planID(req),
		Strategy: s.Name(),
		Goal:     planGoal(req),
		Steps:    []PlanStep{step},
		MaxSteps: 1,
		StopPolicy: StopPolicy{
			MaxSteps:        1,
			StopOnError:     true,
			StopOnFinalStep: true,
		},
		CreatedAt: time.Now(),
	}, nil
}

// Revise returns a fresh direct plan for the current context.
func (s DirectPlanStrategy) Revise(ctx context.Context, agentCtx AgentContext, _ *AgentPlan, _ *Evaluation) (*AgentPlan, error) {
	return s.Plan(ctx, agentCtx)
}

// SequentialActionRunner runs plan steps in order with the first matching executor.
type SequentialActionRunner struct {
	executors []PlanActionExecutor
}

// NewSequentialActionRunner creates an ordered action runner.
func NewSequentialActionRunner(executors ...PlanActionExecutor) *SequentialActionRunner {
	return &SequentialActionRunner{executors: executors}
}

// RunPlan executes each plan step sequentially.
func (r *SequentialActionRunner) RunPlan(ctx context.Context, agentCtx AgentContext, plan *AgentPlan) (*ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("agent plan is nil")
	}
	out := &ActionResult{Status: "completed"}
	for index, step := range plan.Steps {
		executor := r.findExecutor(ctx, agentCtx, step)
		if executor == nil {
			err := fmt.Errorf("%w: %s", ErrActionExecutorNotFound, step.Action)
			out.Status = "failed"
			out.Error = err.Error()
			return out, err
		}
		result, err := executor.Execute(ctx, agentCtx, step)
		if result != nil {
			result.Index = index
			out.StepResults = append(out.StepResults, *result)
			if result.Output != "" {
				out.Output = result.Output
			}
			if result.Result != nil {
				out.Result = cloneMap(result.Result)
			}
		}
		if err != nil {
			out.Status = "failed"
			out.Error = err.Error()
			return out, err
		}
	}
	return out, nil
}

func (r *SequentialActionRunner) findExecutor(ctx context.Context, agentCtx AgentContext, step PlanStep) PlanActionExecutor {
	for _, executor := range r.executors {
		if executor != nil && executor.CanExecute(ctx, agentCtx, step) {
			return executor
		}
	}
	return nil
}

func buildRunResult(state *LoopState) *RunResult {
	result := &RunResult{
		RunID:       state.RunID,
		AgentID:     state.Agent.ID,
		TaskID:      state.Request.TaskID,
		SubTaskID:   state.Request.SubTaskID,
		Status:      string(state.Status),
		Strategy:    strategyFromState(state),
		Steps:       cloneLoopSteps(state.StepResults),
		Evaluation:  state.Evaluation,
		Error:       state.LastError,
		CompletedAt: time.Now(),
	}
	if state.ActionResult != nil {
		result.Output = state.ActionResult.Output
		result.Result = cloneMap(state.ActionResult.Result)
	}
	return result
}

func strategyFromState(state *LoopState) string {
	if state == nil || state.Plan == nil {
		return ""
	}
	strategy, _ := state.Plan["strategy"].(string)
	return strategy
}

func stepResultsToLoopSteps(in []StepResult) []LoopStep {
	if in == nil {
		return nil
	}
	out := make([]LoopStep, len(in))
	for i, step := range in {
		out[i] = LoopStep{
			Index:       step.Index,
			Type:        string(step.Type),
			Action:      step.Action,
			SkillName:   step.SkillName,
			Status:      step.Status,
			Output:      step.Output,
			Result:      cloneMap(step.Result),
			Error:       step.Error,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
			Metadata:    cloneMap(step.Metadata),
		}
	}
	return out
}

func stepCursorFromStepResults(in []StepResult) int {
	cursor := 0
	for _, step := range in {
		if step.Status != "completed" {
			break
		}
		cursor++
	}
	return cursor
}

func planToMap(plan *AgentPlan) map[string]any {
	if plan == nil {
		return nil
	}
	return map[string]any{
		"plan_id":  plan.PlanID,
		"strategy": plan.Strategy,
		"goal":     plan.Goal,
		"steps":    len(plan.Steps),
	}
}

func planID(req RunRequest) string {
	if req.RunID != "" {
		return req.RunID + ":direct"
	}
	if req.TaskID != "" {
		return req.TaskID + ":direct"
	}
	return "direct"
}

func planGoal(req RunRequest) string {
	switch {
	case req.Goal != "":
		return req.Goal
	case req.Action != "":
		return req.Action
	case req.SkillName != "":
		return req.SkillName
	default:
		return "complete request"
	}
}
