package agent

import (
	"context"
	"sync"
	"time"
)

// CheckpointStore persists Agent Loop checkpoints for pause/resume and recovery.
type CheckpointStore interface {
	Save(ctx context.Context, checkpoint LoopCheckpoint) error
	Load(ctx context.Context, runID string) (*LoopCheckpoint, error)
	Delete(ctx context.Context, runID string) error
}

// ResumeToken identifies a resumable checkpoint snapshot.
type ResumeToken string

// LoopCheckpoint records the mutable Agent Loop state needed to resume a run.
type LoopCheckpoint struct {
	RunID        string         `json:"run_id"`
	AgentID      AgentID        `json:"agent_id,omitempty"`
	Iteration    int            `json:"iteration"`
	Phase        LoopPhase      `json:"phase"`
	StepCursor   int            `json:"step_cursor"`
	Request      RunRequest     `json:"request"`
	Agent        AgentProfile   `json:"agent"`
	AgentState   AgentState     `json:"agent_state"`
	AgentContext AgentContext   `json:"agent_context"`
	Plan         map[string]any `json:"plan,omitempty"`
	AgentPlan    *AgentPlan     `json:"agent_plan,omitempty"`
	StepResults  []LoopStep     `json:"step_results,omitempty"`
	Context      map[string]any `json:"context,omitempty"`
	Status       LoopStatus     `json:"status"`
	LastError    string         `json:"last_error,omitempty"`
	ResumeToken  ResumeToken    `json:"resume_token,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// NewLoopCheckpoint creates a checkpoint snapshot from the current loop state.
func NewLoopCheckpoint(state *LoopState) LoopCheckpoint {
	if state == nil {
		return LoopCheckpoint{}
	}
	now := time.Now()
	checkpoint := LoopCheckpoint{
		RunID:        state.RunID,
		AgentID:      state.Agent.ID,
		Iteration:    state.Iteration,
		Phase:        state.Phase,
		StepCursor:   state.StepCursor,
		Request:      state.Request.Clone(),
		Agent:        cloneAgentProfile(state.Agent),
		AgentState:   cloneAgentState(state.AgentState),
		AgentContext: cloneAgentContext(state.AgentContext),
		Plan:         cloneMap(state.Plan),
		AgentPlan:    cloneAgentPlanPtr(state.AgentPlan),
		StepResults:  cloneLoopSteps(state.StepResults),
		Context:      cloneMap(state.Request.Context),
		Status:       state.Status,
		LastError:    state.LastError,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     cloneMap(state.Metadata),
	}
	return checkpoint
}

// Clone returns an independent checkpoint copy for safe store boundaries.
func (c LoopCheckpoint) Clone() LoopCheckpoint {
	clone := c
	clone.Request = c.Request.Clone()
	clone.Agent = cloneAgentProfile(c.Agent)
	clone.AgentState = cloneAgentState(c.AgentState)
	clone.AgentContext = cloneAgentContext(c.AgentContext)
	clone.Plan = cloneMap(c.Plan)
	clone.AgentPlan = cloneAgentPlanPtr(c.AgentPlan)
	clone.StepResults = cloneLoopSteps(c.StepResults)
	clone.Context = cloneMap(c.Context)
	clone.Metadata = cloneMap(c.Metadata)
	return clone
}

// ToLoopState restores a LoopState snapshot from the checkpoint.
func (c LoopCheckpoint) ToLoopState() *LoopState {
	clone := c.Clone()
	return &LoopState{
		RunID:        clone.RunID,
		Iteration:    clone.Iteration,
		Phase:        clone.Phase,
		Request:      clone.Request,
		Agent:        clone.Agent,
		AgentState:   clone.AgentState,
		AgentContext: clone.AgentContext,
		Plan:         clone.Plan,
		AgentPlan:    clone.AgentPlan,
		StepCursor:   clone.StepCursor,
		StepResults:  clone.StepResults,
		LastError:    clone.LastError,
		Status:       clone.Status,
		Metadata:     clone.Metadata,
	}
}

// InMemoryCheckpointStore stores checkpoints in process memory.
type InMemoryCheckpointStore struct {
	mu          sync.RWMutex
	checkpoints map[string]LoopCheckpoint
}

// NewInMemoryCheckpointStore creates an empty in-memory checkpoint store.
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]LoopCheckpoint),
	}
}

// Save stores or replaces a checkpoint by run ID.
func (s *InMemoryCheckpointStore) Save(ctx context.Context, checkpoint LoopCheckpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.ensure()
	clone := checkpoint.Clone()
	if clone.RunID == "" {
		clone.RunID = clone.Request.RunID
	}
	if clone.CreatedAt.IsZero() {
		clone.CreatedAt = time.Now()
	}
	clone.UpdatedAt = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[clone.RunID] = clone
	return nil
}

// Load returns an independent checkpoint copy by run ID.
func (s *InMemoryCheckpointStore) Load(ctx context.Context, runID string) (*LoopCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.ensure()

	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.checkpoints[runID]
	if !ok {
		return nil, ErrCheckpointNotFound
	}
	clone := checkpoint.Clone()
	return &clone, nil
}

// Delete removes a checkpoint by run ID.
func (s *InMemoryCheckpointStore) Delete(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.ensure()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.checkpoints[runID]; !ok {
		return ErrCheckpointNotFound
	}
	delete(s.checkpoints, runID)
	return nil
}

func (s *InMemoryCheckpointStore) ensure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkpoints == nil {
		s.checkpoints = make(map[string]LoopCheckpoint)
	}
}

func cloneAgentProfile(in AgentProfile) AgentProfile {
	out := in
	out.Capabilities = cloneCapabilities(in.Capabilities)
	out.Tools = append([]string(nil), in.Tools...)
	out.Skills = append([]string(nil), in.Skills...)
	out.Reasoning.Metadata = cloneMap(in.Reasoning.Metadata)
	out.Memory.Metadata = cloneMap(in.Memory.Metadata)
	out.Reflection.Metadata = cloneMap(in.Reflection.Metadata)
	out.Collaboration.AllowedRoles = append([]AgentRole(nil), in.Collaboration.AllowedRoles...)
	out.Collaboration.Metadata = cloneMap(in.Collaboration.Metadata)
	out.Metadata = cloneMap(in.Metadata)
	return out
}

func cloneAgentState(in AgentState) AgentState {
	out := in
	out.Scratchpad = cloneMap(in.Scratchpad)
	if in.LastResult != nil {
		result := cloneActionResult(*in.LastResult)
		out.LastResult = &result
	}
	return out
}

func cloneAgentContext(in AgentContext) AgentContext {
	out := in
	out.Request = in.Request.Clone()
	out.Agent = cloneAgentProfile(in.Agent)
	out.State = cloneAgentState(in.State)
	out.WorkingMemory = cloneMemoryItems(in.WorkingMemory)
	out.LongMemory = cloneMemoryItems(in.LongMemory)
	out.SharedMemory = cloneMap(in.SharedMemory)
	out.SkillSpec = cloneMap(in.SkillSpec)
	out.ActivatedSkills = cloneActivatedSkills(in.ActivatedSkills)
	out.ToolCatalog = cloneToolDescriptors(in.ToolCatalog)
	out.Trace = cloneAgentEvents(in.Trace)
	return out
}

func cloneCapabilities(in []Capability) []Capability {
	if in == nil {
		return nil
	}
	out := make([]Capability, len(in))
	for i, capability := range in {
		out[i] = capability
		out[i].Actions = append([]string(nil), capability.Actions...)
		out[i].Skills = append([]string(nil), capability.Skills...)
		out[i].Tools = append([]string(nil), capability.Tools...)
	}
	return out
}

func cloneLoopSteps(in []LoopStep) []LoopStep {
	if in == nil {
		return nil
	}
	out := make([]LoopStep, len(in))
	for i, step := range in {
		out[i] = step
		out[i].Input = cloneMap(step.Input)
		out[i].Result = cloneMap(step.Result)
		out[i].Metadata = cloneMap(step.Metadata)
	}
	return out
}

func cloneAgentPlanPtr(in *AgentPlan) *AgentPlan {
	if in == nil {
		return nil
	}
	out := in.Clone()
	return &out
}

func cloneActionResult(in ActionResult) ActionResult {
	out := in
	out.Result = cloneMap(in.Result)
	out.Steps = cloneLoopSteps(in.Steps)
	return out
}

func cloneMemoryItems(in []MemoryItem) []MemoryItem {
	if in == nil {
		return nil
	}
	out := make([]MemoryItem, len(in))
	for i, item := range in {
		out[i] = item
		out[i].Metadata = cloneMap(item.Metadata)
	}
	return out
}

func cloneActivatedSkills(in []ActivatedSkill) []ActivatedSkill {
	if in == nil {
		return nil
	}
	out := make([]ActivatedSkill, len(in))
	for i, skill := range in {
		out[i] = skill
		out[i].Metadata = cloneMap(skill.Metadata)
	}
	return out
}

func cloneToolDescriptors(in []ToolDescriptor) []ToolDescriptor {
	if in == nil {
		return nil
	}
	out := make([]ToolDescriptor, len(in))
	for i, tool := range in {
		out[i] = tool
		out[i].InputSchema = cloneMap(tool.InputSchema)
		out[i].Metadata = cloneMap(tool.Metadata)
	}
	return out
}

func cloneAgentEvents(in []AgentEvent) []AgentEvent {
	if in == nil {
		return nil
	}
	out := make([]AgentEvent, len(in))
	for i, event := range in {
		out[i] = event
		out[i].Metadata = cloneMap(event.Metadata)
	}
	return out
}
