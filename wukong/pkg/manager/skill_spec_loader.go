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
	return &ctxengine.SkillSpec{
		SkillName:      item.SkillName,
		Description:    item.Description,
		Version:        item.Version,
		Enabled:        item.Enabled,
		Tools:          append([]string(nil), item.Tools...),
		Params:         params,
		MemoryType:     item.Memory.MemoryType,
		MemoryWindow:   item.Memory.WindowSize,
		MemoryCompress: item.Memory.CompressSwitch,
	}, nil
}
