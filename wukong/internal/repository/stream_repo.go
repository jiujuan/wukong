package repository

import (
	"context"
	"time"

	"github.com/jiujuan/wukong/internal/model"
	dbpkg "github.com/jiujuan/wukong/pkg/database"
)

type StreamRepository struct {
	db *dbpkg.DB
}

func NewStreamRepository(db *dbpkg.DB) *StreamRepository {
	return &StreamRepository{db: db}
}

func (r *StreamRepository) AppendMessage(ctx context.Context, taskID string, msgType string, content string) (*model.StreamMessage, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	query := `
		WITH next_seq AS (
			SELECT COALESCE(MAX(seq), 0) + 1 AS seq
			FROM stream_message
			WHERE task_id = $1
		)
		INSERT INTO stream_message (task_id, msg_type, content, seq, created_at)
		SELECT $1, $2, $3, next_seq.seq, NOW()
		FROM next_seq
		RETURNING id, task_id, msg_type, content, seq, created_at
	`
	item := &model.StreamMessage{}
	err := r.db.Pool().QueryRow(ctx, query, taskID, msgType, content).Scan(
		&item.ID, &item.TaskID, &item.MsgType, &item.Content, &item.Seq, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *StreamRepository) ListAfterSeq(ctx context.Context, taskID string, afterSeq int, limit int) ([]*model.StreamMessage, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, msg_type, content, seq, created_at
		FROM stream_message
		WHERE task_id = $1 AND seq > $2
		ORDER BY seq
		LIMIT $3
	`, taskID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*model.StreamMessage, 0, limit)
	for rows.Next() {
		item := &model.StreamMessage{}
		if err := rows.Scan(&item.ID, &item.TaskID, &item.MsgType, &item.Content, &item.Seq, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *StreamRepository) DeleteBefore(ctx context.Context, taskID string, before time.Time) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM stream_message
		WHERE task_id = $1 AND created_at < $2
	`, taskID, before)
	return err
}
