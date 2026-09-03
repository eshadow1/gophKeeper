package model

import (
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	// UserIDContextKey используется как ключ для хранения и извлечения
	// идентификатора пользователя из контекста HTTP-запроса.
	UserIDContextKey contextKey = "user_id"
)

// UserClaims представляет структуру утверждений JWT-токена,
// содержащую идентификатор пользователя и стандартные зарегистрированные поля.
type UserClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// TokenAuth представляет структуру для аутентификации
type TokenAuth struct {
	UID   string `json:"uid"`
	Token string `json:"token"`
}
