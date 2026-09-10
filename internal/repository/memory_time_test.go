package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateUsers_GetAndUpdate(t *testing.T) {
	time1 := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2023, 10, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		initialUpdates map[string]time.Time
		userIDToUpdate string
		timeToUpdate   time.Time
		userIDToGet    string
		expectedTime   time.Time
		expectedErr    string
	}{
		{
			name:           "получение несуществующего пользователя из пустого хранилища",
			initialUpdates: nil,
			userIDToGet:    "user_1",
			expectedTime:   time.Time{},
			expectedErr:    "user not found",
		},
		{
			name: "получение существующего пользователя",
			initialUpdates: map[string]time.Time{
				"user_1": time1,
			},
			userIDToGet:  "user_1",
			expectedTime: time1,
			expectedErr:  "",
		},
		{
			name:           "обновление нового пользователя и последующее получение",
			initialUpdates: nil,
			userIDToUpdate: "user_2",
			timeToUpdate:   time1,
			userIDToGet:    "user_2",
			expectedTime:   time1,
			expectedErr:    "",
		},
		{
			name: "перезапись времени существующего пользователя",
			initialUpdates: map[string]time.Time{
				"user_1": time1,
			},
			userIDToUpdate: "user_1",
			timeToUpdate:   time2,
			userIDToGet:    "user_1",
			expectedTime:   time2,
			expectedErr:    "",
		},
		{
			name: "получение пользователя, которого нет, но есть другие",
			initialUpdates: map[string]time.Time{
				"user_1": time1,
			},
			userIDToUpdate: "user_1",
			timeToUpdate:   time2,
			userIDToGet:    "user_99",
			expectedTime:   time.Time{},
			expectedErr:    "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewUpdateUsers()

			for uid, ts := range tt.initialUpdates {
				repo.Update(uid, ts)
			}

			if tt.userIDToUpdate != "" {
				repo.Update(tt.userIDToUpdate, tt.timeToUpdate)
			}

			gotTime, gotErr := repo.Get(tt.userIDToGet)

			if tt.expectedErr != "" {
				require.Error(t, gotErr)
				assert.Equal(t, tt.expectedErr, gotErr.Error())
			} else {
				require.NoError(t, gotErr)
			}

			assert.Equal(t, tt.expectedTime, gotTime)
		})
	}
}

func TestUpdateUsers_Concurrency(t *testing.T) {
	repo := NewUpdateUsers()
	const numGoroutines = 500

	var wg sync.WaitGroup
	targetUser := "concurrent_user"

	for i := range numGoroutines {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			repo.Update(targetUser, time.Now().Add(time.Duration(iteration)*time.Millisecond))
		}(i)
	}

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.Get(targetUser)
		}()
	}

	wg.Wait()

	finalTime, err := repo.Get(targetUser)
	require.NoError(t, err)
	assert.NotEqual(t, time.Time{}, finalTime)
}
