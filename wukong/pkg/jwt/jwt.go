package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Option 函数选项模式
type Option func(*JWT)

// JWT JWT工具
type JWT struct {
	secret      string
	expireHours int
	issuer      string
}

// New 创建JWT工具
func New(opts ...Option) *JWT {
	j := &JWT{
		secret:      "wukong-secret-key-change-in-production",
		expireHours: 2,
		issuer:      "wukong",
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

// WithSecret 设置密钥
func WithSecret(secret string) Option {
	return func(j *JWT) {
		j.secret = secret
	}
}

// WithExpireHours 设置过期小时数
func WithExpireHours(hours int) Option {
	return func(j *JWT) {
		j.expireHours = hours
	}
}

// WithIssuer 设置发行者
func WithIssuer(issuer string) Option {
	return func(j *JWT) {
		j.issuer = issuer
	}
}

// Claims JWT声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Generate 生成Token
func (j *JWT) Generate(userID, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(j.expireHours) * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

// Parse 解析Token
func (j *JWT) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// Validate 验证Token
func (j *JWT) Validate(tokenString string) bool {
	_, err := j.Parse(tokenString)
	return err == nil
}

// GetUserID 从Token获取用户ID
func (j *JWT) GetUserID(tokenString string) (string, error) {
	claims, err := j.Parse(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
