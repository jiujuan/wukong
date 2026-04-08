package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Option 函数选项模式
type Option func(*options)

type options struct {
	level     slog.Level
	format    string
	output    io.Writer
	addSource bool
}

// WithLevel 设置日志级别
func WithLevel(level slog.Level) Option {
	return func(o *options) {
		o.level = level
	}
}

// WithFormat 设置日志格式 (json/text)
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = format
	}
}

// WithOutput 设置输出目标
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		o.output = w
	}
}

// WithAddSource 添加源码位置
func WithAddSource(add bool) Option {
	return func(o *options) {
		o.addSource = add
	}
}

// Logger 日志器
type Logger struct {
	logger *slog.Logger
}

// New 新建日志实例
func New(opts ...Option) *Logger {
	o := &options{
		level:     slog.LevelInfo,
		format:    "json",
		output:    os.Stdout,
		addSource: false,
	}

	for _, opt := range opts {
		opt(o)
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     o.level,
		AddSource: o.addSource,
	}

	var handler slog.Handler
	if o.format == "json" {
		handler = slog.NewJSONHandler(o.output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(o.output, handlerOpts)
	}

	return &Logger{
		logger: slog.New(handler),
	}
}

func FromSlog(base *slog.Logger) *Logger {
	if base == nil {
		base = slog.Default()
	}
	return &Logger{logger: base}
}

// Info 输出Info级别日志
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Debug 输出Debug级别日志
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Warn 输出Warn级别日志
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error 输出Error级别日志
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Fatal 输出Fatal级别日志并退出
func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

// WithContext 添加上下文
func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
	return l.logger
}

// With 添加属性
func (l *Logger) With(attrs ...any) *slog.Logger {
	return l.logger.With(attrs...)
}

// WithTime 添加时间字段
func (l *Logger) WithTime(t interface{}) *slog.Logger {
	return l.logger.With("time", t)
}
