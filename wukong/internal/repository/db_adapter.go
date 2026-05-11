package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	dbpkg "github.com/jiujuan/wukong/pkg/database"
)

type repositoryDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (repositoryTx, error)
}

type repositoryTx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type repositoryDBAdapter struct {
	db *dbpkg.DB
}

func wrapRepositoryDB(db *dbpkg.DB) repositoryDB {
	if db == nil {
		return nil
	}
	return &repositoryDBAdapter{db: db}
}

func (a *repositoryDBAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.db.Exec(ctx, sql, args...)
}

func (a *repositoryDBAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.db.Query(ctx, sql, args...)
}

func (a *repositoryDBAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.db.Pool().QueryRow(ctx, sql, args...)
}

func (a *repositoryDBAdapter) Begin(ctx context.Context) (repositoryTx, error) {
	tx, err := a.db.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &repositoryTxAdapter{tx: tx}, nil
}

type repositoryTxAdapter struct {
	tx pgx.Tx
}

func (a *repositoryTxAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.tx.Exec(ctx, sql, args...)
}

func (a *repositoryTxAdapter) Commit(ctx context.Context) error {
	return a.tx.Commit(ctx)
}

func (a *repositoryTxAdapter) Rollback(ctx context.Context) error {
	return a.tx.Rollback(ctx)
}
