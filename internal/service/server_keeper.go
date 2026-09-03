package service

import (
	"context"
	"errors"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/model"
)

var (
	// ErrNoUpdate возвращается методом GetItems, когда клиент запрашивает данные,
	// но с момента его последнего известного обновления (timeUpdate) на сервере
	// не было произведено никаких изменений.
	ErrNoUpdate = errors.New("no update available")
)

// KeeperRepository определяет интерфейс для слоя доступа к данным (Data Access Layer).
type KeeperRepository interface {
	// CreateItem сохраняет новый элемент данных.
	CreateItem(ctx context.Context, item *model.ItemDB) (*model.ItemDB, error)
	// UpdateItem обновляет элемент данных.
	UpdateItem(ctx context.Context, item *model.ItemDB) error
	// DeleteItem удаляет элемент данных.
	DeleteItem(ctx context.Context, item *model.ItemDB) error
	// GetItemsByUserID возвращает все элементы данных для указанного пользователя.
	GetItemsByUserID(ctx context.Context, userID string) ([]*model.ItemDB, error)
	// Close закрывает соединения с базой данных.
	Close()
}

// UpdateUsers определяет интерфейс для сервиса отслеживания времени последнего
// обновления данных пользователя.
type UpdateUsers interface {
	// Update фиксирует время последнего изменения данных для указанного пользователя.
	Update(userID string, timeUpdate time.Time)
	// Get возвращает время последнего обновления данных пользователя.
	Get(userID string) (time.Time, error)
}

type keeperService struct {
	storage KeeperRepository
	cfg     *config.ServerConfig
	users   UpdateUsers
}

// NewKeeperService создает и возвращает новый экземпляр keeperService,
// инициализированный переданными зависимостями.
func NewKeeperService(storage KeeperRepository, cfg *config.ServerConfig, u UpdateUsers) *keeperService {
	return &keeperService{
		storage: storage,
		cfg:     cfg,
		users:   u,
	}
}

// CreateItem создает новый зашифрованный элемент данных для пользователя.
func (s *keeperService) CreateItem(ctx context.Context, item *model.ItemDB) (*model.ItemDB, error) {
	s.users.Update(item.UserID, time.Now())

	return s.storage.CreateItem(ctx, item)
}

// UpdateItem обновляет существующий зашифрованный элемент данных.
func (s *keeperService) UpdateItem(ctx context.Context, item *model.ItemDB) error {
	s.users.Update(item.UserID, time.Now())

	return s.storage.UpdateItem(ctx, item)
}

// DeleteItem удаляет зашифрованный элемент данных.
func (s *keeperService) DeleteItem(ctx context.Context, item *model.ItemDB) error {
	s.users.Update(item.UserID, time.Now())

	return s.storage.DeleteItem(ctx, item)
}

// GetItems возвращает список элементов данных пользователя, реализуя логику
// инкрементальной синхронизации.
func (s *keeperService) GetItems(ctx context.Context, userID string, timeUpdate time.Time) ([]*model.ItemDB, error) {
	lastUpdate, errGet := s.users.Get(userID)
	s.users.Update(userID, time.Now())
	if errGet == nil {
		if timeUpdate.After(lastUpdate) {
			return nil, ErrNoUpdate
		}
	}

	return s.storage.GetItemsByUserID(ctx, userID)
}
