package collaboration

import (
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

// HandoffRequest describes one delegated child run between agents.
type HandoffRequest struct {
	HandoffID   string          `json:"handoff_id"`
	FromAgentID agent.AgentID   `json:"from_agent_id"`
	ToAgentID   agent.AgentID   `json:"to_agent_id"`
	ParentRunID string          `json:"parent_run_id"`
	Goal        string          `json:"goal"`
	Action      string          `json:"action"`
	SkillName   string          `json:"skill_name,omitempty"`
	Params      map[string]any  `json:"params,omitempty"`
	Context     map[string]any  `json:"context,omitempty"`
	Contract    HandoffContract `json:"contract"`
}

// HandoffContract captures the expected behavior of a delegated child run.
type HandoffContract struct {
	ExpectedOutput       string        `json:"expected_output"`
	Deadline             time.Duration `json:"deadline,omitempty"`
	ReturnTrace          bool          `json:"return_trace,omitempty"`
	AllowFurtherDelegate bool          `json:"allow_further_delegate,omitempty"`
}

// HandoffResult records the outcome of one delegated child run.
type HandoffResult struct {
	HandoffID string             `json:"handoff_id"`
	AgentID   agent.AgentID      `json:"agent_id"`
	RunID     string             `json:"run_id"`
	Status    string             `json:"status"`
	Output    string             `json:"output,omitempty"`
	Result    map[string]any     `json:"result,omitempty"`
	Error     string             `json:"error,omitempty"`
	Trace     []agent.AgentEvent `json:"trace,omitempty"`
}
