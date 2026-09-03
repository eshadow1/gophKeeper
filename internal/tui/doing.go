// Package tui реализует слой терминального пользовательского интерфейса (TUI)
// с использованием фреймворка Bubble Tea.
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	models "github.com/eshadow1/gophkeeper/internal/model"
)

type (
	authSuccessMsg struct{}
	authErrorMsg   struct{ err error }
	syncDoneMsg    struct{ err error }
	addItemDoneMsg struct{ err error }
	logoutDoneMsg  struct{}
)

type doModel struct {
	keeper KeeperClient
}

// NewDoModel создает и возвращает новый экземпляр модели выполнения
// команд doModel, инициализированный переданным клиентом KeeperClient.
func NewDoModel(keeper KeeperClient) *doModel {
	return &doModel{keeper: keeper}
}

// Login возвращает асинхронную команду (tea.Cmd) для выполнения входа
// пользователя в систему. После успешной аутентификации автоматически
// инициирует первичную синхронизацию данных с сервером для получения
// актуального состояния хранилища.
func (m doModel) Login(u, p string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if errLogin := m.keeper.Login(ctx, u, p); errLogin != nil {
			return authErrorMsg{errLogin}
		}
		if errSync := m.keeper.SyncFromServer(ctx); errSync != nil {
			return authErrorMsg{errSync}
		}
		return authSuccessMsg{}
	}
}

// Register возвращает асинхронную команду (tea.Cmd) для регистрации
// нового пользователя. Аналогично методу Login, после успешной
// регистрации выполняет первичную синхронизацию данных.
func (m doModel) Register(u, p string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := m.keeper.Register(ctx, u, p); err != nil {
			return authErrorMsg{err}
		}
		if errSync := m.keeper.SyncFromServer(ctx); errSync != nil {
			return authErrorMsg{errSync}
		}
		return authSuccessMsg{}
	}
}

// Sync возвращает асинхронную команду для принудительной синхронизации
// локальных данных с сервером.
func (m doModel) Sync() tea.Cmd {
	return func() tea.Msg { return syncDoneMsg{err: m.keeper.SyncFromServer(context.Background())} }
}

// AddItem возвращает асинхронную команду для добавления нового
// зашифрованного элемента на сервер.
func (m doModel) AddItem(dt models.ItemType, payload any, meta string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errAdd := m.keeper.AddItem(ctx, dt, payload, meta)
		if errAdd != nil {
			return addItemDoneMsg{err: errAdd}
		}
		_ = m.keeper.SyncFromServer(ctx)
		return addItemDoneMsg{err: nil}
	}
}

// UpdateItem возвращает асинхронную команду для обновления существующего
// элемента на сервере.
func (m doModel) UpdateItem(id string, dt models.ItemType, payload any, meta string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errUpdate := m.keeper.UpdateItem(ctx, id, dt, payload, meta)
		if errUpdate != nil {
			return addItemDoneMsg{err: errUpdate}
		}
		_ = m.keeper.SyncFromServer(ctx)
		return addItemDoneMsg{err: nil}
	}
}

// DeleteItem возвращает асинхронную команду для удаления элемента
// с сервера по его идентификатору. После успешного удаления инициирует
// фоновую синхронизацию для удаления элемента из локального кэша.
func (m doModel) DeleteItem(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := m.keeper.DeleteItem(ctx, id)
		if err != nil {
			return authErrorMsg{err}
		}
		_ = m.keeper.SyncFromServer(ctx)
		return syncDoneMsg{err: nil}
	}
}

// Logout возвращает асинхронную команду для выполнения выхода из системы.
// Вызывает метод очистки сессии на стороне клиента.
func (m doModel) Logout() tea.Cmd {
	return func() tea.Msg { _ = m.keeper.Logout(); return logoutDoneMsg{} }
}
