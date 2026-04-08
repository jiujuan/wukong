package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/middleware"
	"github.com/jiujuan/wukong/internal/service"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/response"
	"github.com/jiujuan/wukong/pkg/uuid"
)

// TaskHandler 任务处理器
type TaskHandler struct {
	taskService *service.TaskService
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{taskService: taskService}
}

// CreateTaskReq 创建任务请求
type CreateTaskReq struct {
	SessionID string         `json:"session_id"`
	SkillName string         `json:"skill_name" binding:"required"`
	Params    map[string]any `json:"params"`
	Priority  int            `json:"priority"`
}

// CreateTask 创建任务
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}

	userID := middleware.GetUserID(c)

	task, err := h.taskService.CreateTask(c.Request.Context(), userID, req.SessionID, req.SkillName, req.Params, req.Priority)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "创建任务失败")
		return
	}

	response.Success(c, gin.H{
		"task_id":    task.TaskID,
		"skill_name": task.SkillName,
		"status":     task.Status,
		"priority":   task.Priority,
		"created_at": task.CreatedAt,
	})
}

// ListTasksReq 任务列表请求
type ListTasksReq struct {
	Page   int    `form:"page"`
	Size   int    `form:"size"`
	Status string `form:"status"`
}

// ListTasks 任务列表
func (h *TaskHandler) ListTasks(c *gin.Context) {
	var req ListTasksReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Size < 1 {
		req.Size = 10
	}
	if req.Size > 100 {
		req.Size = 100
	}

	userID := middleware.GetUserID(c)

	tasks, total, err := h.taskService.ListTasks(c.Request.Context(), userID, req.Status, req.Page, req.Size)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "获取任务列表失败")
		return
	}

	response.Success(c, response.NewPageResp(tasks, total, req.Page, req.Size))
}

// DetailReq 任务详情请求
type DetailReq struct {
	TaskID string `form:"task_id" binding:"required"`
}

// Detail 任务详情
func (h *TaskHandler) Detail(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		response.Fail(c, errors.CodeBadRequest, "task_id不能为空")
		return
	}

	task, err := h.taskService.GetTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, errors.CodeTaskNot, "任务不存在")
		return
	}

	// 获取子任务
	subtasks, _ := h.taskService.GetSubTasks(c.Request.Context(), taskID)

	response.Success(c, gin.H{
		"task":     task,
		"subtasks": subtasks,
	})
}

// CancelReq 取消任务请求
type CancelReq struct {
	TaskID string `json:"task_id" binding:"required"`
}

// Cancel 取消任务
func (h *TaskHandler) Cancel(c *gin.Context) {
	var req CancelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}

	if err := h.taskService.CancelTask(c.Request.Context(), req.TaskID); err != nil {
		response.Fail(c, errors.CodeServerError, "取消任务失败")
		return
	}

	response.Success(c, gin.H{
		"task_id": req.TaskID,
		"status":  "CANCELLED",
	})
}

// ListSubTasksReq 子任务列表请求
type ListSubTasksReq struct {
	TaskID string `form:"task_id" binding:"required"`
}

// ListSubTasks 子任务列表
func (h *TaskHandler) ListSubTasks(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		response.Fail(c, errors.CodeBadRequest, "task_id不能为空")
		return
	}

	subtasks, err := h.taskService.GetSubTasks(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "获取子任务列表失败")
		return
	}

	response.Success(c, gin.H{
		"list":  subtasks,
		"total": len(subtasks),
	})
}

// GenerateID 生成ID
func GenerateID(prefix string) string {
	switch prefix {
	case "task":
		return uuid.NewTaskID()
	case "sub":
		return uuid.NewSubTaskID()
	case "sess":
		return uuid.NewSessionID()
	case "msg":
		return uuid.NewMsgID()
	default:
		return uuid.New()
	}
}
