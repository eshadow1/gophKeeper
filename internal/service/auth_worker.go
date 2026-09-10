// Package service предоставляет реализацию бизнес-логики приложения.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// CookieName определяет имя HTTP-куки, в которой хранится JWT-токен аутентификации.
	CookieName = "auth_token"
	// CookieMaxAge определяет срок жизни JWT-токена и соответствующей куки в секундах (30 дней).
	CookieMaxAge = 30 * 24 * 60 * 60 // 30 дней
)

var (
	// ErrInvalidToken возвращается при обнаружении недействительного, просроченного или неправильно подписанного JWT-токена.
	ErrInvalidToken = errors.New("invalid token")
)

type jwtWorker struct {
	cfg *config.AuthConfig
}

// NewJWTWorker создает и возвращает новый экземпляр компонента для работы с JWT-токенами.
func NewJWTWorker(cfg *config.AuthConfig) *jwtWorker {
	return &jwtWorker{
		cfg: cfg,
	}
}

// CreateJWT формирует и подписывает новый JWT-токен с утверждениями.
func (jw *jwtWorker) CreateJWT(userID string, secret []byte) (string, error) {
	claims := model.UserClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jw.cfg.TokenIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(CookieMaxAge) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateJWT выполняет разбор и проверку подписи JWT-токена,
// возвращая извлеченные утверждения пользователя в случае успешной валидации.
func (*jwtWorker) ValidateJWT(tokenString string, secret []byte) (*model.UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &model.UserClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*model.UserClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetUID выполняет получение JWT-токена
func (jw *jwtWorker) GetUID(_ context.Context, token string, jwtSecret []byte) (model.TokenAuth, error) {
	if token == "" {
		return model.TokenAuth{}, errors.New("invalid token")
	}

	claims, errValidate := jw.ValidateJWT(token, jwtSecret)
	if errValidate != nil {
		return model.TokenAuth{}, errors.New("invalid token")
	}

	return model.TokenAuth{
		UID: claims.UserID,
	}, nil
}

// GenerateUserID генерирует новый уникальный идентификатор пользователя в формате UUID.
func (*jwtWorker) GenerateUserID() string {
	return uuid.NewString()
}
