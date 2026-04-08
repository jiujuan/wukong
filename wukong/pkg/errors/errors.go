package errors

import (
	"fmt"
	"net/http"
)

// 错误码定义
const (
	CodeSuccess       = 200
	CodeBadRequest    = 400
	CodeUnauthorized  = 401
	CodeForbidden     = 403
	CodeNotFound      = 404
	CodeTooManyReq    = 429
	CodeServerError   = 500
	CodeSessionNot    = 1001
	CodeTaskNot       = 1002
	CodeSkillDisabled = 1003
	CodeMsgSendFail   = 1004
)

// Option 函数选项模式
type Option func(*Error)

// Error 统一错误结构
type Error struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Detail string `json:"detail,omitempty"`
	Err    error  `json:"-"`
}

// WithDetail 设置错误详情
func WithDetail(detail string) Option {
	return func(e *Error) {
		e.Detail = detail
	}
}

// WithError 设置底层错误
func WithError(err error) Option {
	return func(e *Error) {
		e.Err = err
	}
}

// New 创建新错误
func New(code int, msg string, opts ...Option) *Error {
	e := &Error{Code: code, Msg: msg}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Error 实现error接口
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// HTTPStatus 获取HTTP状态码
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeTooManyReq:
		return http.StatusTooManyRequests
	case CodeServerError:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// 预定义错误
var (
	ErrSessionNotFound = New(CodeSessionNot, "会话不存在")
	ErrTaskNotFound    = New(CodeTaskNot, "任务不存在")
	ErrSkillDisabled   = New(CodeSkillDisabled, "技能未启用")
	ErrMsgSendFailed   = New(CodeMsgSendFail, "消息发送失败")
	ErrUnauthorized    = New(CodeUnauthorized, "未授权")
	ErrForbidden       = New(CodeForbidden, "无权限访问")
	ErrNotFound        = New(CodeNotFound, "资源不存在")
	ErrBadRequest      = New(CodeBadRequest, "参数错误")
	ErrTooManyRequest  = New(CodeTooManyReq, "请求过于频繁")
	ErrServerError     = New(CodeServerError, "服务内部错误")
)
