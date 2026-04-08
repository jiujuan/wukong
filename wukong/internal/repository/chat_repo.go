package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jiujuan/wukong/internal/model"
	dbpkg "github.com/jiujuan/wukong/pkg/database"
)

type ChatRepository struct {
	db *dbpkg.DB
}

func NewChatRepository(db *dbpkg.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateSession(ctx context.Context, item *model.ChatSession) error {
	if r == nil || r.db == nil || item == nil {
		return fmt.Errorf("chat repository create session failed: repository not ready")
	}
	query := `
		INSERT INTO chat_session (session_id, user_id, title, scene, status, created_at, updated_at, expire_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), $6)
		RETURNING id, created_at, updated_at
	`
	if err := r.db.Pool().QueryRow(
		ctx,
		query,
		item.SessionID,
		item.UserID,
		item.Title,
		item.Scene,
		item.Status,
		item.ExpireAt,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return fmt.Errorf("chat repository create session query failed: %w", err)
	}
	return nil
}

func (r *ChatRepository) GetSession(ctx context.Context, userID, sessionID string) (*model.ChatSession, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("chat repository not ready")
	}
	query := `
		SELECT id, session_id, user_id, title, scene, status, created_at, updated_at, expire_at
		FROM chat_session
		WHERE user_id = $1 AND session_id = $2
		LIMIT 1
	`
	item := &model.ChatSession{}
	var title pgtype.Text
	var expireAt pgtype.Timestamptz
	err := r.db.Pool().QueryRow(ctx, query, userID, sessionID).Scan(
		&item.ID, &item.SessionID, &item.UserID, &title, &item.Scene, &item.Status,
		&item.CreatedAt, &item.UpdatedAt, &expireAt,
	)
	if err != nil {
		return nil, err
	}
	if title.Valid {
		item.Title = title.String
	}
	if expireAt.Valid {
		item.ExpireAt = &expireAt.Time
	}
	return item, nil
}

func (r *ChatRepository) ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("chat repository not ready")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	offset := (page - 1) * size
	countQuery := `SELECT COUNT(*) FROM chat_session WHERE user_id = $1`
	var total int64
	if err := r.db.Pool().QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT id, session_id, user_id, title, scene, status, created_at, updated_at, expire_at
		FROM chat_session
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, size, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*model.ChatSession, 0)
	for rows.Next() {
		item := &model.ChatSession{}
		var title pgtype.Text
		var expireAt pgtype.Timestamptz
		if err := rows.Scan(
			&item.ID, &item.SessionID, &item.UserID, &title, &item.Scene, &item.Status,
			&item.CreatedAt, &item.UpdatedAt, &expireAt,
		); err != nil {
			return nil, 0, err
		}
		if title.Valid {
			item.Title = title.String
		}
		if expireAt.Valid {
			item.ExpireAt = &expireAt.Time
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ChatRepository) CreateMessage(ctx context.Context, item *model.ChatMessage) error {
	if r == nil || r.db == nil || item == nil {
		return fmt.Errorf("chat repository not ready")
	}
	query := `
		WITH next_seq AS (
			SELECT COALESCE(MAX(seq), 0) + 1 AS seq
			FROM chat_message
			WHERE session_id = $1
		)
		INSERT INTO chat_message (
			msg_id, session_id, user_id, role, content, content_type, task_id, thought, tool_call, tool_result, seq, created_at
		)
		SELECT
			$2, $1, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, next_seq.seq, NOW()
		FROM next_seq
		RETURNING id, seq, created_at
	`
	return r.db.Pool().QueryRow(
		ctx,
		query,
		item.SessionID,
		item.MsgID,
		item.UserID,
		item.Role,
		item.Content,
		item.ContentType,
		nullableString(item.TaskID),
		nullableString(item.Thought),
		nullableJSON(item.ToolCall),
		nullableJSON(item.ToolResult),
	).Scan(&item.ID, &item.Seq, &item.CreatedAt)
}

func (r *ChatRepository) ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("chat repository not ready")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size
	countQuery := `
		SELECT COUNT(*)
		FROM chat_message m
		INNER JOIN chat_session s ON s.session_id = m.session_id
		WHERE s.user_id = $1 AND m.session_id = $2
	`
	var total int64
	if err := r.db.Pool().QueryRow(ctx, countQuery, userID, sessionID).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT m.id, m.msg_id, m.session_id, m.user_id, m.role, m.content, m.content_type, m.task_id, m.thought, m.tool_call, m.tool_result, m.seq, m.created_at
		FROM chat_message m
		INNER JOIN chat_session s ON s.session_id = m.session_id
		WHERE s.user_id = $1 AND m.session_id = $2
		ORDER BY m.seq ASC, m.created_at ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, userID, sessionID, size, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*model.ChatMessage, 0)
	for rows.Next() {
		item := &model.ChatMessage{}
		var taskID, thought pgtype.Text
		var toolCall, toolResult []byte
		if err := rows.Scan(
			&item.ID, &item.MsgID, &item.SessionID, &item.UserID, &item.Role, &item.Content, &item.ContentType,
			&taskID, &thought, &toolCall, &toolResult, &item.Seq, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if taskID.Valid {
			item.TaskID = taskID.String
		}
		if thought.Valid {
			item.Thought = thought.String
		}
		item.ToolCall = strings.TrimSpace(string(toolCall))
		item.ToolResult = strings.TrimSpace(string(toolResult))
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *ChatRepository) SessionExists(ctx context.Context, userID, sessionID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("chat repository not ready")
	}
	query := `SELECT 1 FROM chat_session WHERE user_id = $1 AND session_id = $2 LIMIT 1`
	var one int
	err := r.db.Pool().QueryRow(ctx, query, userID, sessionID).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *ChatRepository) DeleteSession(ctx context.Context, userID, sessionID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("chat repository not ready")
	}
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chat_message WHERE user_id = $1 AND session_id = $2`, userID, sessionID); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM chat_session WHERE user_id = $1 AND session_id = $2`, userID, sessionID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullableJSON(s string) any {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
