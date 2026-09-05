package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/model"

	mockservice "github.com/eshadow1/gophkeeper/mocks/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func recentTimeMatcher() any {
	return mock.MatchedBy(func(t time.Time) bool {
		return time.Since(t) <= 2*time.Second
	})
}

func TestKeeperService_CreateItem(t *testing.T) {
	cfg := &config.ServerConfig{}
	testItem := &model.ItemDB{ID: "item-1", UserID: "user-1"}

	tests := []struct {
		name          string
		inputItem     *model.ItemDB
		mockRepoSetup func(*mockservice.MockKeeperRepository)
		mockUserSetup func(*mockservice.MockUpdateUsers)
		wantItem      *model.ItemDB
		wantErr       error
	}{
		{
			name:      "успешное создание элемента",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().CreateItem(mock.Anything, testItem).Return(testItem, nil)
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantItem: testItem,
			wantErr:  nil,
		},
		{
			name:      "ошибка при сохранении в репозиторий",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().CreateItem(mock.Anything, testItem).Return(nil, errors.New("db error"))
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantItem: nil,
			wantErr:  errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockKeeperRepository(t)
			mockUsers := mockservice.NewMockUpdateUsers(t)

			tt.mockRepoSetup(mockRepo)
			tt.mockUserSetup(mockUsers)

			svc := NewKeeperService(mockRepo, cfg, mockUsers)

			gotItem, err := svc.CreateItem(context.Background(), tt.inputItem)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantItem, gotItem)
		})
	}
}

func TestKeeperService_UpdateItem(t *testing.T) {
	cfg := &config.ServerConfig{}
	testItem := &model.ItemDB{ID: "item-1", UserID: "user-1"}

	tests := []struct {
		name          string
		inputItem     *model.ItemDB
		mockRepoSetup func(*mockservice.MockKeeperRepository)
		mockUserSetup func(*mockservice.MockUpdateUsers)
		wantErr       error
	}{
		{
			name:      "успешное обновление элемента",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().UpdateItem(mock.Anything, testItem).Return(nil)
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantErr: nil,
		},
		{
			name:      "ошибка при обновлении в репозитории",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().UpdateItem(mock.Anything, testItem).Return(errors.New("update failed"))
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantErr: errors.New("update failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockKeeperRepository(t)
			mockUsers := mockservice.NewMockUpdateUsers(t)

			tt.mockRepoSetup(mockRepo)
			tt.mockUserSetup(mockUsers)

			svc := NewKeeperService(mockRepo, cfg, mockUsers)

			err := svc.UpdateItem(context.Background(), tt.inputItem)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperService_DeleteItem(t *testing.T) {
	cfg := &config.ServerConfig{}
	testItem := &model.ItemDB{ID: "item-1", UserID: "user-1"}

	tests := []struct {
		name          string
		inputItem     *model.ItemDB
		mockRepoSetup func(*mockservice.MockKeeperRepository)
		mockUserSetup func(*mockservice.MockUpdateUsers)
		wantErr       error
	}{
		{
			name:      "успешное удаление элемента",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().DeleteItem(mock.Anything, testItem).Return(nil)
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantErr: nil,
		},
		{
			name:      "ошибка при удалении из репозитория",
			inputItem: testItem,
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().DeleteItem(mock.Anything, testItem).Return(errors.New("delete failed"))
			},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Update("user-1", recentTimeMatcher())
			},
			wantErr: errors.New("delete failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockKeeperRepository(t)
			mockUsers := mockservice.NewMockUpdateUsers(t)

			tt.mockRepoSetup(mockRepo)
			tt.mockUserSetup(mockUsers)

			svc := NewKeeperService(mockRepo, cfg, mockUsers)

			err := svc.DeleteItem(context.Background(), tt.inputItem)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperService_GetItems(t *testing.T) {
	cfg := &config.ServerConfig{}
	userID := "user-1"
	expectedItems := []*model.ItemDB{{ID: "item-1", UserID: userID}}

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	oneHourLater := now.Add(1 * time.Hour)

	tests := []struct {
		name          string
		timeUpdate    time.Time
		mockUserSetup func(*mockservice.MockUpdateUsers)
		mockRepoSetup func(*mockservice.MockKeeperRepository)
		wantItems     []*model.ItemDB
		wantErr       error
	}{
		{
			name:       "первая синхронизация",
			timeUpdate: time.Time{},
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Get(userID).Return(time.Time{}, errors.New("user not found"))
				m.EXPECT().Update(userID, recentTimeMatcher())
			},
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().GetItemsByUserID(mock.Anything, userID).Return(expectedItems, nil)
			},
			wantItems: expectedItems,
			wantErr:   nil,
		},
		{
			name:       "клиент актуален",
			timeUpdate: oneHourLater,
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Get(userID).Return(oneHourAgo, nil)
				m.EXPECT().Update(userID, recentTimeMatcher())
			},
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
			},
			wantItems: nil,
			wantErr:   ErrNoUpdate,
		},
		{
			name:       "клиент устарел",
			timeUpdate: oneHourAgo,
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Get(userID).Return(now, nil)
				m.EXPECT().Update(userID, recentTimeMatcher())
			},
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().GetItemsByUserID(mock.Anything, userID).Return(expectedItems, nil)
			},
			wantItems: expectedItems,
			wantErr:   nil,
		},
		{
			name:       "ошибка репозитория при синхронизации",
			timeUpdate: oneHourAgo,
			mockUserSetup: func(m *mockservice.MockUpdateUsers) {
				m.EXPECT().Get(userID).Return(now, nil)
				m.EXPECT().Update(userID, recentTimeMatcher())
			},
			mockRepoSetup: func(m *mockservice.MockKeeperRepository) {
				m.EXPECT().GetItemsByUserID(mock.Anything, userID).Return(nil, errors.New("db timeout"))
			},
			wantItems: nil,
			wantErr:   errors.New("db timeout"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockKeeperRepository(t)
			mockUsers := mockservice.NewMockUpdateUsers(t)

			tt.mockUserSetup(mockUsers)
			tt.mockRepoSetup(mockRepo)

			svc := NewKeeperService(mockRepo, cfg, mockUsers)

			gotItems, err := svc.GetItems(context.Background(), userID, tt.timeUpdate)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrNoUpdate) {
					assert.ErrorIs(t, err, ErrNoUpdate)
				} else {
					assert.EqualError(t, err, tt.wantErr.Error())
				}
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantItems, gotItems)
		})
	}
}
