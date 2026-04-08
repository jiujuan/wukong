package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/jwt"
	"github.com/jiujuan/wukong/pkg/response"
)

// JWTAuth JWT认证中间件
func JWTAuth(jwtTool *jwt.JWT) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				response.Fail(c, errors.CodeUnauthorized, "Authorization格式错误")
				c.Abort()
				return
			}
			token = parts[1]
		} else {
			token = strings.TrimSpace(c.Query("access_token"))
			if token == "" {
				token = strings.TrimSpace(c.Query("token"))
			}
			if token == "" {
				response.Fail(c, errors.CodeUnauthorized, "缺少Authorization头")
				c.Abort()
				return
			}
		}

		if token == "" {
			response.Fail(c, errors.CodeUnauthorized, "Token为空")
			c.Abort()
			return
		}

		// 解析Token
		if jwtTool == nil {
			jwtTool = jwt.New()
		}

		claims, err := jwtTool.Parse(token)
		if err != nil {
			response.Fail(c, errors.CodeUnauthorized, "Token无效或已过期")
			c.Abort()
			return
		}

		// 设置用户信息到上下文
		c.Set("UserID", claims.UserID)
		c.Set("Username", claims.Username)
		c.Set("Token", token)

		c.Next()
	}
}

// GetUserID 获取用户ID
func GetUserID(c *gin.Context) string {
	userID, _ := c.Get("UserID")
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}

// GetUsername 获取用户名
func GetUsername(c *gin.Context) string {
	username, _ := c.Get("Username")
	if name, ok := username.(string); ok {
		return name
	}
	return ""
}
