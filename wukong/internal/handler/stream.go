package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/service"
	wsconn "github.com/jiujuan/wukong/pkg/websocket"
)

type StreamHandler struct {
	appService *service.StreamAppService
}

func NewStreamHandler(appService *service.StreamAppService) *StreamHandler {
	return &StreamHandler{
		appService: appService,
	}
}

func (h *StreamHandler) ChatSSE(c *gin.Context) {
	if h.appService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "stream service unavailable"})
		return
	}
	sessionID := strings.TrimSpace(c.Query("sessionId"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "sessionId不能为空"})
		return
	}
	lastSeq := resolveLastSeq(c)
	h.serveSSE(c, true, sessionID, lastSeq)
}

func (h *StreamHandler) TaskSSE(c *gin.Context) {
	if h.appService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "stream service unavailable"})
		return
	}
	taskID := strings.TrimSpace(c.Query("taskId"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "taskId不能为空"})
		return
	}
	lastSeq := resolveLastSeq(c)
	h.serveSSE(c, false, taskID, lastSeq)
}

func (h *StreamHandler) TaskWebSocket(c *gin.Context) {
	if h.appService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "stream service unavailable"})
		return
	}
	taskID := strings.TrimSpace(c.Query("taskId"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "taskId不能为空"})
		return
	}
	conn, err := wsconn.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		c.Abort()
		return
	}
	defer conn.Close()

	lastSeq := resolveLastSeq(c)
	backlog, streamCh, cancel := h.appService.SubscribeTask(c.Request.Context(), taskID, lastSeq)
	defer cancel()

	for _, item := range backlog {
		_ = conn.WriteText(marshalWSPayload(item))
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range streamCh {
			_ = conn.WriteText(marshalWSPayload(item))
		}
	}()

	for {
		payload, err := conn.ReadText()
		if err != nil {
			break
		}
		action, targetTaskID, content := parseWSCommand(payload)
		if targetTaskID == "" {
			targetTaskID = taskID
		}
		switch action {
		case "interrupt":
			h.appService.HandleTaskCommand(c.Request.Context(), taskID, action, targetTaskID, "")
		case "inject":
			h.appService.HandleTaskCommand(c.Request.Context(), taskID, action, targetTaskID, content)
		}
	}
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}
}

func (h *StreamHandler) serveSSE(c *gin.Context, isChat bool, id string, lastSeq int) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var backlog []*service.StreamMessage
	var streamCh <-chan *service.StreamMessage
	var cancel func()
	if isChat {
		var rows []*service.StreamMessage
		rows, streamCh, cancel = h.appService.SubscribeChat(c.Request.Context(), id, lastSeq)
		backlog = rows
	} else {
		var rows []*service.StreamMessage
		rows, streamCh, cancel = h.appService.SubscribeTask(c.Request.Context(), id, lastSeq)
		backlog = rows
	}
	defer cancel()

	for _, item := range backlog {
		if !writeSSE(c, item) {
			return
		}
	}

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := c.Writer.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
		case item, ok := <-streamCh:
			if !ok {
				return
			}
			if !writeSSE(c, item) {
				return
			}
			if item.MsgType == service.StreamTypeFinish {
				return
			}
		}
	}
}

func writeSSE(c *gin.Context, item *service.StreamMessage) bool {
	if item == nil {
		return true
	}
	body, _ := json.Marshal(gin.H{
		"seq":        item.Seq,
		"type":       item.MsgType,
		"content":    item.Content,
		"created_at": item.CreatedAt,
	})
	payload := fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", item.Seq, item.MsgType, string(body))
	if _, err := c.Writer.Write([]byte(payload)); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

func resolveLastSeq(c *gin.Context) int {
	if seq, ok := parseSeqValue(c.Query("last_seq")); ok {
		return seq
	}
	if seq, ok := parseSeqValue(c.GetHeader("Last-Event-ID")); ok {
		return seq
	}
	return 0
}

func parseSeqValue(raw string) (int, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, false
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func parseWSCommand(payload []byte) (action string, taskID string, content string) {
	cmd := map[string]any{}
	_ = json.Unmarshal(payload, &cmd)
	if v, ok := cmd["action"].(string); ok {
		action = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := cmd["task_id"].(string); ok {
		taskID = strings.TrimSpace(v)
	}
	if v, ok := cmd["content"].(string); ok {
		content = strings.TrimSpace(v)
	}
	return
}

func marshalWSPayload(item *service.StreamMessage) []byte {
	if item == nil {
		return []byte("{}")
	}
	body, _ := json.Marshal(gin.H{
		"seq":        item.Seq,
		"type":       item.MsgType,
		"content":    item.Content,
		"created_at": item.CreatedAt,
	})
	return body
}
