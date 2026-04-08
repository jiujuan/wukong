package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/middleware"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/jwt"
	"github.com/jiujuan/wukong/pkg/response"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	jwtTool *jwt.JWT
}

// NewAuthHandler 新建认证处理器
func NewAuthHandler(jwtTool *jwt.JWT) *AuthHandler {
	return &AuthHandler{jwtTool: jwtTool}
}

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errors.CodeBadRequest, "参数错误")
		return
	}

	// 模拟用户验证（实际应从数据库查询）
	if req.Username == "admin" && req.Password == "admin123" {
		userID := "user_admin"

		token, err := h.jwtTool.Generate(userID, req.Username)
		if err != nil {
			response.Fail(c, errors.CodeServerError, "生成Token失败")
			return
		}

		response.Success(c, gin.H{
			"access_token": token,
			"expire":       7200,
		})
		return
	}

	// 验证密码（示例）
	// 实际应从数据库查询用户并验证
	if req.Username != "admin" {
		response.Fail(c, errors.CodeUnauthorized, "用户名或密码错误")
		return
	}

	// 验证密码
	err := bcrypt.CompareHashAndPassword([]byte("$2a$10$xxx"), []byte(req.Password))
	if err != nil {
		response.Fail(c, errors.CodeUnauthorized, "用户名或密码错误")
		return
	}

	response.Fail(c, errors.CodeUnauthorized, "用户名或密码错误")
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	// 实际可以加入Redis黑名单
	userID := middleware.GetUserID(c)

	response.Success(c, gin.H{
		"message": "登出成功",
		"user_id": userID,
	})
}

// Profile 用户信息
func (h *AuthHandler) Profile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	response.Success(c, gin.H{
		"user_id":  userID,
		"username": username,
		"email":    "admin@wukong.com",
		"status":   "ACTIVE",
	})
}
