package repository

import (
	"sync"
	"testing"

	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newItem(id string) *model.Item {
	return &model.Item{
		ID: id,
	}
}

func TestMemoryRepository_Save(t *testing.T) {
	tests := []struct {
		name          string
		initialItems  []*model.Item
		itemToSave    *model.Item
		expectedCount int
	}{
		{
			name:          "добавление первого элемента",
			initialItems:  nil,
			itemToSave:    newItem("id-1"),
			expectedCount: 1,
		},
		{
			name: "добавление второго элемента",
			initialItems: []*model.Item{
				newItem("id-1"),
			},
			itemToSave:    newItem("id-2"),
			expectedCount: 2,
		},
		{
			name: "обновление существующего элемента (тот же ID)",
			initialItems: []*model.Item{
				newItem("id-1"),
			},
			itemToSave:    newItem("id-1"),
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMemoryRepository()

			for _, item := range tt.initialItems {
				repo.Save(item)
			}

			repo.Save(tt.itemToSave)

			assert.Equal(t, tt.expectedCount, repo.Count())

			allItems := repo.GetAll()
			assert.Len(t, allItems, tt.expectedCount)

			found := false
			for _, item := range allItems {
				if item.ID == tt.itemToSave.ID {
					found = true
					break
				}
			}
			assert.True(t, found)
		})
	}
}

func TestMemoryRepository_Delete(t *testing.T) {
	tests := []struct {
		name          string
		initialItems  []*model.Item
		deleteID      string
		expectedCount int
		expectedErr   error
	}{
		{
			name: "успешное удаление существующего элемента",
			initialItems: []*model.Item{
				newItem("id-1"),
				newItem("id-2"),
			},
			deleteID:      "id-1",
			expectedCount: 1,
			expectedErr:   nil,
		},
		{
			name: "попытка удаления несуществующего элемента",
			initialItems: []*model.Item{
				newItem("id-1"),
			},
			deleteID:      "id-999",
			expectedCount: 1,
			expectedErr:   nil,
		},
		{
			name:          "удаление из пустого хранилища",
			initialItems:  nil,
			deleteID:      "id-1",
			expectedCount: 0,
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMemoryRepository()
			for _, item := range tt.initialItems {
				repo.Save(item)
			}

			err := repo.Delete(tt.deleteID)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedCount, repo.Count())

			if tt.expectedCount < len(tt.initialItems) {
				allItems := repo.GetAll()
				for _, item := range allItems {
					assert.NotEqual(t, tt.deleteID, item.ID)
				}
			}
		})
	}
}

func TestMemoryRepository_Clear(t *testing.T) {
	tests := []struct {
		name         string
		initialItems []*model.Item
	}{
		{
			name: "очистка заполненного хранилища",
			initialItems: []*model.Item{
				newItem("id-1"),
				newItem("id-2"),
				newItem("id-3"),
			},
		},
		{
			name:         "очистка уже пустого хранилища",
			initialItems: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMemoryRepository()
			for _, item := range tt.initialItems {
				repo.Save(item)
			}

			repo.Clear()

			assert.Equal(t, 0, repo.Count())
			assert.Empty(t, repo.GetAll())
		})
	}
}

func TestMemoryRepository_Concurrency(t *testing.T) {
	repo := NewMemoryRepository()
	const numGoroutines = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			item := newItem(string(rune(id)))
			repo.Save(item)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, numGoroutines, repo.Count())
	assert.Len(t, repo.GetAll(), numGoroutines)
}
