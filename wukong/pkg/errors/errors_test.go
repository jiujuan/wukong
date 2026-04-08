package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		msg     string
		wantErr bool
	}{
		{"success code", CodeSuccess, "success", false},
		{"bad request", CodeBadRequest, "bad request", false},
		{"unauthorized", CodeUnauthorized, "unauthorized", false},
		{"not found", CodeNotFound, "not found", false},
		{"server error", CodeServerError, "server error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.msg)
			if err == nil {
				t.Fatal("New() returned nil")
			}
			if err.Code != tt.code {
				t.Errorf("Code = %v, want %v", err.Code, tt.code)
			}
			if err.Msg != tt.msg {
				t.Errorf("Msg = %v, want %v", err.Msg, tt.msg)
			}
		})
	}
}

func TestNewWithOptions(t *testing.T) {
	innerErr := errors.New("inner error")
	err := New(CodeBadRequest, "bad request",
		WithDetail("additional detail"),
		WithError(innerErr),
	)

	if err.Detail != "additional detail" {
		t.Errorf("Detail = %v, want %v", err.Detail, "additional detail")
	}
	if err.Err != innerErr {
		t.Error("Err should be set")
	}
}

func TestErrorInterface(t *testing.T) {
	err := New(CodeBadRequest, "bad request")

	if err.Error() != "bad request" {
		t.Errorf("Error() = %v, want %v", err.Error(), "bad request")
	}
}

func TestErrorWithInnerError(t *testing.T) {
	innerErr := errors.New("inner error")
	err := New(CodeBadRequest, "outer error", WithError(innerErr))

	expected := "outer error: inner error"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		code int
		want int
	}{
		{CodeSuccess, http.StatusOK},
		{CodeBadRequest, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeTooManyReq, http.StatusTooManyRequests},
		{CodeServerError, http.StatusInternalServerError},
		{1001, http.StatusOK}, // 自定义错误码返回OK
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := New(tt.code, "test")
			if got := err.HTTPStatus(); got != tt.want {
				t.Errorf("HTTPStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		code int
	}{
		{"ErrSessionNotFound", ErrSessionNotFound, CodeSessionNot},
		{"ErrTaskNotFound", ErrTaskNotFound, CodeTaskNot},
		{"ErrSkillDisabled", ErrSkillDisabled, CodeSkillDisabled},
		{"ErrMsgSendFailed", ErrMsgSendFailed, CodeMsgSendFail},
		{"ErrUnauthorized", ErrUnauthorized, CodeUnauthorized},
		{"ErrForbidden", ErrForbidden, CodeForbidden},
		{"ErrNotFound", ErrNotFound, CodeNotFound},
		{"ErrBadRequest", ErrBadRequest, CodeBadRequest},
		{"ErrTooManyRequest", ErrTooManyRequest, CodeTooManyReq},
		{"ErrServerError", ErrServerError, CodeServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %v, want %v", tt.err.Code, tt.code)
			}
			if tt.err.Msg == "" {
				t.Error("Msg should not be empty")
			}
		})
	}
}
