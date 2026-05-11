package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/pkg/response"
)

type ToolHandler struct {
	toolService ToolService
}

func NewToolHandler(toolService ToolService) *ToolHandler {
	return &ToolHandler{toolService: toolService}
}

func (h *ToolHandler) ListTools(c *gin.Context) {
	items := h.toolService.ListTools(c.Request.Context())
	response.Success(c, gin.H{
		"list":  items,
		"total": len(items),
	})
}
