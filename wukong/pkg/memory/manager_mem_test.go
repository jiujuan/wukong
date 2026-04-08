package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkingMemoryFlow(t *testing.T) {
	mgr := NewManager(nil, nil)
	taskID := "task_1"
	err := mgr.WriteMemory(context.Background(), NamespaceWorking, taskID, map[string]any{
		"user_id":     "user_1",
		"window_size": 3,
		"role":        "user",
		"content":     "hello",
	})
	if err != nil {
		t.Fatalf("write working memory failed: %v", err)
	}
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, taskID, map[string]any{
		"role":    "assistant",
		"content": "hi",
	})
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, taskID, map[string]any{
		"role":    "user",
		"content": "summarize this",
	})
	summary, err := mgr.CompressMemory(context.Background(), taskID)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}
	if !strings.Contains(summary, "user:") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	item, ok, err := mgr.ReadMemory(context.Background(), NamespaceWorking, taskID)
	if err != nil || !ok {
		t.Fatalf("read working failed: ok=%v err=%v", ok, err)
	}
	if item["task_id"] != taskID {
		t.Fatalf("unexpected task id: %v", item["task_id"])
	}
}

func TestArchiveAndSearchLongMemory(t *testing.T) {
	mgr := NewManager(nil, nil)
	taskID := "task_2"
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, taskID, map[string]any{
		"user_id": "user_2",
		"role":    "user",
		"content": "需要一份行业报告",
	})
	mem, err := mgr.SessionArchive(context.Background(), taskID, "report_gen", "行业报告")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if mem == nil || mem.MemoryID == "" {
		t.Fatalf("archive should return memory id")
	}
	list := mgr.long.Search("user_2", "report_gen", "行业", 10)
	if len(list) == 0 {
		t.Fatalf("search long memory should return result")
	}
}

func TestSharedAndExpire(t *testing.T) {
	mgr := NewManager(nil, nil)
	shareKey := "share_1"
	err := mgr.WriteMemory(context.Background(), NamespaceShared, shareKey, map[string]any{
		"data": map[string]any{
			"k1": "v1",
		},
		"expire_sec": 1,
	})
	if err != nil {
		t.Fatalf("write shared failed: %v", err)
	}
	err = mgr.SharedMemorySync(context.Background(), shareKey, map[string]any{"k2": "v2"})
	if err != nil {
		t.Fatalf("sync shared failed: %v", err)
	}
	item, ok, err := mgr.ReadMemory(context.Background(), NamespaceShared, shareKey)
	if err != nil || !ok {
		t.Fatalf("read shared failed: ok=%v err=%v", ok, err)
	}
	data, ok := item["data"].(map[string]any)
	if !ok || data["k2"] != "v2" {
		t.Fatalf("unexpected shared data: %+v", item)
	}
	time.Sleep(1100 * time.Millisecond)
	n, err := mgr.MemoryExpire(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("expire failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("expire count should be > 0")
	}
}

func TestListWorkingAndLongMemory(t *testing.T) {
	mgr := NewManager(nil, nil)
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, "task_a", map[string]any{
		"user_id": "user_list",
		"role":    "user",
		"content": "first",
	})
	time.Sleep(5 * time.Millisecond)
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, "task_b", map[string]any{
		"user_id": "user_list",
		"role":    "user",
		"content": "second",
	})
	_ = mgr.WriteMemory(context.Background(), NamespaceWorking, "task_c", map[string]any{
		"user_id": "other_user",
		"role":    "user",
		"content": "third",
	})

	working := mgr.ListWorking(context.Background(), "user_list", "", 10)
	if len(working) != 2 {
		t.Fatalf("expected 2 working items, got %d", len(working))
	}
	if working[0].TaskID != "task_b" {
		t.Fatalf("expected newest task first, got %s", working[0].TaskID)
	}

	_, _ = mgr.SessionArchive(context.Background(), "task_a", "report_gen", "topic_a")
	time.Sleep(5 * time.Millisecond)
	_, _ = mgr.SessionArchive(context.Background(), "task_b", "report_gen", "topic_b")
	_, _ = mgr.SessionArchive(context.Background(), "task_c", "planner", "topic_c")

	longList := mgr.ListLong(context.Background(), "user_list", "report_gen", "", 10)
	if len(longList) != 2 {
		t.Fatalf("expected 2 long items, got %d", len(longList))
	}
	if longList[0].Topic != "topic_b" {
		t.Fatalf("expected newest memory first, got topic=%s", longList[0].Topic)
	}
}
