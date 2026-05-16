package builtin

import "github.com/jiujuan/wukong/pkg/agent"

// GeneralAgent is the default fallback agent profile.
var GeneralAgent = agent.AgentProfile{
	ID:          agent.AgentID("general"),
	Name:        "General Agent",
	Role:        agent.AgentRoleGeneral,
	Description: "Default fallback agent for broad tasks.",
	Tools:       []string{"llm_chat", "web_search", "file_read"},
	Capabilities: []agent.Capability{
		{
			Name:        "fallback",
			Description: "Handle general requests when no specialized agent matches.",
			Actions:     []string{"respond", "answer", "chat"},
			Tools:       []string{"llm_chat", "web_search", "file_read"},
			Priority:    1,
		},
	},
	Reasoning: agent.ReasoningConfig{
		Strategy:   "react",
		MaxRetries: 1,
	},
	Memory: agent.MemoryConfig{
		Enabled:        true,
		WorkingEnabled: true,
		LongEnabled:    true,
	},
	Reflection: agent.ReflectConfig{
		Enabled:    true,
		Strategy:   "rule",
		MaxRetries: 1,
	},
	Collaboration: agent.CollaborationConfig{
		CanDelegate: true,
		MaxDepth:    2,
	},
}

// ResearchAgent is optimized for search, synthesis, and source-heavy work.
var ResearchAgent = agent.AgentProfile{
	ID:          agent.AgentID("research"),
	Name:        "Research Agent",
	Role:        agent.AgentRoleResearch,
	Description: "Performs deep search and information synthesis.",
	Tools:       []string{"web_search", "http", "file_read", "llm_chat"},
	Capabilities: []agent.Capability{
		{
			Name:        "research",
			Description: "Search, gather, and synthesize information.",
			Actions:     []string{"research", "search", "summarize_sources"},
			Tools:       []string{"web_search", "http", "file_read", "llm_chat"},
			Priority:    10,
		},
	},
	Reasoning: agent.ReasoningConfig{
		Strategy: "plan_execute",
		Metadata: map[string]any{
			"max_iterations": 12,
		},
	},
	Memory: agent.MemoryConfig{
		Enabled:        true,
		WorkingEnabled: true,
		LongEnabled:    true,
		SharedEnabled:  true,
	},
	Reflection: agent.ReflectConfig{
		Enabled:  true,
		Strategy: "llm",
	},
	Collaboration: agent.CollaborationConfig{
		CanDelegate: true,
		CanBeTarget: true,
		MaxDepth:    2,
	},
}

// ToolAgent is optimized for deterministic tool execution.
var ToolAgent = agent.AgentProfile{
	ID:          agent.AgentID("tool"),
	Name:        "Tool Agent",
	Role:        agent.AgentRoleTool,
	Description: "Executes deterministic tool and file operations.",
	Tools:       []string{"file_read", "file_write", "http", "code_exec", "memory_read", "memory_write"},
	Capabilities: []agent.Capability{
		{
			Name:        "tool_execution",
			Description: "Run deterministic tools and return structured results.",
			Actions:     []string{"file_read", "file_write", "http", "code_exec", "memory_read", "memory_write"},
			Tools:       []string{"file_read", "file_write", "http", "code_exec", "memory_read", "memory_write"},
			Priority:    20,
		},
	},
	Reasoning: agent.ReasoningConfig{
		Strategy: "direct",
	},
	Memory: agent.MemoryConfig{
		Enabled:        true,
		WorkingEnabled: true,
	},
	Reflection: agent.ReflectConfig{
		Enabled:    true,
		Strategy:   "rule",
		MaxRetries: 1,
	},
	Collaboration: agent.CollaborationConfig{
		CanBeTarget: true,
	},
}

// CriticAgent is optimized for review and evaluation handoffs.
var CriticAgent = agent.AgentProfile{
	ID:          agent.AgentID("critic"),
	Name:        "Critic Agent",
	Role:        agent.AgentRoleCritic,
	Description: "Reviews, evaluates, and critiques agent outputs.",
	Tools:       []string{"llm_chat"},
	Capabilities: []agent.Capability{
		{
			Name:        "review",
			Description: "Review, evaluate, and critique results.",
			Actions:     []string{"review", "evaluate", "critique"},
			Tools:       []string{"llm_chat"},
			Priority:    15,
		},
	},
	Reasoning: agent.ReasoningConfig{
		Strategy: "direct",
	},
	Reflection: agent.ReflectConfig{
		Enabled:  true,
		Strategy: "llm",
	},
	Collaboration: agent.CollaborationConfig{
		CanBeTarget:  true,
		AllowedRoles: []agent.AgentRole{agent.AgentRoleGeneral, agent.AgentRoleResearch, agent.AgentRoleTool},
	},
}

// DefaultProfiles returns the built-in agent profiles in fallback order.
func DefaultProfiles() []agent.AgentProfile {
	return []agent.AgentProfile{
		GeneralAgent,
		ResearchAgent,
		ToolAgent,
		CriticAgent,
	}
}
