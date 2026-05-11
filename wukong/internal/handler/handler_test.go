package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/pkg/jwt"
	"github.com/jiujuan/wukong/pkg/manager"
	"github.com/jiujuan/wukong/pkg/memory"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type apiResponse struct {
	Code      int             `json:"code"`
	Msg       string          `json:"msg"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func newTestContext(t *testing.T, method, target string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var reader *bytes.Reader
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case []byte:
		reader = bytes.NewReader(v)
	case string:
		reader = bytes.NewReader([]byte(v))
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Set("RequestID", "req-test-1")
	c.Set("UserID", "user-123")
	c.Set("Username", "alice")
	return c, w
}

func decodeAPIResponse(t *testing.T, w *httptest.ResponseRecorder) apiResponse {
	t.Helper()
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func decodeData[T any](t *testing.T, raw json.RawMessage, out *T) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
}

type mockChatService struct {
	createSessionFn func(context.Context, string, string, string) (*model.ChatSession, error)
	listSessionsFn  func(context.Context, string, int, int) ([]*model.ChatSession, int64, error)
	sendMessageFn   func(context.Context, string, string, string, string) (*model.ChatMessage, error)
	listMessagesFn  func(context.Context, string, string, int, int) ([]*model.ChatMessage, int64, error)
	deleteSessionFn func(context.Context, string, string) (bool, error)
}

func (m *mockChatService) CreateSession(ctx context.Context, userID, title, scene string) (*model.ChatSession, error) {
	return m.createSessionFn(ctx, userID, title, scene)
}

func (m *mockChatService) ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error) {
	return m.listSessionsFn(ctx, userID, page, size)
}

func (m *mockChatService) SendMessage(ctx context.Context, userID, sessionID, content, skillName string) (*model.ChatMessage, error) {
	return m.sendMessageFn(ctx, userID, sessionID, content, skillName)
}

func (m *mockChatService) ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error) {
	return m.listMessagesFn(ctx, userID, sessionID, page, size)
}

func (m *mockChatService) DeleteSession(ctx context.Context, userID, sessionID string) (bool, error) {
	return m.deleteSessionFn(ctx, userID, sessionID)
}

type mockTaskService struct {
	createTaskFn  func(context.Context, string, string, string, map[string]any, int) (*manager.Task, error)
	listTasksFn   func(context.Context, string, string, int, int) ([]*manager.Task, int64, error)
	getTaskFn     func(context.Context, string) (*manager.Task, error)
	cancelTaskFn  func(context.Context, string) error
	getSubTasksFn func(context.Context, string) ([]*manager.SubTask, error)
}

func (m *mockTaskService) CreateTask(ctx context.Context, userID, sessionID, skillName string, params map[string]any, priority int) (*manager.Task, error) {
	return m.createTaskFn(ctx, userID, sessionID, skillName, params, priority)
}

func (m *mockTaskService) ListTasks(ctx context.Context, userID, status string, page, size int) ([]*manager.Task, int64, error) {
	return m.listTasksFn(ctx, userID, status, page, size)
}

func (m *mockTaskService) GetTask(ctx context.Context, taskID string) (*manager.Task, error) {
	return m.getTaskFn(ctx, taskID)
}

func (m *mockTaskService) CancelTask(ctx context.Context, taskID string) error {
	return m.cancelTaskFn(ctx, taskID)
}

func (m *mockTaskService) GetSubTasks(ctx context.Context, taskID string) ([]*manager.SubTask, error) {
	return m.getSubTasksFn(ctx, taskID)
}

type mockSkillService struct {
	listSkillsFn func(context.Context) ([]*model.SkillMeta, error)
}

func (m *mockSkillService) ListSkills(ctx context.Context) ([]*model.SkillMeta, error) {
	return m.listSkillsFn(ctx)
}

type mockMemoryService struct {
	listWorkingFn func(context.Context, string, string, int) []*memory.WorkingMemory
	listLongFn    func(context.Context, string, string, string, int) []*memory.LongTermMemory
}

func (m *mockMemoryService) ListWorking(ctx context.Context, userID string, taskID string, limit int) []*memory.WorkingMemory {
	return m.listWorkingFn(ctx, userID, taskID, limit)
}

func (m *mockMemoryService) ListLong(ctx context.Context, userID string, skillName string, keyword string, limit int) []*memory.LongTermMemory {
	return m.listLongFn(ctx, userID, skillName, keyword, limit)
}

type mockStreamAppService struct {
	chatBacklog []*StreamMessage
	chatCh      chan *StreamMessage
	taskBacklog []*StreamMessage
	taskCh      chan *StreamMessage

	chatLastSeq int
	taskLastSeq int

	handleCalls []struct {
		defaultTaskID string
		action        string
		taskID        string
		content       string
	}
}

func (m *mockStreamAppService) SubscribeChat(ctx context.Context, sessionID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func()) {
	m.chatLastSeq = lastSeq
	return m.chatBacklog, m.chatCh, func() {}
}

func (m *mockStreamAppService) SubscribeTask(ctx context.Context, taskID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func()) {
	m.taskLastSeq = lastSeq
	return m.taskBacklog, m.taskCh, func() {}
}

func (m *mockStreamAppService) HandleTaskCommand(ctx context.Context, defaultTaskID string, action string, taskID string, content string) {
	m.handleCalls = append(m.handleCalls, struct {
		defaultTaskID string
		action        string
		taskID        string
		content       string
	}{defaultTaskID: defaultTaskID, action: action, taskID: taskID, content: content})
}

func TestAuthHandler_LoginSuccess(t *testing.T) {
	h := NewAuthHandler(jwt.New(jwt.WithSecret("test-secret")))
	c, w := newTestContext(t, http.MethodPost, "/login", gin.H{
		"username": "admin",
		"password": "admin123",
	})

	h.Login(c)

	resp := decodeAPIResponse(t, w)
	if resp.Code != 200 || resp.RequestID != "req-test-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	var data map[string]any
	decodeData(t, resp.Data, &data)
	token, ok := data["access_token"].(string)
	if !ok || token == "" {
		t.Fatalf("missing access_token: %#v", data)
	}
	if _, err := jwt.New(jwt.WithSecret("test-secret")).Parse(token); err != nil {
		t.Fatalf("token parse failed: %v", err)
	}
}

func TestAuthHandler_LoginFailure(t *testing.T) {
	h := NewAuthHandler(jwt.New())
	c, w := newTestContext(t, http.MethodPost, "/login", gin.H{
		"username": "admin",
		"password": "wrong",
	})

	h.Login(c)

	resp := decodeAPIResponse(t, w)
	if resp.Code != 401 {
		t.Fatalf("code = %d, want 401", resp.Code)
	}
}

func TestAuthHandler_ProfileAndLogout(t *testing.T) {
	h := NewAuthHandler(jwt.New())

	_, w1 := newTestContext(t, http.MethodGet, "/profile", nil)
	c1, _ := gin.CreateTestContext(w1)
	req1 := httptest.NewRequest(http.MethodGet, "/profile", nil)
	c1.Request = req1
	c1.Set("RequestID", "req-test-1")
	c1.Set("UserID", "user-123")
	c1.Set("Username", "alice")
	h.Profile(c1)
	resp1 := decodeAPIResponse(t, w1)
	var profile map[string]any
	decodeData(t, resp1.Data, &profile)
	if profile["username"] != "alice" {
		t.Fatalf("profile username = %#v", profile["username"])
	}

	_, w2 := newTestContext(t, http.MethodPost, "/logout", nil)
	c2, _ := gin.CreateTestContext(w2)
	req2 := httptest.NewRequest(http.MethodPost, "/logout", nil)
	c2.Request = req2
	c2.Set("RequestID", "req-test-1")
	c2.Set("UserID", "user-123")
	h.Logout(c2)
	resp2 := decodeAPIResponse(t, w2)
	var logout map[string]any
	decodeData(t, resp2.Data, &logout)
	if logout["user_id"] != "user-123" {
		t.Fatalf("logout user_id = %#v", logout["user_id"])
	}
}

func TestChatHandler_CreateListSendDelete(t *testing.T) {
	mock := &mockChatService{
		createSessionFn: func(ctx context.Context, userID, title, scene string) (*model.ChatSession, error) {
			if userID != "user-123" || title != "hello" || scene != "CHAT" {
				t.Fatalf("unexpected create args: %s %s %s", userID, title, scene)
			}
			return &model.ChatSession{
				SessionID: "sess-1",
				Title:     title,
				Scene:     scene,
				Status:    "OPEN",
				CreatedAt: time.Unix(100, 0),
			}, nil
		},
		listSessionsFn: func(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error) {
			if page != 2 || size != 3 {
				t.Fatalf("unexpected list args: %d %d", page, size)
			}
			return []*model.ChatSession{
				nil,
				{SessionID: "sess-1", Title: "hello", Scene: "CHAT", Status: "OPEN", CreatedAt: time.Unix(100, 0)},
			}, 5, nil
		},
		sendMessageFn: func(ctx context.Context, userID, sessionID, content, skillName string) (*model.ChatMessage, error) {
			if sessionID != "sess-1" || content != "ping" || skillName != "skill-a" {
				t.Fatalf("unexpected send args: %s %s %s", sessionID, content, skillName)
			}
			return &model.ChatMessage{
				MsgID:       "msg-1",
				SessionID:   sessionID,
				Content:     "pong",
				Role:        "assistant",
				ContentType: "TEXT",
				CreatedAt:   time.Unix(101, 0),
			}, nil
		},
		listMessagesFn: func(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error) {
			return []*model.ChatMessage{
				{MsgID: "msg-1", SessionID: sessionID, Role: "user", Content: "ping", ContentType: "TEXT", Seq: 1, CreatedAt: time.Unix(101, 0)},
			}, 1, nil
		},
		deleteSessionFn: func(ctx context.Context, userID, sessionID string) (bool, error) {
			if sessionID != "sess-1" {
				t.Fatalf("unexpected delete session: %s", sessionID)
			}
			return true, nil
		},
	}
	h := NewChatHandler(mock)

	c1, w1 := newTestContext(t, http.MethodPost, "/chat/session", nil)
	c1.Request = httptest.NewRequest(http.MethodPost, "/chat/session", bytes.NewReader(mustJSON(t, gin.H{"title": "hello", "scene": "CHAT"})))
	c1.Request.Header.Set("Content-Type", "application/json")
	h.CreateSession(c1)
	resp1 := decodeAPIResponse(t, w1)
	var session map[string]any
	decodeData(t, resp1.Data, &session)
	if session["session_id"] != "sess-1" {
		t.Fatalf("session_id = %#v", session["session_id"])
	}

	c2, w2 := newTestContext(t, http.MethodGet, "/chat/sessions?page=2&size=3", nil)
	h.GetSessionList(c2)
	resp2 := decodeAPIResponse(t, w2)
	var sessions map[string]any
	decodeData(t, resp2.Data, &sessions)
	if sessions["total"].(float64) != 5 {
		t.Fatalf("total = %#v", sessions["total"])
	}

	c3, w3 := newTestContext(t, http.MethodPost, "/chat/send", gin.H{
		"sessionId":  "sess-1",
		"content":    "ping",
		"skill_name": "skill-a",
	})
	h.SendMessage(c3)
	resp3 := decodeAPIResponse(t, w3)
	var msg map[string]any
	decodeData(t, resp3.Data, &msg)
	if msg["msg_id"] != "msg-1" {
		t.Fatalf("msg_id = %#v", msg["msg_id"])
	}

	c4, w4 := newTestContext(t, http.MethodGet, "/chat/messages?session_id=sess-1&page=1&size=50", nil)
	h.GetMessageList(c4)
	resp4 := decodeAPIResponse(t, w4)
	var messages map[string]any
	decodeData(t, resp4.Data, &messages)
	if messages["total"].(float64) != 1 {
		t.Fatalf("messages total = %#v", messages["total"])
	}

	c5, w5 := newTestContext(t, http.MethodDelete, "/chat/session?sessionId=sess-1", nil)
	h.DeleteSession(c5)
	resp5 := decodeAPIResponse(t, w5)
	var deleted map[string]any
	decodeData(t, resp5.Data, &deleted)
	if deleted["deleted"] != true {
		t.Fatalf("deleted = %#v", deleted["deleted"])
	}
}

func TestTaskHandler_Flow(t *testing.T) {
	mock := &mockTaskService{
		createTaskFn: func(ctx context.Context, userID, sessionID, skillName string, params map[string]any, priority int) (*manager.Task, error) {
			if priority != 99 {
				t.Fatalf("priority = %d, want 99", priority)
			}
			return &manager.Task{
				TaskID:    "task-1",
				SkillName: skillName,
				Status:    "PENDING",
				Priority:  priority,
				CreatedAt: time.Unix(200, 0),
			}, nil
		},
		listTasksFn: func(ctx context.Context, userID, status string, page, size int) ([]*manager.Task, int64, error) {
			if page != 1 || size != 100 {
				t.Fatalf("unexpected page size: %d %d", page, size)
			}
			return []*manager.Task{{TaskID: "task-1", SkillName: "skill-a", Status: status, Priority: 9}}, 1, nil
		},
		getTaskFn: func(ctx context.Context, taskID string) (*manager.Task, error) {
			return &manager.Task{TaskID: taskID, SkillName: "skill-a", Status: "RUNNING"}, nil
		},
		cancelTaskFn: func(ctx context.Context, taskID string) error { return nil },
		getSubTasksFn: func(ctx context.Context, taskID string) ([]*manager.SubTask, error) {
			return []*manager.SubTask{{SubTaskID: "sub-1", TaskID: taskID, Action: "think"}}, nil
		},
	}
	h := NewTaskHandler(mock)

	c1, w1 := newTestContext(t, http.MethodPost, "/tasks", gin.H{
		"skill_name": "skill-a",
		"priority":   99,
	})
	h.CreateTask(c1)
	resp1 := decodeAPIResponse(t, w1)
	var task map[string]any
	decodeData(t, resp1.Data, &task)
	if task["task_id"] != "task-1" {
		t.Fatalf("task_id = %#v", task["task_id"])
	}

	c2, w2 := newTestContext(t, http.MethodGet, "/tasks?page=1&size=999&status=RUNNING", nil)
	h.ListTasks(c2)
	resp2 := decodeAPIResponse(t, w2)
	var page map[string]any
	decodeData(t, resp2.Data, &page)
	if page["size"].(float64) != 100 {
		t.Fatalf("size = %#v", page["size"])
	}

	c3, w3 := newTestContext(t, http.MethodGet, "/task/detail?task_id=task-1", nil)
	h.Detail(c3)
	resp3 := decodeAPIResponse(t, w3)
	var detail map[string]any
	decodeData(t, resp3.Data, &detail)
	if detail["task"] == nil {
		t.Fatal("task detail missing")
	}

	c4, w4 := newTestContext(t, http.MethodPost, "/task/cancel", gin.H{"task_id": "task-1"})
	h.Cancel(c4)
	resp4 := decodeAPIResponse(t, w4)
	var cancelled map[string]any
	decodeData(t, resp4.Data, &cancelled)
	if cancelled["status"] != "CANCELLED" {
		t.Fatalf("status = %#v", cancelled["status"])
	}

	c5, w5 := newTestContext(t, http.MethodGet, "/task/subtasks?task_id=task-1", nil)
	h.ListSubTasks(c5)
	resp5 := decodeAPIResponse(t, w5)
	var subtasks map[string]any
	decodeData(t, resp5.Data, &subtasks)
	if subtasks["total"].(float64) != 1 {
		t.Fatalf("subtasks total = %#v", subtasks["total"])
	}
}

func TestSkillAndMemoryHandlers(t *testing.T) {
	skillHandler := NewSkillHandler(&mockSkillService{
		listSkillsFn: func(ctx context.Context) ([]*model.SkillMeta, error) {
			return []*model.SkillMeta{{SkillName: "skill-a", Enabled: true}}, nil
		},
	})
	c1, w1 := newTestContext(t, http.MethodGet, "/skills", nil)
	skillHandler.ListSkills(c1)
	resp1 := decodeAPIResponse(t, w1)
	var skills map[string]any
	decodeData(t, resp1.Data, &skills)
	if skills["total"].(float64) != 1 {
		t.Fatalf("skills total = %#v", skills["total"])
	}

	memoryHandler := NewMemoryHandler(&mockMemoryService{
		listWorkingFn: func(ctx context.Context, userID, taskID string, limit int) []*memory.WorkingMemory {
			if limit != 200 {
				t.Fatalf("working limit = %d", limit)
			}
			return []*memory.WorkingMemory{{TaskID: "task-1", UserID: "user-123", Summary: "v1"}}
		},
		listLongFn: func(ctx context.Context, userID, skillName, keyword string, limit int) []*memory.LongTermMemory {
			if limit != 20 {
				t.Fatalf("long limit = %d", limit)
			}
			return []*memory.LongTermMemory{{MemoryID: "id-1", UserID: "user-123", SkillName: "skill-a", Content: "v2"}}
		},
	})
	c2, w2 := newTestContext(t, http.MethodGet, "/memory/working?limit=999", nil)
	memoryHandler.ListWorking(c2)
	resp2 := decodeAPIResponse(t, w2)
	var working map[string]any
	decodeData(t, resp2.Data, &working)
	if working["total"].(float64) != 1 {
		t.Fatalf("working total = %#v", working["total"])
	}

	c3, w3 := newTestContext(t, http.MethodGet, "/memory/long", nil)
	memoryHandler.ListLong(c3)
	resp3 := decodeAPIResponse(t, w3)
	var longTerm map[string]any
	decodeData(t, resp3.Data, &longTerm)
	if longTerm["total"].(float64) != 1 {
		t.Fatalf("long total = %#v", longTerm["total"])
	}

	_, w4 := newTestContext(t, http.MethodGet, "/memory/long", nil)
	NewMemoryHandler(nil).ListLong(func() *gin.Context {
		c, _ := gin.CreateTestContext(w4)
		req := httptest.NewRequest(http.MethodGet, "/memory/long", nil)
		c.Request = req
		c.Set("RequestID", "req-test-1")
		c.Set("UserID", "user-123")
		return c
	}())
	resp4 := decodeAPIResponse(t, w4)
	if resp4.Code != 500 {
		t.Fatalf("nil memory service code = %d", resp4.Code)
	}
}

func TestStreamHelpers(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/stream?last_seq=12", nil)
	req.Header.Set("Last-Event-ID", "7")
	c.Request = req
	if got := resolveLastSeq(c); got != 12 {
		t.Fatalf("resolveLastSeq query = %d", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req2.Header.Set("Last-Event-ID", "7")
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = req2
	if got := resolveLastSeq(c2); got != 7 {
		t.Fatalf("resolveLastSeq header = %d", got)
	}

	if got, ok := parseSeqValue("-1"); ok || got != 0 {
		t.Fatalf("parseSeqValue negative = %d %v", got, ok)
	}

	action, taskID, content := parseWSCommand([]byte(`{"action":" Inject ","task_id":" task-1 ","content":" hello "}`))
	if action != "inject" || taskID != "task-1" || content != "hello" {
		t.Fatalf("parseWSCommand = %q %q %q", action, taskID, content)
	}

	if string(marshalWSPayload(nil)) != "{}" {
		t.Fatal("marshalWSPayload nil should be {}")
	}
}

func TestStreamHandler_SSEAndErrors(t *testing.T) {
	mock := &mockStreamAppService{
		chatBacklog: []*StreamMessage{
			{Seq: 1, MsgType: "CHUNK", Content: "hello", CreatedAt: time.Unix(300, 0)},
		},
		chatCh: make(chan *StreamMessage, 1),
		taskBacklog: []*StreamMessage{
			{Seq: 2, MsgType: "CHUNK", Content: "work", CreatedAt: time.Unix(301, 0)},
		},
		taskCh: make(chan *StreamMessage, 1),
	}
	mock.chatCh <- &StreamMessage{Seq: 2, MsgType: "FINISH", Content: "done", CreatedAt: time.Unix(302, 0)}
	close(mock.chatCh)
	mock.taskCh <- &StreamMessage{Seq: 3, MsgType: "FINISH", Content: "done", CreatedAt: time.Unix(303, 0)}
	close(mock.taskCh)

	h := NewStreamHandler(mock)

	c1, w1 := newTestContext(t, http.MethodGet, "/stream/chat?sessionId=sess-1&last_seq=1", nil)
	h.ChatSSE(c1)
	body1 := w1.Body.String()
	if !strings.Contains(body1, "event: FINISH") || !strings.Contains(body1, "hello") {
		t.Fatalf("unexpected chat sse body: %s", body1)
	}

	c2, w2 := newTestContext(t, http.MethodGet, "/stream/task?taskId=task-1", nil)
	h.TaskSSE(c2)
	body2 := w2.Body.String()
	if !strings.Contains(body2, "work") || !strings.Contains(body2, "event: FINISH") {
		t.Fatalf("unexpected task sse body: %s", body2)
	}

	c3, w3 := newTestContext(t, http.MethodGet, "/stream/task?taskId=task-1", nil)
	NewStreamHandler(nil).TaskWebSocket(c3)
	resp3 := decodeAPIResponse(t, w3)
	if resp3.Code != 500 {
		t.Fatalf("expected nil service code 500, got %d", resp3.Code)
	}

	c4, w4 := newTestContext(t, http.MethodGet, "/stream/task", nil)
	h.TaskSSE(c4)
	resp4 := decodeAPIResponse(t, w4)
	if resp4.Code != 400 {
		t.Fatalf("expected missing taskId code 400, got %d", resp4.Code)
	}

	c5, w5 := newTestContext(t, http.MethodGet, "/stream/chat?sessionId=sess-1", nil)
	NewStreamHandler(nil).ChatSSE(c5)
	resp5 := decodeAPIResponse(t, w5)
	if resp5.Code != 500 {
		t.Fatalf("expected nil service code 500, got %d", resp5.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}
