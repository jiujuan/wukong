package handler

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/middleware"
	"github.com/jiujuan/wukong/internal/service"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/response"
)

// ChatHandler 对话处理器
type ChatHandler struct {
	chatService *service.ChatService
}

// NewChatHandler 新建对话处理器
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// CreateSessionReq 创建会话请求
type CreateSessionReq struct {
	Title string `json:"title"`
	Scene string `json:"scene"`
}

// CreateSession 创建会话
func (h *ChatHandler) CreateSession(c *gin.Context) {
	if h.chatService == nil {
		response.Fail(c, errors.CodeServerError, "会话服务未初始化")
		return
	}
	var req CreateSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Fail(c, errors.CodeUnauthorized, "未授权")
		return
	}

	session, err := h.chatService.CreateSession(c.Request.Context(), userID, req.Title, req.Scene)
	if err != nil {
		requestID := c.GetString("RequestID")
		slog.Error("create chat session failed", "request_id", requestID, "user_id", userID, "error", err)
		response.Fail(c, errors.CodeServerError, "创建会话失败: "+requestID)
		return
	}
	response.Success(c, gin.H{
		"session_id": session.SessionID,
		"title":      session.Title,
		"scene":      session.Scene,
		"status":     session.Status,
		"created_at": session.CreatedAt,
	})
}

// GetSessionList 获取会话列表
func (h *ChatHandler) GetSessionList(c *gin.Context) {
	if h.chatService == nil {
		response.Fail(c, errors.CodeServerError, "会话服务未初始化")
		return
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Fail(c, errors.CodeUnauthorized, "未授权")
		return
	}
	page, size := parsePageSize(c, 1, 20, 100)
	sessions, total, err := h.chatService.ListSessions(c.Request.Context(), userID, page, size)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "查询会话列表失败")
		return
	}
	list := make([]gin.H, 0, len(sessions))
	for _, item := range sessions {
		if item == nil {
			continue
		}
		list = append(list, gin.H{
			"session_id": item.SessionID,
			"title":      item.Title,
			"scene":      item.Scene,
			"status":     item.Status,
			"created_at": item.CreatedAt,
		})
	}
	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
		"pages": calcPages(total, size),
	})
}

// SendMessageReq 发送消息请求
type SendMessageReq struct {
	SessionID    string `json:"session_id"`
	SessionIDAlt string `json:"sessionId"`
	Content      string `json:"content" binding:"required"`
	SkillName    string `json:"skill_name"`
}

type DeleteSessionReq struct {
	SessionID    string `json:"session_id"`
	SessionIDAlt string `json:"sessionId"`
}

// SendMessage 发送消息
func (h *ChatHandler) SendMessage(c *gin.Context) {
	if h.chatService == nil {
		response.Fail(c, errors.CodeServerError, "会话服务未初始化")
		return
	}
	var req SendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = strings.TrimSpace(req.SessionIDAlt)
	}
	if req.SessionID == "" {
		response.Fail(c, errors.CodeBadRequest, "session_id不能为空")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		response.Fail(c, errors.CodeBadRequest, "content不能为空")
		return
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Fail(c, errors.CodeUnauthorized, "未授权")
		return
	}

	assistantMsg, err := h.chatService.SendMessage(c.Request.Context(), userID, req.SessionID, req.Content, req.SkillName)
	if err == errors.ErrSessionNotFound {
		response.Fail(c, errors.CodeSessionNot, "会话不存在")
		return
	}
	if err != nil {
		response.Fail(c, errors.CodeMsgSendFail, "消息发送失败")
		return
	}
	response.Success(c, gin.H{
		"msg_id":       assistantMsg.MsgID,
		"session_id":   assistantMsg.SessionID,
		"content":      assistantMsg.Content,
		"role":         assistantMsg.Role,
		"content_type": assistantMsg.ContentType,
		"created_at":   assistantMsg.CreatedAt,
	})
}

// GetMessageList 获取消息列表
func (h *ChatHandler) GetMessageList(c *gin.Context) {
	if h.chatService == nil {
		response.Fail(c, errors.CodeServerError, "会话服务未初始化")
		return
	}
	sessionID := c.Query("session_id")
	if sessionID == "" {
		sessionID = c.Query("sessionId")
	}
	if sessionID == "" {
		response.Fail(c, errors.CodeBadRequest, "session_id不能为空")
		return
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Fail(c, errors.CodeUnauthorized, "未授权")
		return
	}
	page, size := parsePageSize(c, 1, 50, 200)
	messages, total, err := h.chatService.ListMessages(c.Request.Context(), userID, sessionID, page, size)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "查询消息列表失败")
		return
	}
	list := make([]gin.H, 0, len(messages))
	for _, item := range messages {
		if item == nil {
			continue
		}
		list = append(list, gin.H{
			"msg_id":       item.MsgID,
			"session_id":   item.SessionID,
			"role":         item.Role,
			"content":      item.Content,
			"content_type": item.ContentType,
			"seq":          item.Seq,
			"created_at":   item.CreatedAt,
		})
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
		"pages": calcPages(total, size),
	})
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	if h.chatService == nil {
		response.Fail(c, errors.CodeServerError, "会话服务未初始化")
		return
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Fail(c, errors.CodeUnauthorized, "未授权")
		return
	}
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Query("sessionId"))
	}
	if sessionID == "" {
		var req DeleteSessionReq
		if err := c.ShouldBindJSON(&req); err == nil {
			if strings.TrimSpace(req.SessionID) != "" {
				sessionID = strings.TrimSpace(req.SessionID)
			} else {
				sessionID = strings.TrimSpace(req.SessionIDAlt)
			}
		}
	}
	if sessionID == "" {
		response.Fail(c, errors.CodeBadRequest, "session_id不能为空")
		return
	}
	deleted, err := h.chatService.DeleteSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		response.Fail(c, errors.CodeServerError, "删除会话失败")
		return
	}
	if !deleted {
		response.Fail(c, errors.CodeSessionNot, "会话不存在")
		return
	}
	response.Success(c, gin.H{
		"session_id": sessionID,
		"deleted":    true,
	})
}

func parsePageSize(c *gin.Context, defaultPage, defaultSize, maxSize int) (int, int) {
	page, err := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultPage)))
	if err != nil || page < 1 {
		page = defaultPage
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", strconv.Itoa(defaultSize)))
	if err != nil || size < 1 {
		size = defaultSize
	}
	if size > maxSize {
		size = maxSize
	}
	return page, size
}

func calcPages(total int64, size int) int {
	if size <= 0 {
		return 1
	}
	pages := int((total + int64(size) - 1) / int64(size))
	if pages <= 0 {
		return 1
	}
	return pages
}
