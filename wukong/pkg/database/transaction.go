package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tx 事务封装
type Tx struct {
	tx pgx.Tx
}

// Begin 开启事务
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Commit 提交事务
func (tx *Tx) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

// Rollback 回滚事务
func (tx *Tx) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}

// WithTransaction 自动管理事务（推荐用法）
func (db *DB) WithTransaction(ctx context.Context, fn func(context.Context, *Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

// Tx 上的 CRUD 方法（复用主库方法，替换底层执行器）
func (tx *Tx) Exec(ctx context.Context, sql string, args ...interface{}) (commandTag pgconn.CommandTag, err error) {
	return tx.tx.Exec(ctx, sql, args...)
}

func (tx *Tx) QueryRow(ctx context.Context, dest interface{}, sql string, args ...interface{}) error {
	return tx.tx.QueryRow(ctx, sql, args...).Scan(dest)
}

func (tx *Tx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return tx.tx.Query(ctx, sql, args...)
}
