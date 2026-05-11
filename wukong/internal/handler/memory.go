package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/middleware"
	"github.com/jiujuan/wukong/pkg/response"
)

type MemoryHandler struct {
	memoryService MemoryService
}

func NewMemoryHandler(memoryService MemoryService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService}
}

type ListWorkingMemoryReq struct {
	TaskID string `form:"task_id"`
	Limit  int    `form:"limit"`
}

func (h *MemoryHandler) ListWorking(c *gin.Context) {
	if h.memoryService == nil {
		response.Fail(c, 500, "memory service unavailable")
		return
	}
	var req ListWorkingMemoryReq
	_ = c.ShouldBindQuery(&req)
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 200 {
		req.Limit = 200
	}
	userID := middleware.GetUserID(c)
	rows := h.memoryService.ListWorking(c.Request.Context(), userID, req.TaskID, req.Limit)
	response.Success(c, gin.H{
		"list":  rows,
		"total": len(rows),
	})
}

type ListLongMemoryReq struct {
	SkillName string `form:"skill_name"`
	Keyword   string `form:"keyword"`
	Limit     int    `form:"limit"`
}

func (h *MemoryHandler) ListLong(c *gin.Context) {
	if h.memoryService == nil {
		response.Fail(c, 500, "memory service unavailable")
		return
	}
	var req ListLongMemoryReq
	_ = c.ShouldBindQuery(&req)
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 200 {
		req.Limit = 200
	}
	userID := middleware.GetUserID(c)
	rows := h.memoryService.ListLong(c.Request.Context(), userID, req.SkillName, req.Keyword, req.Limit)
	response.Success(c, gin.H{
		"list":  rows,
		"total": len(rows),
	})
}
