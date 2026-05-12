package promptbuilder

import (
	"strings"
	"sync"
)

type Registry struct {
	mu              sync.RWMutex
	sceneTemplates  map[string]string
	sceneAssemblers map[string]SceneAssembler
}

func NewRegistry() *Registry {
	return &Registry{
		sceneTemplates:  make(map[string]string),
		sceneAssemblers: make(map[string]SceneAssembler),
	}
}

func (r *Registry) BindSceneTemplate(scene, templateKey string) {
	if r == nil {
		return
	}
	scene = strings.TrimSpace(scene)
	templateKey = strings.TrimSpace(templateKey)
	if scene == "" || templateKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sceneTemplates[scene] = templateKey
}

func (r *Registry) ResolveTemplate(scene string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, ok := r.sceneTemplates[strings.TrimSpace(scene)]
	return key, ok
}

func (r *Registry) RegisterAssembler(scene string, assembler SceneAssembler) {
	if r == nil || assembler == nil {
		return
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sceneAssemblers[scene] = assembler
}

func (r *Registry) GetAssembler(scene string) SceneAssembler {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sceneAssemblers[strings.TrimSpace(scene)]
}
