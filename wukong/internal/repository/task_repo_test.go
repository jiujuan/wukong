package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jiujuan/wukong/pkg/manager"
)

func TestTaskRepositoryPlaceholderTuple(t *testing.T) {
	if got := placeholderTuple(4, 3); got != "($4, $5, $6)" {
		t.Fatalf("unexpected tuple: %s", got)
	}
}

func TestTaskRepositoryCreateAndGetTask(t *testing.T) {
	var gotQuery string
	db := &fakeRepositoryDB{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotQuery = sql
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "task-1"
				*(dest[1].(*string)) = "user-1"
				*(dest[2].(*pgtype.Text)) = pgtype.Text{String: "sess-1", Valid: true}
				*(dest[3].(*string)) = "planner"
				*(dest[4].(*[]byte)) = []byte(`{"a":1}`)
				*(dest[5].(*string)) = "RUNNING"
				*(dest[6].(*int)) = 5
				*(dest[7].(*int)) = 1
				*(dest[8].(*int)) = 3
				*(dest[9].(*time.Time)) = time.Unix(1, 0)
				*(dest[10].(*time.Time)) = time.Unix(2, 0)
				*(dest[11].(*[]byte)) = []byte(`{"result":true}`)
				*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "boom", Valid: true}
				return nil
			}}
		},
	}
	repo := &TaskRepository{db: db}

	task := &manager.Task{
		TaskID:     "task-1",
		UserID:     "user-1",
		SessionID:  "sess-1",
		SkillName:  "planner",
		Params:     map[string]any{"a": 1},
		Status:     "RUNNING",
		Priority:   5,
		RetryCount: 1,
		MaxRetry:   3,
		CreatedAt:  time.Unix(1, 0),
		UpdatedAt:  time.Unix(2, 0),
	}

	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if gotQuery == "" {
		t.Fatalf("expected exec query to be recorded")
	}

	got, err := repo.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.SessionID != "sess-1" || got.Error != "boom" || got.Params["a"].(float64) != 1 {
		t.Fatalf("unexpected get task result: %+v", got)
	}
}

func TestTaskRepositoryListAndSubTasks(t *testing.T) {
	db := &fakeRepositoryDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sqlContains(sql, "COUNT(*)") {
				return &fakeRows{
					scanFns: []func(dest ...any) error{
						func(dest ...any) error {
							*(dest[0].(*int64)) = 1
							return nil
						},
					},
				}, nil
			}
			if sqlContains(sql, "FROM task_info") && sqlContains(sql, "ORDER BY created_at DESC") {
				return &fakeRows{
					scanFns: []func(dest ...any) error{
						func(dest ...any) error {
							*(dest[0].(*string)) = "task-1"
							*(dest[1].(*string)) = "user-1"
							*(dest[2].(*pgtype.Text)) = pgtype.Text{String: "sess-1", Valid: true}
							*(dest[3].(*string)) = "planner"
							*(dest[4].(*[]byte)) = []byte(`{"x":1}`)
							*(dest[5].(*string)) = "PENDING"
							*(dest[6].(*int)) = 5
							*(dest[7].(*int)) = 0
							*(dest[8].(*int)) = 3
							*(dest[9].(*time.Time)) = time.Unix(3, 0)
							*(dest[10].(*time.Time)) = time.Unix(4, 0)
							*(dest[11].(*[]byte)) = []byte(`{"done":true}`)
							*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "", Valid: false}
							return nil
						},
					},
				}, nil
			}
			return &fakeRows{
				scanFns: []func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*string)) = "sub-1"
						*(dest[1].(*string)) = "task-1"
						*(dest[2].(*[]byte)) = []byte(`["dep-1"]`)
						*(dest[3].(*string)) = "action"
						*(dest[4].(*[]byte)) = []byte(`{"k":1}`)
						*(dest[5].(*string)) = "WAITING"
						*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "worker-1", Valid: true}
						*(dest[7].(*time.Time)) = time.Unix(5, 0)
						*(dest[8].(*time.Time)) = time.Unix(6, 0)
						return nil
					},
				},
			}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int64)) = 1
				return nil
			}}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := &TaskRepository{db: db}

	tasks, total, err := repo.ListTasks(context.Background(), "user-1", "", 1, 20)
	if err != nil || total != 1 || len(tasks) != 1 {
		t.Fatalf("unexpected tasks list: tasks=%+v total=%d err=%v", tasks, total, err)
	}
	subtasks, err := repo.GetSubTasks(context.Background(), "task-1")
	if err != nil || len(subtasks) != 1 || subtasks[0].WorkerID != "worker-1" {
		t.Fatalf("unexpected subtasks: subtasks=%+v err=%v", subtasks, err)
	}
}
