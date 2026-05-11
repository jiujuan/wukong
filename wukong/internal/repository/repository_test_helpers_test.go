package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRepositoryDB struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	beginFn    func(ctx context.Context) (repositoryTx, error)
}

func (f *fakeRepositoryDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeRepositoryDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFn != nil {
		return f.queryFn(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

func (f *fakeRepositoryDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{}
}

func (f *fakeRepositoryDB) Begin(ctx context.Context) (repositoryTx, error) {
	if f.beginFn != nil {
		return f.beginFn(ctx)
	}
	return &fakeTx{}, nil
}

type fakeRepositoryTx struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (f *fakeRepositoryTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (f *fakeRepositoryTx) Commit(ctx context.Context) error {
	if f.commitFn != nil {
		return f.commitFn(ctx)
	}
	return nil
}

func (f *fakeRepositoryTx) Rollback(ctx context.Context) error {
	if f.rollbackFn != nil {
		return f.rollbackFn(ctx)
	}
	return nil
}

type fakeTx struct {
	execCalls int
	rows      int64
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls++
	if f.rows > 0 {
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	return pgconn.NewCommandTag("DELETE 0"), nil
}

func (f *fakeTx) Commit(ctx context.Context) error {
	return nil
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	return nil
}

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

type fakeRows struct {
	scanFns []func(dest ...any) error
	idx     int
	err     error
	closed  bool
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT 1")
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.scanFns) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.scanFns) {
		return nil
	}
	return r.scanFns[r.idx-1](dest...)
}

func (r *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}
