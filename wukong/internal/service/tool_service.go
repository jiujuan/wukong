package service

import "context"

type toolLister interface {
	List() []map[string]string
}

type ToolService struct {
	manager toolLister
}

func NewToolService(manager toolLister) *ToolService {
	return &ToolService{manager: manager}
}

func (s *ToolService) ListTools(_ context.Context) []map[string]string {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.List()
}
