package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	short  *ShortMemoryStore
	long   *LongMemoryStore
	mu     sync.RWMutex
	shared map[string]*SharedMemory
}

func NewManager(shortStore *ShortMemoryStore, longStore *LongMemoryStore) *Manager {
	if shortStore == nil {
		shortStore = NewShortMemoryStore()
	}
	if longStore == nil {
		longStore = NewLongMemoryStore()
	}
	return &Manager{
		short:  shortStore,
		long:   longStore,
		shared: make(map[string]*SharedMemory),
	}
}

func (m *Manager) WriteMemory(ctx context.Context, namespace, key string, value map[string]any) error {
	return m.write(ctx, namespace, key, value, false)
}

func (m *Manager) UpdateMemory(ctx context.Context, namespace, key string, value map[string]any) error {
	return m.write(ctx, namespace, key, value, true)
}

func (m *Manager) DeleteMemory(ctx context.Context, namespace, key string) error {
	ns := normalizeNamespace(namespace)
	switch ns {
	case NamespaceWorking:
		m.short.Delete(key)
		return nil
	case NamespaceLong:
		m.long.Delete(key)
		return nil
	case NamespaceShared:
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.shared, key)
		return nil
	default:
		return fmt.Errorf("unsupported namespace: %s", ns)
	}
}

func (m *Manager) ReadMemory(ctx context.Context, namespace, key string) (map[string]any, bool, error) {
	ns := normalizeNamespace(namespace)
	switch ns {
	case NamespaceWorking:
		item, ok := m.short.Get(key)
		if !ok {
			return nil, false, nil
		}
		return workingToMap(item), true, nil
	case NamespaceLong:
		item, ok := m.long.Get(key)
		if !ok {
			return nil, false, nil
		}
		return longToMap(item), true, nil
	case NamespaceShared:
		m.mu.RLock()
		item, ok := m.shared[key]
		m.mu.RUnlock()
		if !ok {
			return nil, false, nil
		}
		return sharedToMap(item), true, nil
	default:
		return nil, false, fmt.Errorf("unsupported namespace: %s", ns)
	}
}

func (m *Manager) CompressMemory(ctx context.Context, taskID string) (string, error) {
	item, ok := m.short.Get(taskID)
	if !ok {
		return "", nil
	}
	if len(item.FullHistory) == 0 {
		return "", nil
	}
	window := item.WindowSize
	if window <= 0 {
		window = 5
	}
	if len(item.FullHistory) < window {
		window = len(item.FullHistory)
	}
	start := len(item.FullHistory) - window
	lines := make([]string, 0, window)
	for i := start; i < len(item.FullHistory); i++ {
		msg := item.FullHistory[i]
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, role+": "+strings.TrimSpace(msg.Content))
	}
	summary := strings.Join(lines, "\n")
	m.short.SetSummary(taskID, summary, true)
	return summary, nil
}

func (m *Manager) SessionArchive(ctx context.Context, taskID string, skillName string, topic string) (*LongTermMemory, error) {
	item, ok := m.short.Get(taskID)
	if !ok {
		return nil, nil
	}
	content := strings.TrimSpace(item.Summary)
	if content == "" {
		parts := make([]string, 0, len(item.FullHistory))
		for _, msg := range item.FullHistory {
			parts = append(parts, strings.TrimSpace(msg.Role)+": "+strings.TrimSpace(msg.Content))
		}
		content = strings.Join(parts, "\n")
	}
	if strings.TrimSpace(topic) == "" {
		topic = "task_" + taskID
	}
	mem := m.long.Create(item.UserID, skillName, topic, content, taskID)
	return mem, nil
}

func (m *Manager) SharedMemorySync(ctx context.Context, shareKey string, patch map[string]any) error {
	if strings.TrimSpace(shareKey) == "" {
		return fmt.Errorf("share key empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	item, ok := m.shared[shareKey]
	if !ok {
		item = &SharedMemory{
			ShareKey:  shareKey,
			Data:      map[string]any{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		m.shared[shareKey] = item
	}
	if item.ReadOnly {
		return fmt.Errorf("shared memory is read only")
	}
	for k, v := range patch {
		item.Data[k] = v
	}
	item.UpdatedAt = now
	return nil
}

func (m *Manager) MemoryExpire(ctx context.Context, now time.Time) (int, error) {
	total := m.short.DeleteExpired(now)
	m.mu.Lock()
	for key, item := range m.shared {
		if item.ExpireAt != nil && !item.ExpireAt.After(now) {
			delete(m.shared, key)
			total++
		}
	}
	m.mu.Unlock()
	return total, nil
}

func (m *Manager) Write(ctx context.Context, namespace, key string, value map[string]any) error {
	return m.WriteMemory(ctx, namespace, key, value)
}

func (m *Manager) Read(ctx context.Context, namespace, key string) (map[string]any, bool, error) {
	return m.ReadMemory(ctx, namespace, key)
}

func (m *Manager) ListWorking(ctx context.Context, userID string, taskID string, limit int) []*WorkingMemory {
	return m.short.List(userID, taskID, limit)
}

func (m *Manager) ListLong(ctx context.Context, userID string, skillName string, keyword string, limit int) []*LongTermMemory {
	return m.long.List(userID, skillName, keyword, limit)
}

func (m *Manager) write(ctx context.Context, namespace, key string, value map[string]any, merge bool) error {
	ns := normalizeNamespace(namespace)
	switch ns {
	case NamespaceWorking:
		return m.writeWorking(key, value)
	case NamespaceLong:
		return m.writeLong(key, value, merge)
	case NamespaceShared:
		return m.writeShared(key, value, merge)
	default:
		return fmt.Errorf("unsupported namespace: %s", ns)
	}
}

func (m *Manager) writeWorking(taskID string, value map[string]any) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("task id empty")
	}
	userID := readString(value, "user_id", "userId")
	windowSize := readInt(value, 5, "window_size", "windowSize")
	expireSec := readInt(value, 0, "expire_sec", "expireSec")
	var expireAt *time.Time
	if expireSec > 0 {
		t := time.Now().Add(time.Duration(expireSec) * time.Second)
		expireAt = &t
	}
	item := m.short.Upsert(taskID, userID, windowSize, expireAt)
	if summary := readString(value, "summary"); summary != "" {
		m.short.SetSummary(taskID, summary, true)
	}
	if history, ok := value["full_history"]; ok {
		if raw, ok := history.([]any); ok {
			for _, row := range raw {
				obj, ok := row.(map[string]any)
				if !ok {
					continue
				}
				msg := MemoryMessage{
					Role:    readString(obj, "role"),
					Content: readString(obj, "content"),
				}
				m.short.Append(taskID, msg)
			}
		}
	}
	if role := readString(value, "role"); role != "" {
		msg := MemoryMessage{
			Role:    role,
			Content: readString(value, "content"),
		}
		m.short.Append(taskID, msg)
	}
	if item != nil && item.WindowSize > 0 {
		nowItem, _ := m.short.Get(taskID)
		if nowItem != nil && len(nowItem.FullHistory) > nowItem.WindowSize {
			start := len(nowItem.FullHistory) - nowItem.WindowSize
			trimmed := nowItem.FullHistory[start:]
			m.short.mu.Lock()
			if origin, ok := m.short.items[taskID]; ok {
				origin.FullHistory = append([]MemoryMessage(nil), trimmed...)
				origin.UpdatedAt = time.Now()
			}
			m.short.mu.Unlock()
		}
	}
	return nil
}

func (m *Manager) writeLong(key string, value map[string]any, merge bool) error {
	if strings.TrimSpace(key) != "" {
		if existing, ok := m.long.Get(key); ok && merge {
			content := readString(value, "content")
			if content == "" {
				content = existing.Content
			}
			topic := readString(value, "topic")
			if topic == "" {
				topic = existing.Topic
			}
			userID := readString(value, "user_id", "userId")
			if userID == "" {
				userID = existing.UserID
			}
			skillName := readString(value, "skill_name", "skillName")
			if skillName == "" {
				skillName = existing.SkillName
			}
			sourceTaskID := readString(value, "source_task_id", "sourceTaskId")
			if sourceTaskID == "" {
				sourceTaskID = existing.SourceTaskID
			}
			m.long.mu.Lock()
			if item, ok := m.long.items[key]; ok {
				item.Content = content
				item.Topic = topic
				item.UserID = userID
				item.SkillName = strings.ToLower(strings.TrimSpace(skillName))
				item.SourceTaskID = sourceTaskID
			}
			m.long.mu.Unlock()
			return nil
		}
	}
	userID := readString(value, "user_id", "userId")
	skillName := readString(value, "skill_name", "skillName")
	topic := readString(value, "topic")
	content := readString(value, "content")
	sourceTaskID := readString(value, "source_task_id", "sourceTaskId")
	item := m.long.Create(userID, skillName, topic, content, sourceTaskID)
	if strings.TrimSpace(key) != "" && key != item.MemoryID {
		m.long.mu.Lock()
		delete(m.long.items, item.MemoryID)
		item.MemoryID = key
		m.long.items[key] = item
		m.long.mu.Unlock()
	}
	return nil
}

func (m *Manager) writeShared(shareKey string, value map[string]any, merge bool) error {
	if strings.TrimSpace(shareKey) == "" {
		return fmt.Errorf("share key empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	item, ok := m.shared[shareKey]
	if !ok {
		item = &SharedMemory{
			ShareKey:  shareKey,
			Data:      map[string]any{},
			CreatedAt: now,
		}
		m.shared[shareKey] = item
	}
	if !merge {
		item.Data = map[string]any{}
	}
	if data, ok := value["data"].(map[string]any); ok {
		for k, v := range data {
			item.Data[k] = v
		}
	} else {
		for k, v := range value {
			item.Data[k] = v
		}
	}
	item.OwnerTaskID = readString(value, "owner_task_id", "ownerTaskId")
	item.ReadOnly = readBool(value, item.ReadOnly, "read_only", "readOnly")
	expireSec := readInt(value, 0, "expire_sec", "expireSec")
	if expireSec > 0 {
		t := now.Add(time.Duration(expireSec) * time.Second)
		item.ExpireAt = &t
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	return nil
}

func normalizeNamespace(namespace string) string {
	ns := strings.ToLower(strings.TrimSpace(namespace))
	switch ns {
	case "", "working", "short", "short_term":
		return NamespaceWorking
	case "long", "long_term", "archive":
		return NamespaceLong
	case "shared", "share":
		return NamespaceShared
	default:
		return ns
	}
}

func readString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := data[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		default:
			raw, err := json.Marshal(value)
			if err == nil && len(raw) > 0 {
				text := strings.TrimSpace(string(raw))
				if text != "null" && text != "\"\"" {
					return text
				}
			}
		}
	}
	return ""
}

func readInt(data map[string]any, defaultValue int, keys ...string) int {
	for _, key := range keys {
		v, ok := data[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case int:
			return value
		case int64:
			return int(value)
		case float64:
			return int(value)
		case json.Number:
			if n, err := value.Int64(); err == nil {
				return int(n)
			}
		}
	}
	return defaultValue
}

func readBool(data map[string]any, defaultValue bool, keys ...string) bool {
	for _, key := range keys {
		v, ok := data[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case bool:
			return value
		}
	}
	return defaultValue
}

func workingToMap(item *WorkingMemory) map[string]any {
	return map[string]any{
		"task_id":       item.TaskID,
		"user_id":       item.UserID,
		"full_history":  item.FullHistory,
		"summary":       item.Summary,
		"window_size":   item.WindowSize,
		"compress_flag": item.CompressFlag,
		"created_at":    item.CreatedAt.Format(time.RFC3339),
		"updated_at":    item.UpdatedAt.Format(time.RFC3339),
	}
}

func longToMap(item *LongTermMemory) map[string]any {
	return map[string]any{
		"memory_id":      item.MemoryID,
		"user_id":        item.UserID,
		"skill_name":     item.SkillName,
		"topic":          item.Topic,
		"content":        item.Content,
		"source_task_id": item.SourceTaskID,
		"created_at":     item.CreatedAt.Format(time.RFC3339),
	}
}

func sharedToMap(item *SharedMemory) map[string]any {
	cp := make(map[string]any, len(item.Data))
	for k, v := range item.Data {
		cp[k] = v
	}
	result := map[string]any{
		"share_key":     item.ShareKey,
		"data":          cp,
		"owner_task_id": item.OwnerTaskID,
		"read_only":     item.ReadOnly,
		"created_at":    item.CreatedAt.Format(time.RFC3339),
		"updated_at":    item.UpdatedAt.Format(time.RFC3339),
	}
	if item.ExpireAt != nil {
		result["expire_at"] = item.ExpireAt.Format(time.RFC3339)
	}
	return result
}
