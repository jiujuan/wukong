package prompt

import (
	"fmt"
	"strings"
	"sync"
)

type Engine struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

func New() *Engine {
	return &Engine{
		templates: make(map[string]*Template),
	}
}

func NewDefaultEngine() *Engine {
	e := New()
	RegisterBuiltins(e)
	return e
}

func (e *Engine) Register(t *Template) error {
	if e == nil {
		return fmt.Errorf("prompt engine is nil")
	}
	if t == nil {
		return fmt.Errorf("template is nil")
	}
	key := strings.TrimSpace(t.Key)
	if key == "" {
		return fmt.Errorf("template key is empty")
	}
	if len(t.Messages) == 0 {
		return fmt.Errorf("template %q has no messages", key)
	}
	for i, msg := range t.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return fmt.Errorf("template %q message[%d] role is empty", key, i)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.templates[key]; exists {
		return fmt.Errorf("template %q already registered", key)
	}
	e.templates[key] = cloneTemplate(t)
	return nil
}

func (e *Engine) MustRegister(t *Template) {
	if err := e.Register(t); err != nil {
		panic(err)
	}
}

func (e *Engine) Get(key string) (*Template, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.templates[strings.TrimSpace(key)]
	if !ok {
		return nil, false
	}
	return cloneTemplate(t), true
}

func cloneTemplate(src *Template) *Template {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Messages != nil {
		dst.Messages = append([]MessageTemplate(nil), src.Messages...)
	}
	return &dst
}
