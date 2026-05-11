package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/pkg/skills"
)

func TestSkillPlaceholderTuple(t *testing.T) {
	if got := skillPlaceholderTuple(3, 4); got != "($3, $4, $5, $6)" {
		t.Fatalf("unexpected tuple: %s", got)
	}
}

func TestSkillRepositoryBatchUpsertSkills(t *testing.T) {
	var gotQuery string
	var gotArgs []any
	db := &fakeRepositoryDB{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotQuery = sql
			gotArgs = append([]any(nil), args...)
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := &SkillRepository{db: db}

	err := repo.BatchUpsertSkills(context.Background(), []*skills.Skill{
		nil,
		{},
		{SkillName: "planner", Description: "desc", Version: "v1", Enabled: true, Memory: skills.MemoryConfig{MemoryType: "short", WindowSize: 8, CompressSwitch: true}},
	})
	if err != nil {
		t.Fatalf("BatchUpsertSkills failed: %v", err)
	}
	if gotQuery == "" || len(gotArgs) != 8 {
		t.Fatalf("unexpected exec payload: query=%q args=%d", gotQuery, len(gotArgs))
	}
}

func TestSkillRepositoryListGetUpdate(t *testing.T) {
	db := &fakeRepositoryDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if len(args) != 1 || args[0] != 200 {
				t.Fatalf("unexpected list args: %#v", args)
			}
			return &fakeRows{
				scanFns: []func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*int64)) = 1
						*(dest[1].(*string)) = "planner"
						*(dest[2].(*string)) = "desc"
						*(dest[3].(*string)) = "v1"
						*(dest[4].(*bool)) = true
						*(dest[5].(*string)) = "short"
						*(dest[6].(*int)) = 8
						*(dest[7].(*bool)) = true
						*(dest[8].(*time.Time)) = time.Unix(1, 0)
						*(dest[9].(*time.Time)) = time.Unix(2, 0)
						return nil
					},
				},
			}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			switch {
			case sqlContains(sql, "WHERE skill_name = $1"):
				return &fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 2
					*(dest[1].(*string)) = "writer"
					*(dest[2].(*string)) = "write"
					*(dest[3].(*string)) = "v2"
					*(dest[4].(*bool)) = false
					*(dest[5].(*string)) = "long"
					*(dest[6].(*int)) = 12
					*(dest[7].(*bool)) = false
					*(dest[8].(*time.Time)) = time.Unix(3, 0)
					*(dest[9].(*time.Time)) = time.Unix(4, 0)
					return nil
				}}
			default:
				return &fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*int64)) = 9
					return nil
				}}
			}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := &SkillRepository{db: db}

	list, err := repo.ListSkills(context.Background(), 0)
	if err != nil || len(list) != 1 || list[0].SkillName != "planner" {
		t.Fatalf("unexpected list result: list=%+v err=%v", list, err)
	}
	item, err := repo.GetSkill(context.Background(), "writer")
	if err != nil || item == nil || item.SkillName != "writer" {
		t.Fatalf("unexpected get result: item=%+v err=%v", item, err)
	}
	if err := repo.UpdateSkill(context.Background(), &model.SkillMeta{SkillName: "writer", Description: "new"}); err != nil {
		t.Fatalf("UpdateSkill failed: %v", err)
	}
}
