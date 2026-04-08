package database

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"
)

// QueryRow 执行查询并扫描到目标结构体
func (db *DB) QueryRow(ctx context.Context, dest interface{}, sql string, args ...interface{}) error {
	return db.pool.QueryRow(ctx, sql, args...).Scan(dest)
}

// Query 执行查询返回行迭代器
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// Exec 执行增删改
func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// GetOne 快捷方法：查询单条记录
func (db *DB) GetOne(ctx context.Context, dest interface{}, builder *SelectBuilder) error {
	builder.Limit(1)
	sql, args := builder.Build()
	return db.QueryRow(ctx, dest, sql, args...)
}

// List 快捷方法：查询多条记录（需配合行迭代处理）
func (db *DB) List(ctx context.Context, builder *SelectBuilder) (pgx.Rows, error) {
	sql, args := builder.Build()
	return db.Query(ctx, sql, args...)
}

// InsertOne 快捷插入单条
func (db *DB) InsertOne(ctx context.Context, builder *InsertBuilder) (pgconn.CommandTag, error) {
	sql, args, err := builder.Build()
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return db.Exec(ctx, sql, args...)
}

// UpdateOne 快捷更新
func (db *DB) UpdateOne(ctx context.Context, builder *UpdateBuilder) (pgconn.CommandTag, error) {
	sql, args := builder.Build()
	return db.Exec(ctx, sql, args...)
}

// DeleteOne 快捷删除
func (db *DB) DeleteOne(ctx context.Context, builder *DeleteBuilder) (pgconn.CommandTag, error) {
	sql, args := builder.Build()
	return db.Exec(ctx, sql, args...)
}

// Exists 检查记录是否存在
func (db *DB) Exists(ctx context.Context, builder *SelectBuilder) (bool, error) {
	builder.columns = []string{"1"}
	builder.Limit(1)
	sql, args := builder.Build()

	var exists bool
	err := db.pool.QueryRow(ctx, "SELECT EXISTS("+sql+")", args...).Scan(&exists)
	return exists, err
}

// Count 统计数量
func (db *DB) Count(ctx context.Context, table string, builder *SelectBuilder) (int64, error) {
	sql, args := builder.Build()
	// 提取 WHERE 部分
	whereIdx := strings.Index(strings.ToUpper(sql), "WHERE")
	var whereClause string
	if whereIdx > 0 {
		whereClause = sql[whereIdx:]
	}

	query := "SELECT COUNT(*) FROM " + table
	if whereClause != "" {
		query += " " + whereClause
	}

	var count int64
	err := db.pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}
