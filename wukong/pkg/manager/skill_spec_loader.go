package manager

import (
	"context"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/skills"
)

type registrySkillSpecLoader struct {
	registry *skills.Registry
}

func (l *registrySkillSpecLoader) LoadSkillSpec(ctx context.Context, skillName string) (*ctxengine.SkillSpec, error) {
	if l == nil || l.registry == nil {
		return nil, nil
	}
	item, ok := l.registry.Get(skillName)
	if !ok || item == nil {
		return nil, nil
	}
	params := make([]ctxengine.SkillParam, 0, len(item.Params))
	for _, param := range item.Params {
		params = append(params, ctxengine.SkillParam{
			Name:       param.Name,
			Type:       param.Type,
			Required:   param.Required,
			DefaultVal: param.DefaultVal,
		})
	}
	canon := item.Canonical()
	return &ctxengine.SkillSpec{
		SkillName:      item.SkillName,
		Description:    item.Description,
		Version:        item.Version,
		Enabled:        item.Enabled,
		SourceType:     canon.Source.Type,
		RootDir:        canon.Source.RootDir,
		Runtime:        canon.Runtime.Runtime,
		Entry:          canon.Runtime.Entry,
		Tools:          append([]string(nil), item.Tools...),
		Params:         params,
		MemoryType:     item.Memory.MemoryType,
		MemoryWindow:   item.Memory.WindowSize,
		MemoryCompress: item.Memory.CompressSwitch,
		References:     append([]string(nil), item.References...),
		Assets:         append([]string(nil), item.Assets...),
		Metadata:       cloneAnyMap(item.Metadata),
	}, nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
