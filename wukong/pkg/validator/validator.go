package validator

import (
	"github.com/go-playground/validator/v10"
)

// Option 函数选项模式
type Option func(*validate)

// validate 验证器包装
type validate struct {
	v *validator.Validate
}

// New 创建验证器
func New(opts ...Option) *validate {
	v := validator.New()
	impl := &validate{v: v}
	for _, opt := range opts {
		opt(impl)
	}
	return impl
}

// WithTagName 设置tag名称
func WithTagName(tag string) Option {
	return func(v *validate) {
		v.v.SetTagName(tag)
	}
}

// Get 获取原始验证器
func (v *validate) Get() *validator.Validate {
	return v.v
}

// Validate 验证结构体
func (v *validate) Validate(s interface{}) error {
	return v.v.Struct(s)
}

// ValidateVar 验证单个变量
func (v *validate) ValidateVar(field interface{}, tag string) error {
	return v.v.Var(field, tag)
}

// Errors 获取验证错误
func (v *validate) Errors(err error) []string {
	var errs []string
	if err == nil {
		return errs
	}

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errs = append(errs, formatError(e))
		}
	}
	return errs
}

// formatError 格式化验证错误
func formatError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return e.Field() + " is required"
	case "email":
		return e.Field() + " must be a valid email"
	case "min":
		return e.Field() + " must be at least " + e.Param()
	case "max":
		return e.Field() + " must be at most " + e.Param()
	case "len":
		return e.Field() + " must be exactly " + e.Param() + " characters"
	default:
		return e.Field() + " is invalid"
	}
}
