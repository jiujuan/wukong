package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/service"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/response"
)

type SkillHandler struct {
	skillService *service.SkillService
}

func NewSkillHandler(skillService *service.SkillService) *SkillHandler {
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
