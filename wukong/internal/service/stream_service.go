package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/internal/repository"
)

const (
	StreamTypeThink  = "THINK"
	StreamTypeTool   = "TOOL"
	StreamTypeChunk  = "CHUNK"
	StreamTypeStatus = "STATUS"
	StreamTypeFinish = "FINISH"
)

type StreamMessage = model.StreamMessage

type StreamService struct {
	repo *repository.StreamRepository

	mu      sync.RWMutex
	nextSub int64
	subs    map[string]map[int64]chan *model.StreamMessage
	buffer  map[string][]*model.StreamMessage
	seq     map[string]int
}

func NewStreamService(repo *repository.StreamRepository) *StreamService {
	return &StreamService{
		repo:   repo,
		subs:   make(map[string]map[int64]chan *model.StreamMessage),
		buffer: make(map[string][]*model.StreamMessage),
		seq:    make(map[string]int),
	}
}

func (s *StreamService) ChatKey(sessionID string) string {
	return "chat:" + sessionID
}

func (s *StreamService) TaskKey(taskID string) string {
	return "task:" + taskID
}

func (s *StreamService) PublishChat(ctx context.Context, sessionID string, msgType string, content string) (*model.StreamMessage, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id empty")
	}
	return s.publish(ctx, s.ChatKey(sessionID), msgType, content)
}

func (s *StreamService) PublishTask(ctx context.Context, taskID string, msgType string, content string) (*model.StreamMessage, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task id empty")
	}
	return s.publish(ctx, s.TaskKey(taskID), msgType, content)
}

func (s *StreamService) ReportTaskEvent(ctx context.Context, taskID string, msgType string, content string) {
	_, _ = s.PublishTask(ctx, taskID, msgType, content)
}

func (s *StreamService) SubscribeTask(ctx context.Context, taskID string, lastSeq int) ([]*model.StreamMessage, <-chan *model.StreamMessage, func()) {
	return s.subscribe(ctx, s.TaskKey(taskID), lastSeq)
}

func (s *StreamService) SubscribeChat(ctx context.Context, sessionID string, lastSeq int) ([]*model.StreamMessage, <-chan *model.StreamMessage, func()) {
	return s.subscribe(ctx, s.ChatKey(sessionID), lastSeq)
}

func (s *StreamService) publish(ctx context.Context, key string, msgType string, content string) (*model.StreamMessage, error) {
	msgType = normalizeType(msgType)
	var item *model.StreamMessage
	if s.repo != nil {
		var err error
		item, err = s.repo.AppendMessage(ctx, key, msgType, content)
		if err != nil {
			slog.Warn("stream append message failed, fallback to memory publish",
				"request_id", requestIDFromContext(ctx),
				"session_id", sessionIDFromKey(key),
				"stream_key", key,
				"msg_type", msgType,
				"error", err,
			)
		}
	}
	s.mu.Lock()
	if item == nil {
		s.seq[key]++
		item = &model.StreamMessage{
			TaskID:    key,
			MsgType:   msgType,
			Content:   content,
			Seq:       s.seq[key],
			CreatedAt: time.Now(),
		}
	} else if item.Seq > s.seq[key] {
		s.seq[key] = item.Seq
	}
	s.buffer[key] = append(s.buffer[key], item)
	if len(s.buffer[key]) > 1000 {
		s.buffer[key] = append([]*model.StreamMessage(nil), s.buffer[key][len(s.buffer[key])-1000:]...)
	}
	chans := make([]chan *model.StreamMessage, 0, len(s.subs[key]))
	for _, ch := range s.subs[key] {
		chans = append(chans, ch)
	}
	shouldClose := msgType == StreamTypeFinish
	if shouldClose {
		delete(s.subs, key)
	}
	s.mu.Unlock()

	for _, ch := range chans {
		select {
		case ch <- item:
		default:
		}
	}
	if shouldClose {
		for _, ch := range chans {
			close(ch)
		}
	}
	return item, nil
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value("RequestID"); value != nil {
		if requestID, ok := value.(string); ok {
			return requestID
		}
		return fmt.Sprint(value)
	}
	return ""
}

func sessionIDFromKey(key string) string {
	if strings.HasPrefix(key, "chat:") {
		return strings.TrimPrefix(key, "chat:")
	}
	return ""
}

func (s *StreamService) subscribe(ctx context.Context, key string, lastSeq int) ([]*model.StreamMessage, <-chan *model.StreamMessage, func()) {
	backlog := s.listAfterSeq(ctx, key, lastSeq)

	s.mu.Lock()
	s.nextSub++
	subID := s.nextSub
	if _, ok := s.subs[key]; !ok {
		s.subs[key] = make(map[int64]chan *model.StreamMessage)
	}
	ch := make(chan *model.StreamMessage, 32)
	s.subs[key][subID] = ch
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		group := s.subs[key]
		if group == nil {
			return
		}
		if sub, ok := group[subID]; ok {
			delete(group, subID)
			close(sub)
		}
		if len(group) == 0 {
			delete(s.subs, key)
		}
	}
	return backlog, ch, cancel
}

func (s *StreamService) listAfterSeq(ctx context.Context, key string, lastSeq int) []*model.StreamMessage {
	if s.repo != nil {
		rows, err := s.repo.ListAfterSeq(ctx, key, lastSeq, 500)
		if err == nil && len(rows) > 0 {
			return rows
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.buffer[key]
	if len(rows) == 0 {
		return nil
	}
	out := make([]*model.StreamMessage, 0, len(rows))
	for _, item := range rows {
		if item.Seq > lastSeq {
			out = append(out, item)
		}
	}
	return out
}

func normalizeType(msgType string) string {
	switch msgType {
	case StreamTypeThink, StreamTypeTool, StreamTypeChunk, StreamTypeStatus, StreamTypeFinish:
		return msgType
	default:
		return StreamTypeChunk
	}
}
