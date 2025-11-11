package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("无效的令牌")
	ErrExpiredToken = errors.New("令牌已过期")
)

// JWTManager JWT 管理器
type JWTManager struct {
	secretKey []byte
	issuer    string
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(secretKey string, issuer string) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
	}
}

// Claims JWT 声明
type Claims struct {
	Email    string `json:"email"`
	UserID   int64  `json:"user_id"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT 令牌
func (m *JWTManager) GenerateToken(email string, userID int64, isAdmin bool, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		Email:   email,
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateToken 验证 JWT 令牌
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// RefreshToken 刷新令牌
func (m *JWTManager) RefreshToken(tokenString string, expiry time.Duration) (string, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// 生成新令牌
	return m.GenerateToken(claims.Email, claims.UserID, claims.IsAdmin, expiry)
}

