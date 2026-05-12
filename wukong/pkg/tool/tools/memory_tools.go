package tools

import (
	"context"
	"fmt"
	"strings"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type MemoryReadTool struct {
	store  MemoryStore
	logger *pkglogger.Logger
}

func NewMemoryReadTool(store MemoryStore, logger *pkglogger.Logger) *MemoryReadTool {
	return &MemoryReadTool{store: store, logger: logger}
}

func (t *MemoryReadTool) Name() string { return "memory_read" }

func (t *MemoryReadTool) Description() string { return "读取记忆内容" }

func (t *MemoryReadTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("namespace", "string", false, "memory namespace", "default", "default"),
		schemaItem("scope", "string", false, "alias for namespace", "default", "default"),
		schemaItem("key", "string", true, "memory key", nil, "summary"),
	}
}

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

func NewMemoryWriteTool(store MemoryStore, logger *pkglogger.Logger) *MemoryWriteTool {
	return &MemoryWriteTool{store: store, logger: logger}
}

func (t *MemoryWriteTool) Name() string { return "memory_write" }

func (t *MemoryWriteTool) Description() string { return "写入记忆内容" }

func (t *MemoryWriteTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("namespace", "string", false, "memory namespace", "default", "default"),
		schemaItem("scope", "string", false, "alias for namespace", "default", "default"),
		schemaItem("key", "string", true, "memory key", nil, "summary"),
		schemaItem("value", "object|any", true, "memory payload", nil, map[string]any{"summary": "hello"}),
	}
}

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
