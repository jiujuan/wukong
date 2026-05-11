package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jiujuan/wukong/internal/model"
)

func TestChatRepositoryHelpers(t *testing.T) {
	if got := nullableString(" "); got != nil {
		t.Fatalf("nullableString should return nil for blank input, got %#v", got)
	}
	if got := nullableString("abc"); got != "abc" {
		t.Fatalf("nullableString mismatch: %#v", got)
	}
	if got := nullableJSON(" "); got != nil {
		t.Fatalf("nullableJSON should return nil for blank input, got %#v", got)
	}
	if got := nullableJSON(`{"a":1}`); got != `{"a":1}` {
		t.Fatalf("nullableJSON mismatch: %#v", got)
	}
	if got := nullableJSONRaw(nil); got != nil {
		t.Fatalf("nullableJSONRaw should return nil for empty input, got %#v", got)
	}
	if got := nullableJSONRaw(json.RawMessage("  ")); got != nil {
		t.Fatalf("nullableJSONRaw should trim blank json, got %#v", got)
	}

	msgs := []*model.ChatMessage{
		{MsgID: "1"},
		{MsgID: "2"},
		{MsgID: "3"},
	}
	reverseMessages(msgs)
	if msgs[0].MsgID != "3" || msgs[2].MsgID != "1" {
		t.Fatalf("reverseMessages failed: %+v", msgs)
	}
}

func TestChatRepositoryCreateSession(t *testing.T) {
	db := &fakeRepositoryDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if len(args) != 6 {
				t.Fatalf("unexpected arg count: %d", len(args))
			}
			return &fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int64)) = 42
				*(dest[1].(*time.Time)) = time.Unix(1, 0)
				*(dest[2].(*time.Time)) = time.Unix(2, 0)
				return nil
			}}
		},
	}
	repo := &ChatRepository{db: db}
	item := &model.ChatSession{SessionID: "s1", UserID: "u1", Title: "hello", Scene: "CHAT", Status: "OPEN"}

	if err := repo.CreateSession(context.Background(), item); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if item.ID != 42 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("unexpected item after create: %+v", item)
	}
}

func TestChatRepositoryListRecentMessages(t *testing.T) {
	db := &fakeRepositoryDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if len(args) != 3 {
				t.Fatalf("unexpected arg count: %d", len(args))
			}
			if args[2] != 12 {
				t.Fatalf("expected default limit 12, got %#v", args[2])
			}
			rows := &fakeRows{
				scanFns: []func(dest ...any) error{
					func(dest ...any) error {
						assignChatMessage(dest, 2, "m2", "s1", "u1", "assistant", "second", "TEXT", "", "", []byte(`{"a":1}`), []byte(`{"b":2}`), 2, time.Unix(2, 0))
						return nil
					},
					func(dest ...any) error {
						assignChatMessage(dest, 1, "m1", "s1", "u1", "user", "first", "TEXT", "task-1", "thinking", nil, nil, 1, time.Unix(1, 0))
						return nil
					},
				},
			}
			return rows, nil
		},
	}
	repo := &ChatRepository{db: db}

	items, err := repo.ListRecentMessages(context.Background(), "u1", "s1", 0)
	if err != nil {
		t.Fatalf("ListRecentMessages failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected list size: %d", len(items))
	}
	if items[0].MsgID != "m1" || items[1].MsgID != "m2" {
		t.Fatalf("expected chronological order, got %+v", items)
	}
	if items[0].TaskID != "task-1" || items[1].ToolCall != `{"a":1}` || items[1].ToolResult != `{"b":2}` {
		t.Fatalf("unexpected scan result: %+v", items)
	}
}

func TestChatRepositoryGetMemoryAndSessionExists(t *testing.T) {
	db := &fakeRepositoryDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			switch {
			case len(args) == 2 && args[0] == "u1" && args[1] == "s1" && sqlContains(sql, "FROM chat_memory"):
				return &fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 7
					*(dest[1].(*string)) = "s1"
					*(dest[2].(*string)) = "u1"
					*(dest[3].(*[]byte)) = []byte(`[{"role":"user","content":"hi"}]`)
					*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "summary", Valid: true}
					*(dest[5].(*[]byte)) = []byte(`{"name":"Ada"}`)
					*(dest[6].(*[]byte)) = []byte(`{"tone":"direct"}`)
					*(dest[7].(*time.Time)) = time.Unix(3, 0)
					*(dest[8].(*time.Time)) = time.Unix(4, 0)
					return nil
				}}
			case len(args) == 2 && sqlContains(sql, "FROM chat_session"):
				return &fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*int)) = 1
					return nil
				}}
			default:
				return &fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
			}
		},
	}
	repo := &ChatRepository{db: db}

	memory, err := repo.GetMemory(context.Background(), "u1", "s1")
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}
	if memory.Summary != "summary" || memory.SessionID != "s1" || string(memory.Preference) != `{"tone":"direct"}` {
		t.Fatalf("unexpected memory: %+v", memory)
	}

	ok, err := repo.SessionExists(context.Background(), "u1", "s1")
	if err != nil || !ok {
		t.Fatalf("SessionExists failed: ok=%v err=%v", ok, err)
	}
}

func TestChatRepositoryDeleteSession(t *testing.T) {
	tx := &fakeTx{rows: 1}
	db := &fakeRepositoryDB{
		beginFn: func(ctx context.Context) (repositoryTx, error) {
			return tx, nil
		},
	}
	repo := &ChatRepository{db: db}

	deleted, err := repo.DeleteSession(context.Background(), "u1", "s1")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted session")
	}
	if tx.execCalls != 2 {
		t.Fatalf("expected two delete statements, got %d", tx.execCalls)
	}
}

func assignChatMessage(dest []any, id int64, msgID, sessionID, userID, role, content, contentType, taskID, thought string, toolCall, toolResult []byte, seq int, createdAt time.Time) {
	*(dest[0].(*int64)) = id
	*(dest[1].(*string)) = msgID
	*(dest[2].(*string)) = sessionID
	*(dest[3].(*string)) = userID
	*(dest[4].(*string)) = role
	*(dest[5].(*string)) = content
	*(dest[6].(*string)) = contentType
	if taskID != "" {
		*(dest[7].(*pgtype.Text)) = pgtype.Text{String: taskID, Valid: true}
	}
	if thought != "" {
		*(dest[8].(*pgtype.Text)) = pgtype.Text{String: thought, Valid: true}
	}
	if toolCall != nil {
		*(dest[9].(*[]byte)) = toolCall
	}
	if toolResult != nil {
		*(dest[10].(*[]byte)) = toolResult
	}
	*(dest[11].(*int)) = seq
	*(dest[12].(*time.Time)) = createdAt
}

func sqlContains(sql, part string) bool {
	return strings.Contains(sql, part)
}
