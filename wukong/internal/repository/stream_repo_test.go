package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStreamRepositoryAppendAndList(t *testing.T) {
	db := &fakeRepositoryDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int64)) = 11
				*(dest[1].(*string)) = "task-1"
				*(dest[2].(*string)) = "CHUNK"
				*(dest[3].(*string)) = "hello"
				*(dest[4].(*int)) = 5
				*(dest[5].(*time.Time)) = time.Unix(1, 0)
				return nil
			}}
		},
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if len(args) != 3 || args[2] != 200 {
				t.Fatalf("unexpected list args: %#v", args)
			}
			return &fakeRows{
				scanFns: []func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*int64)) = 12
						*(dest[1].(*string)) = "task-1"
						*(dest[2].(*string)) = "FINISH"
						*(dest[3].(*string)) = "done"
						*(dest[4].(*int)) = 6
						*(dest[5].(*time.Time)) = time.Unix(2, 0)
						return nil
					},
				},
			}, nil
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := &StreamRepository{db: db}

	item, err := repo.AppendMessage(context.Background(), "task-1", "CHUNK", "hello")
	if err != nil || item == nil || item.MsgType != "CHUNK" {
		t.Fatalf("unexpected append result: item=%+v err=%v", item, err)
	}

	list, err := repo.ListAfterSeq(context.Background(), "task-1", 4, 0)
	if err != nil || len(list) != 1 || list[0].MsgType != "FINISH" {
		t.Fatalf("unexpected list result: list=%+v err=%v", list, err)
	}

	if err := repo.DeleteBefore(context.Background(), "task-1", time.Now()); err != nil {
		t.Fatalf("DeleteBefore failed: %v", err)
	}
}

func TestStreamRepositoryNilDB(t *testing.T) {
	if got, err := (&StreamRepository{}).AppendMessage(context.Background(), "task", "TYPE", "content"); got != nil || err != nil {
		t.Fatalf("expected nil result for nil db, got=%+v err=%v", got, err)
	}
	if got, err := (&StreamRepository{}).ListAfterSeq(context.Background(), "task", 0, 0); got != nil || err != nil {
		t.Fatalf("expected nil result for nil db, got=%+v err=%v", got, err)
	}
	if err := (&StreamRepository{}).DeleteBefore(context.Background(), "task", time.Now()); err != nil {
		t.Fatalf("expected nil error for nil db, got=%v", err)
	}
}
