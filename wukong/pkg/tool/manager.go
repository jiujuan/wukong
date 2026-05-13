package tool

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/skills"
	wkstr "github.com/jiujuan/wukong/pkg/str"
	tooltools "github.com/jiujuan/wukong/pkg/tool/tools"
)

type Manager struct {
	mu             sync.RWMutex
	tools          map[string]Tool
	logger         *pkglogger.Logger
	llmProvider    *llm.Provider
	skillsRegistry *skills.Registry
	memoryStore    MemoryStore
	baseDir        string
	fileWriteDir   string
	httpClient     *http.Client
	execTimeout    time.Duration
}

func NewManager(opts ...Option) *Manager {
	m := &Manager{
		tools:          make(map[string]Tool),
		logger:         pkglogger.FromSlog(slog.Default()),
		skillsRegistry: skills.New(),
		memoryStore:    NewInMemoryStore(),
		baseDir:        ".",
		fileWriteDir:   "storage/output_data",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		execTimeout: 20 * time.Second,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.registerBuiltins()
	m.logger.Info("[ToolManager] initialized",
		"base_dir", m.baseDir,
		"file_write_dir", m.fileWriteDir,
		"exec_timeout", m.execTimeout,
	)
	return m
}

func (m *Manager) Register(tool Tool) {
	if tool == nil {
		m.logger.Warn("[ToolManager] skip register: tool is nil")
		return
	}
	key := wkstr.TrimLower(tool.Name())
	if key == "" {
		m.logger.Warn("[ToolManager] skip register: tool name is empty")
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[key] = tool
	m.logger.Info("[ToolManager] tool registered", "tool", key)
}

func (m *Manager) Get(name string) (Tool, bool) {
	key := wkstr.TrimLower(name)
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.tools[key]
	return item, ok
}

func (m *Manager) List() []ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]ToolInfo, 0, len(m.tools))
	for _, t := range m.tools {
		items = append(items, ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.ParameterSchema(),
		})
	}
	return items
}

func (m *Manager) Execute(ctx context.Context, name string, params map[string]any) (map[string]any, error) {
	start := time.Now()
	item, ok := m.Get(name)
	if !ok {
		m.logger.Warn("[ToolManager] execute failed: tool not found", "tool", name)
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	m.logger.Info("[ToolManager] execute start",
		"tool", item.Name(),
		"params_keys", mapKeys(params),
	)
	result, err := item.Execute(ctx, params)
	if err != nil {
		m.logger.Error("[ToolManager] execute failed",
			"tool", item.Name(),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return nil, err
	}
	m.logger.Info("[ToolManager] execute success",
		"tool", item.Name(),
		"duration_ms", time.Since(start).Milliseconds(),
		"result_keys", mapKeys(result),
	)
	return result, nil
}

func (m *Manager) ExecuteForSkill(ctx context.Context, skillName, toolName string, params map[string]any) (map[string]any, error) {
	normalizedSkill := wkstr.TrimLower(skillName)
	normalizedTool := wkstr.TrimLower(toolName)
	if m.skillsRegistry == nil {
		return nil, fmt.Errorf("skills registry is not configured")
	}
	skill, exists := m.skillsRegistry.Get(normalizedSkill)
	if !exists {
		m.logger.Warn("[ToolManager] execute blocked: skill not found",
			"skill", normalizedSkill, "tool", normalizedTool,
		)
		return nil, fmt.Errorf("skill not found: %s", normalizedSkill)
	}
	if !m.skillsRegistry.CanUseTool(normalizedSkill, normalizedTool) {
		m.logger.Warn("[ToolManager] execute blocked by policy",
			"skill", normalizedSkill, "tool", normalizedTool,
		)
		return nil, fmt.Errorf("tool %s is not allowed for skill %s", normalizedTool, normalizedSkill)
	}
	skillCtx := tooltools.SkillContext{
		SkillName:   skill.SkillName,
		Version:     skill.Version,
		SourceType:  string(skill.Package.SourceType),
		PackageName: skill.Package.PackageName,
	}
	if wkstr.NotEmpty(skill.Package.RootDir) {
		skillCtx.SkillRoot = skill.Package.RootDir
	} else if wkstr.NotEmpty(skill.SourcePath) {
		skillCtx.SkillRoot = filepath.Dir(skill.SourcePath)
	}
	if wkstr.NotEmpty(skillCtx.SkillRoot) {
		skillCtx.SkillRoot = filepath.Clean(skillCtx.SkillRoot)
	}
	if wkstr.NotEmpty(m.fileWriteDir) && wkstr.NotEmpty(skillCtx.SkillName) {
		skillCtx.OutputDir = filepath.Join(m.fileWriteDir, skillCtx.SkillName)
	}
	ctx = tooltools.WithSkillContext(ctx, skillCtx)
	m.logger.Info("[ToolManager] execute for skill",
		"skill", normalizedSkill, "tool", normalizedTool,
	)
	return m.Execute(ctx, normalizedTool, params)
}

func (m *Manager) registerBuiltins() {
	m.Register(tooltools.NewLLMTool(m.llmProvider, m.logger))
	m.Register(tooltools.NewWebSearchTool(m.httpClient, m.logger))
	m.Register(tooltools.NewFileReadTool(m.baseDir, m.logger))
	m.Register(tooltools.NewFileWriteTool(m.fileWriteDir, m.logger))
	m.Register(tooltools.NewHTTPTool(m.httpClient, m.logger))
	m.Register(tooltools.NewCodeExecTool(m.execTimeout, m.logger))
	m.Register(tooltools.NewMemoryReadTool(m.memoryStore, m.logger))
	m.Register(tooltools.NewMemoryWriteTool(m.memoryStore, m.logger))
	m.logger.Info("[ToolManager] builtin tools ready", "count", len(m.tools))
}

func mapKeys(items map[string]any) []string {
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
