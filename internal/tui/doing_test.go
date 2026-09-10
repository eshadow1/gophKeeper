package tui

import (
	"errors"
	"testing"

	"github.com/eshadow1/gophkeeper/internal/model"

	mocktui "github.com/eshadow1/gophkeeper/mocks/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDoModel_Login(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name:     "успешный вход и синхронизация",
			username: "user",
			password: "pass",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Login(mock.Anything, "user", "pass").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: authSuccessMsg{},
			wantErr:     nil,
		},
		{
			name:     "ошибка при входе (синхронизация не вызывается)",
			username: "user",
			password: "wrong_pass",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Login(mock.Anything, "user", "wrong_pass").Return(errors.New("invalid credentials"))
			},
			wantMsgType: authErrorMsg{},
			wantErr:     errors.New("invalid credentials"),
		},
		{
			name:     "ошибка при синхронизации после успешного входа",
			username: "user",
			password: "pass",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Login(mock.Anything, "user", "pass").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(errors.New("network timeout"))
			},
			wantMsgType: authErrorMsg{},
			wantErr:     errors.New("network timeout"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			cmd := m.Login(tt.username, tt.password)

			msg := cmd()

			require.IsType(t, tt.wantMsgType, msg)

			if tt.wantErr != nil {
				switch v := msg.(type) {
				case authErrorMsg:
					assert.EqualError(t, v.err, tt.wantErr.Error())
				default:
					require.Nil(t, msg)
				}
			}
		})
	}
}

func TestDoModel_Register(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name:     "успешная регистрация и синхронизация",
			username: "newuser",
			password: "newpass",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Register(mock.Anything, "newuser", "newpass").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: authSuccessMsg{},
			wantErr:     nil,
		},
		{
			name:     "ошибка при регистрации",
			username: "existinguser",
			password: "pass",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Register(mock.Anything, "existinguser", "pass").Return(errors.New("user exists"))
			},
			wantMsgType: authErrorMsg{},
			wantErr:     errors.New("user exists"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.Register(tt.username, tt.password)()

			require.IsType(t, tt.wantMsgType, msg)
			if tt.wantErr != nil {
				assert.EqualError(t, msg.(authErrorMsg).err, tt.wantErr.Error())
			}
		})
	}
}

func TestDoModel_Sync(t *testing.T) {
	tests := []struct {
		name        string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name: "успешная синхронизация",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: syncDoneMsg{},
			wantErr:     nil,
		},
		{
			name: "ошибка синхронизации",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().SyncFromServer(mock.Anything).Return(errors.New("sync failed"))
			},
			wantMsgType: syncDoneMsg{},
			wantErr:     errors.New("sync failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.Sync()()

			require.IsType(t, tt.wantMsgType, msg)
			if tt.wantErr != nil {
				assert.EqualError(t, msg.(syncDoneMsg).err, tt.wantErr.Error())
			} else {
				assert.NoError(t, msg.(syncDoneMsg).err)
			}
		})
	}
}

func TestDoModel_AddItem(t *testing.T) {
	testPayload := model.TextPayload{Content: "secret"}

	tests := []struct {
		name        string
		dataType    model.ItemType
		payload     any
		meta        string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name:     "успешное добавление и последующая синхронизация",
			dataType: model.TextItem,
			payload:  testPayload,
			meta:     "meta_info",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().AddItem(mock.Anything, model.TextItem, testPayload, "meta_info").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: addItemDoneMsg{},
			wantErr:     nil,
		},
		{
			name:     "ошибка при добавлении (синхронизация не вызывается)",
			dataType: model.TextItem,
			payload:  testPayload,
			meta:     "meta_info",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().AddItem(mock.Anything, model.TextItem, testPayload, "meta_info").Return(errors.New("validation error"))
			},
			wantMsgType: addItemDoneMsg{},
			wantErr:     errors.New("validation error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.AddItem(tt.dataType, tt.payload, tt.meta)()

			require.IsType(t, tt.wantMsgType, msg)
			if tt.wantErr != nil {
				assert.EqualError(t, msg.(addItemDoneMsg).err, tt.wantErr.Error())
			} else {
				assert.NoError(t, msg.(addItemDoneMsg).err)
			}
		})
	}
}

func TestDoModel_UpdateItem(t *testing.T) {
	testPayload := model.TextPayload{Content: "updated_secret"}

	tests := []struct {
		name        string
		id          string
		dataType    model.ItemType
		payload     any
		meta        string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name:     "успешное обновление и последующая синхронизация",
			id:       "item-123",
			dataType: model.TextItem,
			payload:  testPayload,
			meta:     "new_meta",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().UpdateItem(mock.Anything, "item-123", model.TextItem, testPayload, "new_meta").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: addItemDoneMsg{},
			wantErr:     nil,
		},
		{
			name:     "ошибка при обновлении",
			id:       "item-123",
			dataType: model.TextItem,
			payload:  testPayload,
			meta:     "new_meta",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().UpdateItem(mock.Anything, "item-123", model.TextItem, testPayload, "new_meta").Return(errors.New("not found"))
			},
			wantMsgType: addItemDoneMsg{},
			wantErr:     errors.New("not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.UpdateItem(tt.id, tt.dataType, tt.payload, tt.meta)()

			require.IsType(t, tt.wantMsgType, msg)
			if tt.wantErr != nil {
				assert.EqualError(t, msg.(addItemDoneMsg).err, tt.wantErr.Error())
			} else {
				assert.NoError(t, msg.(addItemDoneMsg).err)
			}
		})
	}
}

func TestDoModel_DeleteItem(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
		wantErr     error
	}{
		{
			name: "успешное удаление и последующая синхронизация",
			id:   "item-123",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().DeleteItem(mock.Anything, "item-123").Return(nil)
				m.EXPECT().SyncFromServer(mock.Anything).Return(nil)
			},
			wantMsgType: syncDoneMsg{},
			wantErr:     nil,
		},
		{
			name: "ошибка при удалении",
			id:   "item-999",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().DeleteItem(mock.Anything, "item-999").Return(errors.New("delete failed"))
			},
			wantMsgType: authErrorMsg{},
			wantErr:     errors.New("delete failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.DeleteItem(tt.id)()

			require.IsType(t, tt.wantMsgType, msg)
			if tt.wantErr != nil {
				switch v := msg.(type) {
				case authErrorMsg:
					assert.EqualError(t, v.err, tt.wantErr.Error())
				case syncDoneMsg:
					assert.EqualError(t, v.err, tt.wantErr.Error())
				}
			}
		})
	}
}

func TestDoModel_Logout(t *testing.T) {
	tests := []struct {
		name        string
		mockSetup   func(m *mocktui.MockKeeperClient)
		wantMsgType any
	}{
		{
			name: "успешный выход",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Logout().Return(nil)
			},
			wantMsgType: logoutDoneMsg{},
		},
		{
			name: "выход с игнорированием ошибки (если Logout вернет ошибку, она игнорируется в коде)",
			mockSetup: func(m *mocktui.MockKeeperClient) {
				m.EXPECT().Logout().Return(errors.New("cleanup failed"))
			},
			wantMsgType: logoutDoneMsg{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocktui.NewMockKeeperClient(t)
			tt.mockSetup(mockClient)

			m := NewDoModel(mockClient)
			msg := m.Logout()()

			require.IsType(t, tt.wantMsgType, msg)
		})
	}
}
