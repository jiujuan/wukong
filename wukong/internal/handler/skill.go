package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/response"
)

type SkillHandler struct {
	skillService SkillService
}

func NewSkillHandler(skillService SkillService) *SkillHandler {
	return &SkillHandler{skillService: skillService}
}

func (h *SkillHandler) ListSkills(c *gin.Context) {
	if h.skillService == nil {
		response.Fail(c, errors.CodeServerError, "技能服务未初始化")
		return
	}
	items, err := h.skillService.ListSkills(c.Request.Context())
	if err != nil {
		response.Fail(c, errors.CodeServerError, "查询技能列表失败")
		return
	}
	response.Success(c, gin.H{
		"list":  items,
		"total": len(items),
	})
}

func (h *SkillHandler) Detail(c *gin.Context) {
	if h.skillService == nil {
		response.Fail(c, errors.CodeServerError, "skill service not initialized")
		return
	}
	skillName := strings.TrimSpace(c.Query("skill_name"))
	if skillName == "" {
		response.Fail(c, errors.CodeBadRequest, "skill_name is required")
		return
	}
	item, err := h.skillService.GetSkill(c.Request.Context(), skillName)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "query skill detail failed")
		return
	}
	response.Success(c, item)
}

type UpdateSkillReq struct {
	SkillName      string `json:"skill_name" binding:"required"`
	Description    string `json:"description"`
	Version        string `json:"version"`
	Enabled        bool   `json:"enabled"`
	MemoryType     string `json:"memory_type"`
	MemoryWindow   int    `json:"memory_window"`
	MemoryCompress bool   `json:"memory_compress"`
}

func (h *SkillHandler) UpdateSkill(c *gin.Context) {
	if h.skillService == nil {
		response.Fail(c, errors.CodeServerError, "skill service not initialized")
		return
	}
	var req UpdateSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "invalid params")
		return
	}
	item := &model.SkillMeta{
		SkillName:      strings.TrimSpace(req.SkillName),
		Description:    req.Description,
		Version:        req.Version,
		Enabled:        req.Enabled,
		MemoryType:     req.MemoryType,
		MemoryWindow:   req.MemoryWindow,
		MemoryCompress: req.MemoryCompress,
	}
	if err := h.skillService.UpdateSkill(c.Request.Context(), item); err != nil {
		response.Fail(c, errors.CodeServerError, "update skill failed")
		return
	}
	response.Success(c, gin.H{"skill_name": item.SkillName, "updated": true})
}
