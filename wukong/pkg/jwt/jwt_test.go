package jwt

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"default", []Option{}},
		{"with secret", []Option{WithSecret("custom-secret")}},
		{"with expire", []Option{WithExpireHours(24)}},
		{"with issuer", []Option{WithIssuer("test-issuer")}},
		{"all options", []Option{
			WithSecret("custom-secret"),
			WithExpireHours(24),
			WithIssuer("test-issuer"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := New(tt.opts...)
			if j == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	j := New(WithSecret("test-secret"))

	token, err := j.Generate("user123", "testuser")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if token == "" {
		t.Error("Generate() returned empty token")
	}
}

func TestParse(t *testing.T) {
	j := New(WithSecret("test-secret"))

	token, err := j.Generate("user123", "testuser")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims, err := j.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("UserID = %v, want %v", claims.UserID, "user123")
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %v, want %v", claims.Username, "testuser")
	}
}

func TestParseInvalidToken(t *testing.T) {
	j := New()

	_, err := j.Parse("invalid-token")
	if err == nil {
		t.Error("Parse() should return error for invalid token")
	}
}

func TestParseWithWrongSecret(t *testing.T) {
	j1 := New(WithSecret("secret1"))
	j2 := New(WithSecret("secret2"))

	token, err := j1.Generate("user123", "testuser")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	_, err = j2.Parse(token)
	if err == nil {
		t.Error("Parse() should return error for wrong secret")
	}
}

func TestValidate(t *testing.T) {
	j := New()

	token, _ := j.Generate("user123", "testuser")

	if !j.Validate(token) {
		t.Error("Validate() should return true for valid token")
	}

	if j.Validate("invalid-token") {
		t.Error("Validate() should return false for invalid token")
	}
}

func TestGetUserID(t *testing.T) {
	j := New()

	token, _ := j.Generate("user123", "testuser")

	userID, err := j.GetUserID(token)
	if err != nil {
		t.Fatalf("GetUserID() error = %v", err)
	}
	if userID != "user123" {
		t.Errorf("GetUserID() = %v, want %v", userID, "user123")
	}
}

func TestGetUserIDInvalidToken(t *testing.T) {
	j := New()

	_, err := j.GetUserID("invalid-token")
	if err == nil {
		t.Error("GetUserID() should return error for invalid token")
	}
}

func TestExpiredToken(t *testing.T) {
	// 创建过期时间为1秒的JWT
	j := New(WithExpireHours(0)) // 0小时会立即过期

	token, _ := j.Generate("user123", "testuser")

	// 等待2秒让token过期
	time.Sleep(2 * time.Second)

	_, err := j.Parse(token)
	if err == nil {
		t.Error("Parse() should return error for expired token")
	}
}

func TestClaims(t *testing.T) {
	j := New(WithSecret("test-secret"), WithIssuer("test-issuer"))

	token, _ := j.Generate("user123", "testuser")
	claims, _ := j.Parse(token)

	if claims.Issuer != "test-issuer" {
		t.Errorf("Issuer = %v, want %v", claims.Issuer, "test-issuer")
	}
	if claims.Subject != "user123" {
		t.Errorf("Subject = %v, want %v", claims.Subject, "user123")
	}
	if claims.IssuedAt.IsZero() {
		t.Error("IssuedAt should not be zero")
	}
	if claims.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
}
