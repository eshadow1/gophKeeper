package repository

import (
	"errors"
	"sync"
	"time"
)

type updateUsers struct {
	update map[string]time.Time
	mu     sync.RWMutex
}

// NewUpdateUsers создает и возвращает новый экземпляр updateUsers с
// инициализированной пустой картой временных меток.
func NewUpdateUsers() *updateUsers {
	return &updateUsers{
		update: make(map[string]time.Time),
		mu:     sync.RWMutex{},
	}
}

// Update устанавливает или обновляет временную метку последнего обновления
// для указанного пользователя. Метод потокобезопасен и может вызываться
// из нескольких горутин одновременно.
func (u *updateUsers) Update(userID string, timeUpdate time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.update[userID] = timeUpdate
}

// Get возвращает временную метку последнего обновления для указанного пользователя.
func (u *updateUsers) Get(userID string) (time.Time, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if t, ok := u.update[userID]; ok {
		return t, nil
	}

	return time.Time{}, errors.New("user not found")
}
