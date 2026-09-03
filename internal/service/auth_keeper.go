// Package service реализует бизнес-логику приложения, включая процессы
// аутентификации и авторизации пользователей.
package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/eshadow1/gophkeeper/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// AuthRepository определяет интерфейс для слоя доступа к данным (Data Access Layer),
// специфичного для операций аутентификации и управления пользователями.
type AuthRepository interface {
	// CreateUser сохраняет нового пользователя в базе данных.
	// Возвращает созданную сущность пользователя или ошибку в случае неудачи.
	CreateUser(context.Context, *model.User) (*model.User, error)

	// GetUser находит и возвращает пользователя по его имени (username).
	GetUser(context.Context, string) (*model.User, error)

	// Close закрывает активные соединения с базой данных и освобождает ресурсы.
	Close()
}

// AuthWorker определяет интерфейс для компонента, отвечающего за работу
// с токенами аутентификации (например, генерацию и валидацию JWT).
type AuthWorker interface {
	// CreateJWT генерирует и возвращает строковое представление JSON Web Token (JWT)
	// для указанного идентификатора пользователя, используя предоставленный секретный ключ.
	CreateJWT(userID string, secret []byte) (string, error)
}

var (
	// ErrCompareHash возвращается, когда предоставленный пароль не совпадает
	// с хешированным паролем, хранящимся в базе данных.
	ErrCompareHash = errors.New("failed to compare hash")

	// ErrUserAlreadyExists возвращается при попытке зарегистрировать пользователя
	// с именем (username), которое уже занято в системе.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrUserNotFound возвращается, когда запрашиваемый пользователь не найден
	// в базе данных по заданному имени.
	ErrUserNotFound = errors.New("user not found")
)

// authKeeper реализует бизнес-логику аутентификации (регистрация и вход).
type authKeeper struct {
	// worker отвечает за генерацию JWT токенов.
	worker AuthWorker

	// repo предоставляет доступ к операциям с данными пользователей.
	repo AuthRepository

	// cfg содержит конфигурационные параметры аутентификации (например, секрет JWT).
	cfg *config.AuthConfig
}

// NewAuthKeeper создает и возвращает новый экземпляр authKeeper,
// инициализированный переданными зависимостями.
func NewAuthKeeper(a AuthWorker, r AuthRepository, cfg *config.AuthConfig) *authKeeper {
	return &authKeeper{
		worker: a,
		repo:   r,
		cfg:    cfg,
	}
}

// Register обрабатывает процесс регистрации нового пользователя.
// Метод хеширует предоставленный пароль, создает новую запись пользователя
// в репозитории и генерирует JWT токен для успешной регистрации.
func (a *authKeeper) Register(ctx context.Context, userUnsave *model.UserUnsave) (string, error) {
	hash, errGenerateHash := bcrypt.GenerateFromPassword([]byte(userUnsave.Password), bcrypt.DefaultCost)
	if errGenerateHash != nil {
		return "", errGenerateHash
	}

	user, errCreate := a.repo.CreateUser(ctx, &model.User{
		Username:     userUnsave.Username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	})
	if errCreate != nil {
		if errors.Is(errCreate, repository.ErrUserAlreadyExists) {
			return "", ErrUserAlreadyExists
		}
		loggers.Log.Error("failed to create user: ", errCreate)
		return "", errCreate
	}

	log.Printf("user registered: %s (id: %s)", user.Username, user.ID)
	return a.worker.CreateJWT(user.ID, a.cfg.JWTSecret)
}

// Login обрабатывает процесс аутентификации существующего пользователя.
// Метод извлекает данные пользователя из репозитория, сравнивает предоставленный
// пароль с сохраненным хешем и генерирует новый JWT токен при успешном совпадении.
func (a *authKeeper) Login(ctx context.Context, userWithPass *model.UserUnsave) (string, error) {
	user, errGet := a.repo.GetUser(ctx, userWithPass.Username)
	if errGet != nil {
		if errors.Is(errGet, repository.ErrUserNotFound) {
			return "", ErrUserNotFound
		}

		loggers.Log.Error("failed to get user: ", errGet)
		return "", errGet
	}

	if errCopare := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(userWithPass.Password)); errCopare != nil {
		return "", ErrCompareHash
	}

	log.Printf("user logged in: %s (id: %s)", user.Username, user.ID)
	return a.worker.CreateJWT(user.ID, a.cfg.JWTSecret)
}
