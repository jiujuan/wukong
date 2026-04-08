package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/skills"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}

type MemoryStore interface {
	Write(ctx context.Context, namespace, key string, value map[string]any) error
	Read(ctx context.Context, namespace, key string) (map[string]any, bool, error)
}

type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string]map[string]map[string]any
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]map[string]map[string]any),
	}
}

func (s *InMemoryStore) Write(_ context.Context, namespace, key string, value map[string]any) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("namespace or key empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[namespace]; !ok {
		s.data[namespace] = make(map[string]map[string]any)
	}
	cp := make(map[string]any, len(value))
	for k, v := range value {
		cp[k] = v
	}
	s.data[namespace][key] = cp
	return nil
}

func (s *InMemoryStore) Read(_ context.Context, namespace, key string) (map[string]any, bool, error) {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(key) == "" {
		return nil, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.data[namespace]
	if !ok {
		return nil, false, nil
	}
	item, ok := group[key]
	if !ok {
		return nil, false, nil
	}
	cp := make(map[string]any, len(item))
	for k, v := range item {
		cp[k] = v
	}
	return cp, true, nil
}

type Option func(*Manager)

type Manager struct {
	mu             sync.RWMutex
	tools          map[string]Tool
	logger         *pkglogger.Logger
	llmProvider    *llm.Provider
	skillsRegistry *skills.Registry
	memoryStore    MemoryStore
	baseDir        string
	httpClient     *http.Client
	execTimeout    time.Duration
}

func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = pkglogger.FromSlog(logger)
		}
	}
}

func WithLLMProvider(provider *llm.Provider) Option {
	return func(m *Manager) {
		m.llmProvider = provider
	}
}

func WithSkillsRegistry(registry *skills.Registry) Option {
	return func(m *Manager) {
		m.skillsRegistry = registry
	}
}

func WithMemoryStore(store MemoryStore) Option {
	return func(m *Manager) {
		if store != nil {
			m.memoryStore = store
		}
	}
}

func WithBaseDir(baseDir string) Option {
	return func(m *Manager) {
		if strings.TrimSpace(baseDir) != "" {
			m.baseDir = baseDir
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(m *Manager) {
		if client != nil {
			m.httpClient = client
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		if timeout > 0 {
			m.execTimeout = timeout
		}
	}
}

func NewManager(opts ...Option) *Manager {
	m := &Manager{
		tools:       make(map[string]Tool),
		logger:      pkglogger.FromSlog(slog.Default()),
		memoryStore: NewInMemoryStore(),
		baseDir:     ".",
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
		"exec_timeout", m.execTimeout,
	)
	return m
}

func (m *Manager) Register(tool Tool) {
	if tool == nil {
		m.logger.Warn("[ToolManager] skip register: tool is nil")
		return
	}
	key := strings.ToLower(strings.TrimSpace(tool.Name()))
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
	key := strings.ToLower(strings.TrimSpace(name))
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.tools[key]
	return item, ok
}

func (m *Manager) List() []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]map[string]string, 0, len(m.tools))
	for _, t := range m.tools {
		items = append(items, map[string]string{
			"name":        t.Name(),
			"description": t.Description(),
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
	normalizedSkill := strings.ToLower(strings.TrimSpace(skillName))
	normalizedTool := strings.ToLower(strings.TrimSpace(toolName))
	if m.skillsRegistry != nil {
		if _, exists := m.skillsRegistry.Get(normalizedSkill); !exists {
			m.logger.Warn("[ToolManager] skill not found, bypass tool policy and execute directly",
				"skill", normalizedSkill, "tool", normalizedTool,
			)
			return m.Execute(ctx, normalizedTool, params)
		}
		if !m.skillsRegistry.CanUseTool(normalizedSkill, normalizedTool) {
			m.logger.Warn("[ToolManager] execute blocked by policy",
				"skill", normalizedSkill, "tool", normalizedTool,
			)
			return nil, fmt.Errorf("tool %s is not allowed for skill %s", normalizedTool, normalizedSkill)
		}
	}
	m.logger.Info("[ToolManager] execute for skill",
		"skill", normalizedSkill, "tool", normalizedTool,
	)
	return m.Execute(ctx, normalizedTool, params)
}

func (m *Manager) registerBuiltins() {
	m.Register(&LLMTool{provider: m.llmProvider, logger: m.logger})
	m.Register(&WebSearchTool{client: m.httpClient, logger: m.logger})
	m.Register(&FileReadTool{baseDir: m.baseDir, logger: m.logger})
	m.Register(&FileWriteTool{baseDir: m.baseDir, logger: m.logger})
	m.Register(&HTTPTool{client: m.httpClient, logger: m.logger})
	m.Register(&CodeExecTool{timeout: m.execTimeout, logger: m.logger})
	m.Register(&MemoryReadTool{store: m.memoryStore, logger: m.logger})
	m.Register(&MemoryWriteTool{store: m.memoryStore, logger: m.logger})
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

type LLMTool struct {
	provider *llm.Provider
	logger   *pkglogger.Logger
}

func (t *LLMTool) Name() string { return "llm_chat" }

func (t *LLMTool) Description() string { return "调用 LLM 进行对话推理" }

func (t *LLMTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	if t.provider == nil {
		t.logger.Error("[Tool] llm_chat failed: provider is nil")
		return nil, fmt.Errorf("llm provider is nil")
	}
	t.logger.Info("[Tool] llm_chat start", "params_keys", mapKeys(params))
	messages := make([]llm.Message, 0, 4)
	if system := readString(params, "system"); strings.TrimSpace(system) != "" {
		messages = append(messages, llm.Message{Role: "system", Content: system})
	}
	if rawMessages, ok := params["messages"].([]map[string]any); ok && len(rawMessages) > 0 {
		for _, item := range rawMessages {
			role := readString(item, "role")
			content := readString(item, "content")
			if strings.TrimSpace(role) == "" || strings.TrimSpace(content) == "" {
				continue
			}
			messages = append(messages, llm.Message{Role: role, Content: content})
		}
	}
	if len(messages) == 0 {
		prompt := readString(params, "prompt", "query", "input")
		if strings.TrimSpace(prompt) == "" {
			t.logger.Warn("[Tool] llm_chat invalid params: prompt is empty")
			return nil, fmt.Errorf("prompt is required")
		}
		messages = append(messages, llm.Message{Role: "user", Content: prompt})
	}
	t.logger.Debug("[Tool] llm_chat request prepared", "message_count", len(messages))
	resp, err := t.provider.Chat(ctx, messages)
	if err != nil {
		t.logger.Error("[Tool] llm_chat provider call failed", "error", err)
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.logger.Error("[Tool] llm_chat empty response")
		return nil, fmt.Errorf("llm response is empty")
	}
	result := map[string]any{
		"output":            strings.TrimSpace(resp.Choices[0].Message.Content),
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
	}
	t.logger.Info("[Tool] llm_chat success",
		"model", resp.Model,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
	)
	return result, nil
}

type WebSearchTool struct {
	client *http.Client
	logger *pkglogger.Logger
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string { return "联网搜索并返回结构化结果" }

func (t *WebSearchTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	query := readString(params, "query", "q", "keyword", "topic")
	if strings.TrimSpace(query) == "" {
		t.logger.Warn("[Tool] web_search invalid params: query is empty")
		return nil, fmt.Errorf("query is required")
	}
	t.logger.Info("[Tool] web_search start", "query", query)
	if t.client == nil {
		t.client = &http.Client{Timeout: 15 * time.Second}
	}
	api := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", api, nil)
	if err != nil {
		t.logger.Error("[Tool] web_search build request failed", "error", err)
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.Error("[Tool] web_search request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Error("[Tool] web_search read response failed", "error", err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.logger.Error("[Tool] web_search bad response", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("search status=%d body=%s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.logger.Error("[Tool] web_search parse response failed", "error", err)
		return nil, err
	}
	related := make([]map[string]any, 0, 5)
	if raw, ok := payload["RelatedTopics"].([]any); ok {
		for _, item := range raw {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text := readString(obj, "Text")
			firstURL := readString(obj, "FirstURL")
			if strings.TrimSpace(text) == "" && strings.TrimSpace(firstURL) == "" {
				continue
			}
			related = append(related, map[string]any{
				"text":       text,
				"url":        firstURL,
				"source":     "duckduckgo",
				"confidence": "medium",
			})
			if len(related) >= 5 {
				break
			}
		}
	}
	result := map[string]any{
		"query":    query,
		"heading":  readString(payload, "Heading"),
		"abstract": readString(payload, "AbstractText"),
		"results":  related,
	}
	t.logger.Info("[Tool] web_search success", "query", query, "result_count", len(related))
	return result, nil
}

type FileReadTool struct {
	baseDir string
	logger  *pkglogger.Logger
}

func (t *FileReadTool) Name() string { return "file_read" }

func (t *FileReadTool) Description() string { return "读取本地文件内容" }

func (t *FileReadTool) Execute(_ context.Context, params map[string]any) (map[string]any, error) {
	path := readString(params, "path")
	if strings.TrimSpace(path) == "" {
		t.logger.Warn("[Tool] file_read invalid params: path is empty")
		return nil, fmt.Errorf("path is required")
	}
	t.logger.Info("[Tool] file_read start", "path", path)
	target, err := resolvePath(t.baseDir, path)
	if err != nil {
		t.logger.Error("[Tool] file_read resolve path failed", "path", path, "error", err)
		return nil, err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.logger.Error("[Tool] file_read read file failed", "path", target, "error", err)
		return nil, err
	}
	result := map[string]any{
		"path":    target,
		"content": string(content),
		"size":    len(content),
	}
	t.logger.Info("[Tool] file_read success", "path", target, "size", len(content))
	return result, nil
}

type FileWriteTool struct {
	baseDir string
	logger  *pkglogger.Logger
}

func (t *FileWriteTool) Name() string { return "file_write" }

func (t *FileWriteTool) Description() string { return "写入本地文件内容" }

func (t *FileWriteTool) Execute(_ context.Context, params map[string]any) (map[string]any, error) {
	path := readString(params, "path")
	content := readString(params, "content")
	if strings.TrimSpace(path) == "" {
		t.logger.Warn("[Tool] file_write invalid params: path is empty")
		return nil, fmt.Errorf("path is required")
	}
	t.logger.Info("[Tool] file_write start", "path", path, "content_length", len(content))
	target, err := resolvePath(t.baseDir, path)
	if err != nil {
		t.logger.Error("[Tool] file_write resolve path failed", "path", path, "error", err)
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.logger.Error("[Tool] file_write mkdir failed", "path", target, "error", err)
		return nil, err
	}
	appendMode, _ := params["append"].(bool)
	var f *os.File
	if appendMode {
		f, err = os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	} else {
		f, err = os.Create(target)
	}
	if err != nil {
		t.logger.Error("[Tool] file_write open file failed", "path", target, "append", appendMode, "error", err)
		return nil, err
	}
	defer f.Close()
	n, err := f.WriteString(content)
	if err != nil {
		t.logger.Error("[Tool] file_write write failed", "path", target, "error", err)
		return nil, err
	}
	result := map[string]any{
		"path":          target,
		"written_bytes": n,
		"append":        appendMode,
	}
	t.logger.Info("[Tool] file_write success", "path", target, "written_bytes", n, "append", appendMode)
	return result, nil
}

type HTTPTool struct {
	client *http.Client
	logger *pkglogger.Logger
}

func (t *HTTPTool) Name() string { return "http_request" }

func (t *HTTPTool) Description() string { return "发起外部 HTTP 请求" }

func (t *HTTPTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(readString(params, "method")))
	if method == "" {
		method = "GET"
	}
	rawURL := readString(params, "url")
	if strings.TrimSpace(rawURL) == "" {
		t.logger.Warn("[Tool] http_request invalid params: url is empty")
		return nil, fmt.Errorf("url is required")
	}
	t.logger.Info("[Tool] http_request start", "method", method, "url", rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.logger.Error("[Tool] http_request parse url failed", "url", rawURL, "error", err)
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		t.logger.Warn("[Tool] http_request unsupported scheme", "scheme", parsed.Scheme, "url", rawURL)
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}
	body := readString(params, "body")
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(body))
	if err != nil {
		t.logger.Error("[Tool] http_request build request failed", "error", err)
		return nil, err
	}
	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	client := t.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.logger.Error("[Tool] http_request do request failed", "method", method, "url", rawURL, "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Error("[Tool] http_request read response failed", "method", method, "url", rawURL, "error", err)
		return nil, err
	}
	result := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}
	t.logger.Info("[Tool] http_request success", "method", method, "url", rawURL, "status_code", resp.StatusCode)
	return result, nil
}

type CodeExecTool struct {
	timeout time.Duration
	logger  *pkglogger.Logger
}

func (t *CodeExecTool) Name() string { return "code_exec" }

func (t *CodeExecTool) Description() string { return "执行代码片段" }

func (t *CodeExecTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	language := strings.ToLower(strings.TrimSpace(readString(params, "language", "lang")))
	code := readString(params, "code")
	if language == "" || strings.TrimSpace(code) == "" {
		t.logger.Warn("[Tool] code_exec invalid params", "language", language, "has_code", strings.TrimSpace(code) != "")
		return nil, fmt.Errorf("language and code are required")
	}
	timeout := t.timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	t.logger.Info("[Tool] code_exec start", "language", language, "timeout", timeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "wukong-tool-*"+suffixByLanguage(language))
	if err != nil {
		t.logger.Error("[Tool] code_exec create temp file failed", "language", language, "error", err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		t.logger.Error("[Tool] code_exec write temp file failed", "file", tmpFile.Name(), "error", err)
		return nil, err
	}
	tmpFile.Close()

	cmd, err := commandByLanguage(runCtx, language, tmpFile.Name())
	if err != nil {
		t.logger.Error("[Tool] code_exec build command failed", "language", language, "error", err)
		return nil, err
	}
	output, err := cmd.CombinedOutput()
	result := map[string]any{
		"language": language,
		"output":   string(output),
	}
	if err != nil {
		result["error"] = err.Error()
		t.logger.Error("[Tool] code_exec failed", "language", language, "error", err)
		return result, err
	}
	t.logger.Info("[Tool] code_exec success", "language", language, "output_length", len(output))
	return result, nil
}

type MemoryReadTool struct {
	store  MemoryStore
	logger *pkglogger.Logger
}

func (t *MemoryReadTool) Name() string { return "memory_read" }

func (t *MemoryReadTool) Description() string { return "读取记忆内容" }

func (t *MemoryReadTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	if t.store == nil {
		t.logger.Error("[Tool] memory_read failed: store is nil")
		return nil, fmt.Errorf("memory store is nil")
	}
	namespace := readString(params, "namespace", "scope")
	if strings.TrimSpace(namespace) == "" {
		namespace = "default"
	}
	key := readString(params, "key")
	if strings.TrimSpace(key) == "" {
		t.logger.Warn("[Tool] memory_read invalid params: key is empty")
		return nil, fmt.Errorf("key is required")
	}
	t.logger.Info("[Tool] memory_read start", "namespace", namespace, "key", key)
	value, ok, err := t.store.Read(ctx, namespace, key)
	if err != nil {
		t.logger.Error("[Tool] memory_read store read failed", "namespace", namespace, "key", key, "error", err)
		return nil, err
	}
	result := map[string]any{
		"namespace": namespace,
		"key":       key,
		"found":     ok,
		"value":     value,
	}
	t.logger.Info("[Tool] memory_read success", "namespace", namespace, "key", key, "found", ok)
	return result, nil
}

type MemoryWriteTool struct {
	store  MemoryStore
	logger *pkglogger.Logger
}

func (t *MemoryWriteTool) Name() string { return "memory_write" }

func (t *MemoryWriteTool) Description() string { return "写入记忆内容" }

func (t *MemoryWriteTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	if t.store == nil {
		t.logger.Error("[Tool] memory_write failed: store is nil")
		return nil, fmt.Errorf("memory store is nil")
	}
	namespace := readString(params, "namespace", "scope")
	if strings.TrimSpace(namespace) == "" {
		namespace = "default"
	}
	key := readString(params, "key")
	if strings.TrimSpace(key) == "" {
		t.logger.Warn("[Tool] memory_write invalid params: key is empty")
		return nil, fmt.Errorf("key is required")
	}
	value, ok := params["value"].(map[string]any)
	if !ok {
		raw, ok := params["value"]
		if !ok {
			t.logger.Warn("[Tool] memory_write invalid params: value is empty")
			return nil, fmt.Errorf("value is required")
		}
		value = map[string]any{"data": raw}
	}
	t.logger.Info("[Tool] memory_write start", "namespace", namespace, "key", key, "value_keys", mapKeys(value))
	if err := t.store.Write(ctx, namespace, key, value); err != nil {
		t.logger.Error("[Tool] memory_write store write failed", "namespace", namespace, "key", key, "error", err)
		return nil, err
	}
	result := map[string]any{
		"namespace": namespace,
		"key":       key,
		"written":   true,
	}
	t.logger.Info("[Tool] memory_write success", "namespace", namespace, "key", key)
	return result, nil
}

func readString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		if source == nil {
			continue
		}
		v, ok := source[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		default:
			text := fmt.Sprintf("%v", value)
			if strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}

func resolvePath(baseDir, target string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	targetPath := target
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(baseAbs, targetPath)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	if targetAbs != baseAbs && !strings.HasPrefix(strings.ToLower(targetAbs), strings.ToLower(baseAbs)+string(filepath.Separator)) {
		return "", fmt.Errorf("path out of base dir")
	}
	return targetAbs, nil
}

func suffixByLanguage(language string) string {
	switch language {
	case "python", "py":
		return ".py"
	case "javascript", "js", "node":
		return ".js"
	case "bash", "sh":
		return ".sh"
	case "powershell", "ps1":
		return ".ps1"
	default:
		return ".txt"
	}
}

func commandByLanguage(ctx context.Context, language, filename string) (*exec.Cmd, error) {
	switch language {
	case "python", "py":
		return exec.CommandContext(ctx, "python", filename), nil
	case "javascript", "js", "node":
		return exec.CommandContext(ctx, "node", filename), nil
	case "bash", "sh":
		return exec.CommandContext(ctx, "bash", filename), nil
	case "powershell", "ps1":
		return exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", filename), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}
