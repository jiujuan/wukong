package skills

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Param struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	DefaultVal string `json:"default_val,omitempty"`
}

type MemoryConfig struct {
	MemoryType     string `json:"memory_type"`
	WindowSize     int    `json:"window_size"`
	CompressSwitch bool   `json:"compress_switch"`
	RAGCollection  string `json:"rag_collection,omitempty"`
	ExpireTime     string `json:"expire_time,omitempty"`
}

type Skill struct {
	SkillName   string       `json:"skill_name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Enabled     bool         `json:"enabled"`
	Params      []Param      `json:"params"`
	Tools       []string     `json:"tools"`
	Execute     string       `json:"execute,omitempty"`
	Template    string       `json:"template,omitempty"`
	Memory      MemoryConfig `json:"memory"`
	SourcePath  string       `json:"source_path,omitempty"`
}

type MetaStore interface {
	BatchUpsertSkills(ctx context.Context, items []*Skill) error
}

type Option func(*Registry)

type Registry struct {
	rootDir      string
	pollInterval time.Duration
	execTimeout  time.Duration
	logger       *slog.Logger
	store        MetaStore

	mu      sync.RWMutex
	skills  map[string]*Skill
	started bool
	cancel  context.CancelFunc
}

func WithRootDir(rootDir string) Option {
	return func(r *Registry) {
		if strings.TrimSpace(rootDir) != "" {
			r.rootDir = rootDir
		}
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(r *Registry) {
		if interval > 0 {
			r.pollInterval = interval
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(r *Registry) {
		if timeout > 0 {
			r.execTimeout = timeout
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(r *Registry) {
		if logger != nil {
			r.logger = logger
		}
	}
}

func WithMetaStore(store MetaStore) Option {
	return func(r *Registry) {
		r.store = store
	}
}

func New(opts ...Option) *Registry {
	r := &Registry{
		rootDir:      "skills",
		pollInterval: 3 * time.Second,
		execTimeout:  60 * time.Second,
		logger:       slog.Default(),
		skills:       make(map[string]*Skill),
	}
	for _, opt := range opts {
		opt(r)
	}
	for _, item := range defaultBuiltins() {
		r.skills[item.SkillName] = cloneSkill(item)
	}
	return r
}

func (r *Registry) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.started = true
	r.mu.Unlock()

	if err := r.reload(runCtx); err != nil && r.logger != nil {
		r.logger.Warn("[skills] initial reload failed", "error", err)
	}

	go r.watch(runCtx)
	return nil
}

func (r *Registry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		return
	}
	r.started = false
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}

func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Skill, 0, len(r.skills))
	for _, item := range r.skills {
		result = append(result, cloneSkill(item))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SkillName < result[j].SkillName
	})
	return result
}

func (r *Registry) Get(skillName string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.skills[strings.ToLower(strings.TrimSpace(skillName))]
	if !ok {
		return nil, false
	}
	return cloneSkill(item), true
}

func (r *Registry) CanUseTool(skillName, tool string) bool {
	item, ok := r.Get(skillName)
	if !ok {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(tool))
	for _, allowed := range item.Tools {
		if strings.ToLower(strings.TrimSpace(allowed)) == target {
			return true
		}
	}
	return false
}

func (r *Registry) Execute(ctx context.Context, skillName string) (string, error) {
	result, err := r.ExecuteWithParams(ctx, skillName, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", result["output"]), nil
}

func (r *Registry) ExecuteWithParams(ctx context.Context, skillName string, params map[string]any) (map[string]any, error) {
	item, ok := r.Get(skillName)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}
	if !item.Enabled {
		return nil, fmt.Errorf("skill disabled: %s", skillName)
	}
	if item.Execute == "" {
		return nil, fmt.Errorf("skill execute entry empty: %s", skillName)
	}
	scriptPath := item.Execute
	if item.SourcePath != "" {
		scriptPath = filepath.Join(filepath.Dir(item.SourcePath), item.Execute)
	}
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, r.execTimeout)
	defer cancel()
	envMap := map[string]string{
		"SKILL_NAME": item.SkillName,
	}
	if len(params) > 0 {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		envMap["SKILL_PARAMS"] = string(raw)
	}
	cmd, err := commandForScript(runCtx, absPath, envMap)
	if err != nil {
		return nil, err
	}
	cmd.Dir = filepath.Dir(absPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]any{
			"skill_name": item.SkillName,
			"output":     string(output),
		}, err
	}
	return map[string]any{
		"skill_name": item.SkillName,
		"output":     string(output),
	}, nil
}

func (r *Registry) watch(ctx context.Context) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.reload(ctx)
		}
	}
}

func (r *Registry) reload(ctx context.Context) error {
	loaded, err := r.loadFromDisk()
	if err != nil {
		return err
	}
	for _, item := range defaultBuiltins() {
		if _, ok := loaded[item.SkillName]; !ok {
			loaded[item.SkillName] = item
		}
	}
	items := make([]*Skill, 0, len(loaded))
	for _, item := range loaded {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SkillName < items[j].SkillName
	})

	r.mu.Lock()
	changed := changedWithCurrent(r.skills, loaded)
	if changed {
		r.skills = loaded
	}
	r.mu.Unlock()

	if changed && r.store != nil {
		upsertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := r.store.BatchUpsertSkills(upsertCtx, items)
		cancel()
		if err != nil && r.logger != nil {
			r.logger.Warn("[skills] batch upsert skill meta failed", "error", err)
		}
	}
	return nil
}

func (r *Registry) loadFromDisk() (map[string]*Skill, error) {
	result := make(map[string]*Skill)
	rootInfo, err := os.Stat(r.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	if !rootInfo.IsDir() {
		return result, nil
	}
	entries, err := os.ReadDir(r.rootDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(r.rootDir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}
		item, err := parseSkillFile(skillFile, entry.Name())
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("[skills] parse skill failed", "file", skillFile, "error", err)
			}
			continue
		}
		result[item.SkillName] = item
	}
	return result, nil
}

func defaultBuiltins() map[string]*Skill {
	return map[string]*Skill{
		"chat": {
			SkillName:   "chat",
			Description: "基础对话技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"llm_chat"},
			Memory: MemoryConfig{
				MemoryType:     "working",
				WindowSize:     5,
				CompressSwitch: true,
			},
		},
		"web_search": {
			SkillName:   "web_search",
			Description: "联网搜索技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"web_search", "http_request", "llm_chat"},
			Memory: MemoryConfig{
				MemoryType:     "working",
				WindowSize:     10,
				CompressSwitch: true,
			},
		},
		"report_gen": {
			SkillName:   "report_gen",
			Description: "报告生成技能",
			Version:     "1.0.0",
			Enabled:     true,
			Tools:       []string{"llm_chat", "file_write"},
			Memory: MemoryConfig{
				MemoryType:     "long_term",
				WindowSize:     20,
				CompressSwitch: true,
			},
		},
	}
}

func parseSkillFile(path string, dirName string) (*Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	item := &Skill{
		SkillName: strings.ToLower(strings.TrimSpace(dirName)),
		Version:   "1.0.0",
		Enabled:   true,
		Memory: MemoryConfig{
			MemoryType:     "working",
			WindowSize:     5,
			CompressSwitch: true,
		},
		SourcePath: path,
	}

	section := ""
	descBuf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(f)
	paramRegex := regexp.MustCompile(`^\s*-\s*([^:]+):\s*([^\(]+)\(([^)]*)\)\s*$`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# Skill:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "# Skill:"))
			if name != "" {
				item.SkillName = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}
		if line == "" {
			if section == "description" {
				descBuf.WriteString("\n")
			}
			continue
		}

		switch section {
		case "description":
			if descBuf.Len() > 0 {
				descBuf.WriteString(" ")
			}
			descBuf.WriteString(line)
		case "params":
			match := paramRegex.FindStringSubmatch(line)
			if len(match) == 4 {
				required := strings.Contains(match[3], "必填")
				defaultVal := extractDefaultValue(match[3])
				item.Params = append(item.Params, Param{
					Name:       strings.TrimSpace(match[1]),
					Type:       strings.TrimSpace(match[2]),
					Required:   required,
					DefaultVal: defaultVal,
				})
			}
		case "tools":
			tool := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if tool != "" {
				item.Tools = append(item.Tools, strings.ToLower(tool))
			}
		case "execute":
			item.Execute = strings.TrimSpace(strings.TrimPrefix(line, "-"))
			item.Execute = strings.TrimSpace(item.Execute)
		case "template":
			item.Template = strings.TrimSpace(strings.TrimPrefix(line, "-"))
			item.Template = strings.TrimSpace(item.Template)
		case "memory config":
			parseMemoryConfigLine(item, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	item.Description = strings.TrimSpace(descBuf.String())
	if item.SkillName == "" {
		return nil, fmt.Errorf("skill name empty: %s", path)
	}
	return item, nil
}

func parseMemoryConfigLine(item *Skill, line string) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	val := strings.TrimSpace(parts[1])
	switch key {
	case "memory_type":
		if val != "" {
			item.Memory.MemoryType = val
		}
	case "window_size":
		var n int
		_, _ = fmt.Sscanf(val, "%d", &n)
		if n > 0 {
			item.Memory.WindowSize = n
		}
	case "compress_switch":
		item.Memory.CompressSwitch = strings.EqualFold(val, "true")
	case "rag_collection":
		item.Memory.RAGCollection = val
	case "expire_time":
		item.Memory.ExpireTime = val
	}
}

func extractDefaultValue(raw string) string {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "默认值") {
			return strings.TrimSpace(strings.TrimPrefix(part, "默认值"))
		}
	}
	return ""
}

func changedWithCurrent(current map[string]*Skill, loaded map[string]*Skill) bool {
	if len(current) != len(loaded) {
		return true
	}
	for k, v := range current {
		other, ok := loaded[k]
		if !ok {
			return true
		}
		a, _ := json.Marshal(v)
		b, _ := json.Marshal(other)
		if !bytes.Equal(a, b) {
			return true
		}
	}
	return false
}

func cloneSkill(src *Skill) *Skill {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Params != nil {
		dst.Params = append([]Param(nil), src.Params...)
	}
	if src.Tools != nil {
		dst.Tools = append([]string(nil), src.Tools...)
	}
	return &dst
}

func commandForScript(ctx context.Context, path string, envMap map[string]string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py":
		cmd = exec.CommandContext(ctx, "python", path)
	case ".sh":
		cmd = exec.CommandContext(ctx, "bash", path)
	case ".ps1":
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", path)
	default:
		return nil, fmt.Errorf("unsupported execute script extension: %s", ext)
	}
	if len(envMap) > 0 {
		env := append([]string(nil), os.Environ()...)
		for k, v := range envMap {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	return cmd, nil
}
