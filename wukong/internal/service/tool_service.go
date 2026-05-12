package service

import (
	"context"

	"github.com/jiujuan/wukong/pkg/tool"
)

type toolLister interface {
	List() []tool.ToolInfo
}

type ToolService struct {
	manager toolLister
}

func NewToolService(manager toolLister) *ToolService {
	return &ToolService{manager: manager}
}

func (s *ToolService) ListTools(_ context.Context) []tool.ToolInfo {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.List()
}
