package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config 数据库配置
type Config struct {
	Host            string
	Port            uint16
	Database        string
	User            string
	Password        string
	MaxConns        int32
	MinConns        int32
	MaxConnIdle     time.Duration
	MaxConnLifetime time.Duration
}

// DB 封装的数据库实例
type DB struct {
	pool *pgxpool.Pool
}

// New 创建数据库连接池
func New(cfg Config) (*DB, error) {
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = cfg.MaxConns
	config.MinConns = cfg.MinConns
	config.MaxConnIdleTime = cfg.MaxConnIdle
	config.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close 关闭连接池
func (db *DB) Close() {
	db.pool.Close()
}

// Pool 获取原生连接池（用于高级用法）
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}
