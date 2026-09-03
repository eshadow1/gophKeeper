// Package storage реализует in-memory хранилище для элементов GophKeeper.
// Хранилище обеспечивает потокобезопасный доступ к данным и опциональную
// персистентность в JSON-файл для сохранения состояния между запусками.
package repository

import (
	"errors"
	"sync"

	"github.com/eshadow1/gophkeeper/internal/model"
)

var (
	// ErrNotFound возвращается, если элемент с указанным ID не найден.
	ErrNotFound = errors.New("item not found")
	// ErrAlreadyExists возвращается, если элемент с таким ID уже существует.
	ErrAlreadyExists = errors.New("item already exists")
)

type MemoryRepository struct {
	items map[string]*model.Item
	mu    sync.RWMutex
}

// NewMemoryRepository создает и возвращает новое in-memory хранилище,
// инициализируя его данными, загруженными из файла по указанному пути.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		items: make(map[string]*model.Item),
		mu:    sync.RWMutex{},
	}
}

// Save сохраняет или обновляет элемент в памяти.
func (r *MemoryRepository) Save(item *model.Item) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
}

// GetAll возвращает копию среза всех элементов из памяти.
func (r *MemoryRepository) GetAll() []*model.Item {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []*model.Item
	for _, item := range r.items {
		res = append(res, item)
	}
	return res
}

// Clear полностью очищает хранилище в памяти.
func (r *MemoryRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[string]*model.Item)
}

// Delete удаляет элемент из хранилища по его ID.
// Возвращает ошибку ErrNotFound, если элемент не найден.
func (s *MemoryRepository) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[id]; !exists {
		return nil
	}

	delete(s.items, id)
	return nil
}

// Count возвращает количество элементов в хранилище.
func (s *MemoryRepository) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}
