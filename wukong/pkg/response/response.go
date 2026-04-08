package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Response 统一响应结构
type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data"`
}

// Option 函数选项模式
type Option func(*Response)

// WithRequestID 设置请求ID
func WithRequestID(reqID string) Option {
	return func(r *Response) {
		r.RequestID = reqID
	}
}

// WithCode 设置错误码
func WithCode(code int) Option {
	return func(r *Response) {
		r.Code = code
	}
}

// WithMsg 设置消息
func WithMsg(msg string) Option {
	return func(r *Response) {
		r.Msg = msg
	}
}

// WithData 设置数据
func WithData(data interface{}) Option {
	return func(r *Response) {
		r.Data = data
	}
}

// New 创建响应
func New(opts ...Option) *Response {
	r := &Response{
		Code: 200,
		Msg:  "success",
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	reqID, _ := c.Get("RequestID")
	c.JSON(http.StatusOK, Response{
		Code:      200,
		Msg:       "success",
		RequestID: reqID.(string),
		Data:      data,
	})
}

// Fail 失败响应
func Fail(c *gin.Context, code int, msg string) {
	reqID, _ := c.Get("RequestID")
	c.JSON(http.StatusOK, Response{
		Code:      code,
		Msg:       msg,
		RequestID: reqID.(string),
		Data:      nil,
	})
}
