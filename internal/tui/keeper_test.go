package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/stretchr/testify/mock"

	mocktui "github.com/eshadow1/gophkeeper/mocks/tui"

	"github.com/stretchr/testify/assert"
)

func TestNewModelAndInit(t *testing.T) {
	mockKeeper := mocktui.NewMockKeeperClient(t)
	mockDoing := mocktui.NewMockDoingModel(t)

	m := NewModel(mockKeeper, mockDoing)

	assert.Equal(t, ScreenAuth, m.screen)
	assert.Equal(t, 0, m.menuCursor)
	assert.NotNil(t, m.decrypted)
	assert.False(t, m.quitting)
	assert.NotNil(t, m.inputs)

	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestModelTUI_Update_Messages(t *testing.T) {
	tests := []struct {
		name         string
		initialSetup func(m *modelTUI)
		msg          tea.Msg
		mockSetup    func(k *mocktui.MockKeeperClient)
		assertState  func(t *testing.T, m *modelTUI)
	}{
		{
			name:         "WindowSizeMsg обновляет размеры",
			initialSetup: func(m *modelTUI) {},
			msg:          tea.WindowSizeMsg{Width: 100, Height: 50},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, 100, m.width)
				assert.Equal(t, 50, m.height)
			},
		},
		{
			name:         "ctrl+c на ScreenAuth вызывает выход",
			initialSetup: func(m *modelTUI) { m.screen = ScreenAuth },
			msg:          tea.KeyMsg{Type: tea.KeyCtrlC},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.True(t, m.quitting)
			},
		},
		{
			name:         "authSuccessMsg сбрасывает ошибки и меняет экран",
			initialSetup: func(m *modelTUI) { m.errMsg = "old error"; m.screen = ScreenLoginForm },
			msg:          authSuccessMsg{},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, ScreenMainMenu, m.screen)
				assert.Empty(t, m.errMsg)
				assert.Equal(t, "Успешная аутентификация", m.statusMsg)
			},
		},
		{
			name:         "authErrorMsg устанавливает текст ошибки",
			initialSetup: func(m *modelTUI) { m.loading = true },
			msg:          authErrorMsg{err: errors.New("bad credentials")},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.loading)
				assert.Contains(t, m.errMsg, "bad credentials")
			},
		},
		{
			name:         "syncDoneMsg с ошибкой",
			initialSetup: func(m *modelTUI) { m.loading = true },
			msg:          syncDoneMsg{err: errors.New("network fail")},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.loading)
				assert.Contains(t, m.errMsg, "network fail")
			},
		},
		{
			name:         "syncDoneMsg успешно (ТРЕБУЕТСЯ МОК ListItems)",
			initialSetup: func(m *modelTUI) { m.loading = true },
			msg:          syncDoneMsg{err: nil},
			mockSetup: func(k *mocktui.MockKeeperClient) {
				k.EXPECT().ListItems().Return([]*model.Item{})
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.loading)
				assert.NotNil(t, m.items)
				assert.Contains(t, m.statusMsg, "Синхронизация завершена")
			},
		},
		{
			name: "addItemDoneMsg успешно (ТРЕБУЕТСЯ МОК ListItems)",
			initialSetup: func(m *modelTUI) {
				m.loading = true
				m.isEditing = true
				m.editingItemID = "123"
			},
			msg: addItemDoneMsg{err: nil},
			mockSetup: func(k *mocktui.MockKeeperClient) {
				k.EXPECT().ListItems().Return([]*model.Item{{ID: "123"}})
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.loading)
				assert.False(t, m.isEditing)
				assert.Empty(t, m.editingItemID)
				assert.Equal(t, "Данные успешно обновлены", m.statusMsg)
				assert.Equal(t, ScreenListItems, m.screen)
			},
		},
		{
			name: "logoutDoneMsg сбрасывает состояние",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenMainMenu
				m.items = []*model.Item{{ID: "1"}}
				m.decrypted["1"] = "data"
			},
			msg: logoutDoneMsg{},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, ScreenAuth, m.screen)
				assert.Nil(t, m.items)
				assert.Empty(t, m.decrypted)
				assert.Equal(t, "Сессия завершена", m.statusMsg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mocktui.NewMockKeeperClient(t)
			mockDoing := mocktui.NewMockDoingModel(t)

			if tt.mockSetup != nil {
				tt.mockSetup(mockKeeper)
			}

			m := NewModel(mockKeeper, mockDoing)
			tt.initialSetup(m)

			updatedModel, _ := m.Update(tt.msg)
			m = updatedModel.(*modelTUI)

			tt.assertState(t, m)
		})
	}
}

func TestModelTUI_Update_MenuAndForms(t *testing.T) {
	tests := []struct {
		name         string
		initialSetup func(m *modelTUI)
		msg          tea.Msg
		mockSetup    func(d *mocktui.MockDoingModel)
		assertState  func(t *testing.T, m *modelTUI)
	}{
		{
			name:         "Навигация в меню: вниз",
			initialSetup: func(m *modelTUI) { m.screen = ScreenAuth; m.menuCursor = 0 },
			msg:          tea.KeyMsg{Type: tea.KeyDown},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, 1, m.menuCursor)
			},
		},
		{
			name: "Выбор Login в ScreenAuth (Enter)",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenAuth
				m.menuCursor = 0 // tabLogin
			},
			msg: tea.KeyMsg{Type: tea.KeyEnter},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, ScreenLoginForm, m.screen)
				assert.Equal(t, 0, m.activeFieldIdx)
				assert.True(t, m.inputs["username"].Focused())
			},
		},
		{
			name: "Форма: переключение по Tab",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenLoginForm
				m.currentForm = []string{"username", "password"}
				m.activeFieldIdx = 0
				m.inputs["username"].Focus()
			},
			msg: tea.KeyMsg{Type: tea.KeyTab},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, 1, m.activeFieldIdx)
				assert.True(t, m.inputs["password"].Focused())
				assert.False(t, m.inputs["username"].Focused())
			},
		},
		{
			name: "Форма: отправка с ошибкой валидации",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenLoginForm
				m.currentForm = []string{"username", "password"}
				m.activeFieldIdx = 1
				m.inputs["username"].SetValue("")
				m.inputs["password"].SetValue("pass")
				m.inputs["password"].Focus()
			},
			msg: tea.KeyMsg{Type: tea.KeyEnter},
			assertState: func(t *testing.T, m *modelTUI) {
				// Новое сообщение об ошибке из baseValidate
				assert.Contains(t, m.errMsg, "поле обязательно для заполнения")
			},
		},
		{
			name: "Форма: успешная отправка Login",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenLoginForm
				m.currentForm = []string{"username", "password"}
				m.activeFieldIdx = 1
				m.inputs["username"].SetValue("user")
				m.inputs["password"].SetValue("pass")
				m.inputs["password"].Focus()
			},
			msg: tea.KeyMsg{Type: tea.KeyEnter},
			mockSetup: func(d *mocktui.MockDoingModel) {
				d.EXPECT().Login("user", "pass").Return(func() tea.Msg { return authSuccessMsg{} })
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.True(t, m.loading)
				assert.Empty(t, m.errMsg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mocktui.NewMockKeeperClient(t)
			mockDoing := mocktui.NewMockDoingModel(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockDoing)
			}

			m := NewModel(mockKeeper, mockDoing)
			tt.initialSetup(m)

			updatedModel, _ := m.Update(tt.msg)
			m = updatedModel.(*modelTUI)

			tt.assertState(t, m)
		})
	}
}

func TestModelTUI_Update_ListItemsAndEdit(t *testing.T) {
	testItem := &model.Item{
		ID:        "item-1",
		DataType:  model.TextItem,
		MetaInfo:  "test meta",
		CreatedAt: time.Now(),
	}

	tests := []struct {
		name         string
		initialSetup func(m *modelTUI)
		msg          tea.Msg
		mockSetup    func(k *mocktui.MockKeeperClient, d *mocktui.MockDoingModel)
		assertState  func(t *testing.T, m *modelTUI)
	}{
		{
			name: "Список: навигация вниз",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{testItem, {ID: "item-2"}}
				m.listCursor = 0
			},
			msg: tea.KeyMsg{Type: tea.KeyDown},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.Equal(t, 1, m.listCursor)
			},
		},
		{
			name: "Список: нажатие 'd' запускает подтверждение удаления",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{testItem}
				m.listCursor = 0
			},
			msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.True(t, m.awaitingDeleteConfirm)
			},
		},
		{
			name: "Список: подтверждение удаления ('y')",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{testItem}
				m.listCursor = 0
				m.awaitingDeleteConfirm = true
			},
			msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")},
			mockSetup: func(k *mocktui.MockKeeperClient, d *mocktui.MockDoingModel) {
				d.EXPECT().DeleteItem("item-1").Return(func() tea.Msg { return syncDoneMsg{err: nil} })
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.awaitingDeleteConfirm)
				assert.True(t, m.loading)
			},
		},
		{
			name: "Список: отмена удаления ('n')",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{testItem}
				m.awaitingDeleteConfirm = true
			},
			msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.False(t, m.awaitingDeleteConfirm)
			},
		},
		{
			name: "Список: нажатие 'e' начинает редактирование",
			initialSetup: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{testItem}
				m.listCursor = 0
			},
			msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")},
			mockSetup: func(k *mocktui.MockKeeperClient, d *mocktui.MockDoingModel) {
				k.EXPECT().DecryptAndParse(testItem, mock.Anything).Run(
					func(item *model.Item, p any) {
						if textPayload, ok := p.(*model.TextPayload); ok {
							textPayload.Content = "decrypted content"
						}
					}).Return(nil)
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.True(t, m.isEditing)
				assert.Equal(t, "item-1", m.editingItemID)
				assert.Equal(t, ScreenAddText, m.screen)
				assert.Equal(t, "decrypted content", m.inputs["content"].Value())
			},
		},
		{
			name:         "Список: нажатие 's' запускает синхронизацию",
			initialSetup: func(m *modelTUI) { m.screen = ScreenListItems },
			msg:          tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")},
			mockSetup: func(k *mocktui.MockKeeperClient, d *mocktui.MockDoingModel) {
				d.EXPECT().Sync().Return(func() tea.Msg { return syncDoneMsg{err: nil} })
			},
			assertState: func(t *testing.T, m *modelTUI) {
				assert.True(t, m.loading)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mocktui.NewMockKeeperClient(t)
			mockDoing := mocktui.NewMockDoingModel(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockKeeper, mockDoing)
			}

			m := NewModel(mockKeeper, mockDoing)
			tt.initialSetup(m)

			updatedModel, _ := m.Update(tt.msg)
			m = updatedModel.(*modelTUI)

			tt.assertState(t, m)
		})
	}
}

func TestModelTUI_View(t *testing.T) {
	tests := []struct {
		name         string
		setupModel   func(m *modelTUI)
		mockSetup    func(k *mocktui.MockKeeperClient)
		assertOutput func(t *testing.T, output string)
	}{
		{
			name:       "View: экран выхода (quitting)",
			setupModel: func(m *modelTUI) { m.quitting = true },
			assertOutput: func(t *testing.T, output string) {
				assert.Equal(t, "До свидания!\n", output)
			},
		},
		{
			name: "View: экран аутентификации",
			setupModel: func(m *modelTUI) {
				m.screen = ScreenAuth
				m.errMsg = "Test Error"
				m.statusMsg = "Test Status"
				m.loading = true
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "GophKeeper")
				assert.Contains(t, output, "Добро пожаловать")
				assert.Contains(t, output, "Test Error")
				assert.Contains(t, output, "Test Status")
				assert.Contains(t, output, "Загрузка...")
			},
		},
		{
			name: "View: форма входа",
			setupModel: func(m *modelTUI) {
				m.screen = ScreenLoginForm
				m.currentForm = []string{"username", "password"}
				m.activeFieldIdx = 0
				m.inputs["username"].Focus()
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Вход в систему")
				assert.Contains(t, output, "Имя пользователя:")
				assert.Contains(t, output, "Пароль:")
				assert.Contains(t, output, "tab переключение")
			},
		},
		{
			name: "View: пустой список элементов",
			setupModel: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{}
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Мои данные")
				assert.Contains(t, output, "Список пуст. Нажмите 's' для синхронизации")
			},
		},
		{
			name: "View: список с элементами и подтверждением удаления",
			setupModel: func(m *modelTUI) {
				m.screen = ScreenListItems
				m.items = []*model.Item{{ID: "123", DataType: model.TextItem, MetaInfo: "meta", CreatedAt: time.Now()}}
				m.listCursor = 0
				m.awaitingDeleteConfirm = true
			},
			mockSetup: func(k *mocktui.MockKeeperClient) {
				k.EXPECT().
					DecryptAndParse(mock.Anything, mock.Anything).
					Return(nil).
					Maybe()
			},
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "ID: 123")
				assert.Contains(t, output, "Тип: text")
				assert.Contains(t, output, "Вы уверены, что хотите удалить запись 123?")
				assert.Contains(t, output, "y подтвердить удаление")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mocktui.NewMockKeeperClient(t)

			if tt.mockSetup != nil {
				tt.mockSetup(mockKeeper)
			}

			m := NewModel(mockKeeper, mocktui.NewMockDoingModel(t))
			tt.setupModel(m)

			output := m.View()
			tt.assertOutput(t, output)
		})
	}
}

func TestModelTUI_Helpers(t *testing.T) {
	m := NewModel(mocktui.NewMockKeeperClient(t), mocktui.NewMockDoingModel(t))

	m.inputs["login"].SetValue("user")
	m.inputs["password"].SetValue("pass")
	m.resetForms()
	assert.Empty(t, m.inputs["login"].Value())
	assert.Empty(t, m.inputs["password"].Value())

	m.inputs["username"].Focus()
	m.blurAll()
	assert.False(t, m.inputs["username"].Focused())

	m.isEditing = true
	m.editingItemID = "1"
	m.screen = ScreenAddText
	m.handleEscape()
	assert.False(t, m.isEditing)
	assert.Empty(t, m.editingItemID)
	assert.Equal(t, ScreenListItems, m.screen)
	assert.Equal(t, "Редактирование отменено", m.statusMsg)
}
