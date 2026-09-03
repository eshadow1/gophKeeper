package tui

import (
	"context"
	"fmt"
	"strings"

	models "github.com/eshadow1/gophkeeper/internal/model"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	esc  = "esc"
	down = "down"
	up   = "up"
)

// KeeperClient описывает интерфейс для работы с бизнес-логикой клиента GophKeeper.
type KeeperClient interface {
	// Register регистрирует нового пользователя.
	Register(ctx context.Context, username, password string) error
	// Login выполняет аутентификацию пользователя.
	Login(ctx context.Context, username, password string) error
	// Logout завершает текущую сессию и очищает локальные данные аутентификации.
	Logout() error
	// LoadToken загружает сохраненный токен аутентификации из локального хранилища.
	LoadToken() error
	// SyncFromServer синхронизирует локальные данные с сервером.
	SyncFromServer(ctx context.Context) error
	// AddItem добавляет новый зашифрованный элемент на сервер.
	AddItem(ctx context.Context, dataType models.ItemType, payload any, metaInfo string) error
	// UpdateItem обновляет существующий зашифрованный элемент на сервере.
	UpdateItem(ctx context.Context, id string, dataType models.ItemType, payload any, metaInfo string) error
	// DeleteItem удаляет элемент с сервера по его идентификатору.
	DeleteItem(ctx context.Context, id string) error
	// DecryptAndParse расшифровывает данные элемента и десериализует их в предоставленную структуру payload.
	DecryptAndParse(item *models.Item, payload any) error
	// ListItems возвращает список всех локально сохраненных элементов пользователя.
	ListItems() []*models.Item
}

// DoingModel определяет интерфейс для асинхронных команд Bubble Tea (tea.Cmd)
type DoingModel interface {
	// Login возвращает команду для выполнения входа в систему.
	Login(u, p string) tea.Cmd
	// Logout возвращает команду для выхода из системы.
	Logout() tea.Cmd
	// Register возвращает команду для регистрации пользователя.
	Register(u, p string) tea.Cmd
	// AddItem возвращает команду для добавления нового элемента.
	AddItem(dt models.ItemType, payload any, meta string) tea.Cmd
	// UpdateItem возвращает команду для обновления существующего элемента.
	UpdateItem(id string, dt models.ItemType, payload any, meta string) tea.Cmd
	// DeleteItem возвращает команду для удаления элемента.
	DeleteItem(id string) tea.Cmd
	// Sync возвращает команду для синхронизации данных с сервером.
	Sync() tea.Cmd
}

// Screen представляет собой перечисление возможных экранов пользовательского интерфейса.
type Screen int

const (
	ScreenAuth         Screen = iota // Экран первоначальной аутентификации (выбор между входом и регистрацией).
	ScreenLoginForm                  // Экран формы ввода логина и пароля для входа.
	ScreenRegisterForm               // Экран формы ввода данных для регистрации.
	ScreenMainMenu                   // Главное меню авторизованного пользователя.
	ScreenListItems                  // Экран просмотра списка сохраненных элементов.
	ScreenAddLogin                   // Экран формы добавления/редактирования учетных данных (логин/пароль).
	ScreenAddText                    // Экран формы добавления/редактирования текстовой заметки.
	ScreenAddCard                    // Экран формы добавления/редактирования данных банковской карты.
	ScreenAddBinary                  // Экран формы добавления/редактирования бинарного файла.

	tabLogin    = 0
	tabRegister = 1
	tabQuit     = 2

	tabListItems = 0
	tabAddLogin  = 1
	tabAddText   = 2
	tabAddCard   = 3
	tabAddBinary = 4
	tabLogout    = 5
)

// modelTUI представляет собой основную модель состояния приложения Bubble Tea.
type modelTUI struct {
	keeper     KeeperClient
	doing      DoingModel
	screen     Screen
	menuCursor int
	menuItems  []menuOption

	usernameInput, passwordInput, loginInput    textinput.Model
	cardNumber, cardHolder, cardExpiry, cardCVV textinput.Model
	contentInput, metaInput, filePathInput      textinput.Model
	activeField                                 int

	items             []*models.Item
	decrypted         map[string]any
	statusMsg, errMsg string
	loading, quitting bool
	width, height     int

	listCursor            int
	isEditing             bool
	editingItemID         string
	awaitingDeleteConfirm bool
}

var (
	// titleStyle стиль для заголовков экранов.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
	// menuItemStyle стиль для обычных пунктов меню.
	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)
	// selectedItemStyle стиль для выбранного (активного) пункта меню.
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)
	// statusStyle стиль для информационных сообщений об успехе.
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8FF60")).
			Italic(true)
	// errorStyle стиль для сообщений об ошибках.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			Bold(true)
	// inputStyle стиль для неактивных полей ввода.
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)
	// activeInputStyle стиль для активного (сфокусированного) поля ввода.
	activeInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFD700")).
				Padding(0, 1).
				MarginBottom(1)
	// helpStyle стиль для подсказок по управлению.
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)
	// contentStyle стиль для основного контейнера контента.
	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)
)

func newInput(placeholder string, charLimit, width int, echoPwd bool) textinput.Model {
	i := textinput.New()
	i.Placeholder = placeholder
	i.CharLimit = charLimit
	i.Width = width
	if echoPwd {
		i.EchoMode = textinput.EchoPassword
	}
	return i
}

// NewModel создает и инициализирует новый экземпляр основной модели TUI.
func NewModel(keeper KeeperClient, doing DoingModel) modelTUI {
	return modelTUI{
		keeper:        keeper,
		doing:         doing,
		screen:        ScreenAuth,
		menuItems:     defaultMenu(),
		usernameInput: newInput("Имя пользователя", 64, 40, false),
		passwordInput: newInput("Пароль", 64, 40, true),
		loginInput:    newInput("Логин для сервиса", 128, 40, false),
		cardNumber:    newInput("Номер карты (1234 5678 9012 3456)", 19, 40, false),
		cardHolder:    newInput("Владелец карты", 64, 40, false),
		cardExpiry:    newInput("Срок действия (MM/YY)", 5, 20, false),
		cardCVV:       newInput("CVV", 4, 10, true),
		contentInput:  newInput("Текстовое содержимое", 1024, 60, false),
		metaInput:     newInput("Метаинформация (необязательно)", 256, 60, false),
		filePathInput: newInput("Путь к файлу", 512, 60, false),
		decrypted:     make(map[string]any),
	}
}

// Init возвращает начальную команду для запуска анимации мигания курсора
// в первом активном поле ввода при старте приложения.
func (*modelTUI) Init() tea.Cmd { return textinput.Blink }

// Update является основным методом обработки сообщений в архитектуре Bubble Tea.
func (m *modelTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if (m.screen == ScreenAuth) || (m.screen == ScreenMainMenu && m.menuCursor == 5) {
				m.quitting = true
				return m, tea.Quit
			}
			if m.screen != ScreenAuth && m.screen != ScreenMainMenu {
				return m.handleEscape()
			}
		case "q":
			if (m.screen == ScreenAuth) || (m.screen == ScreenMainMenu && m.menuCursor == 5) {
				m.quitting = true
				return m, tea.Quit
			}
		case esc:
			return m.handleEscape()
		}
	case authSuccessMsg:
		m.loading, m.errMsg = false, ""
		m.screen, m.menuItems, m.menuCursor = ScreenMainMenu, mainMenu(), 0
		m.statusMsg = "Успешная аутентификация"
		return m, nil
	case authErrorMsg:
		m.loading = false
		m.errMsg = fmt.Sprintf("Ошибка: %v", msg.err)
		return m, nil
	case syncDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Ошибка синхронизации: %v", msg.err)
		} else {
			m.items = m.keeper.ListItems()
			if m.awaitingDeleteConfirm {
				m.awaitingDeleteConfirm = false
				m.statusMsg = "Запись успешно удалена"
			} else if !m.isEditing {
				m.statusMsg = fmt.Sprintf("Синхронизация завершена. Загружено элементов: %d", len(m.items))
			}

			if m.listCursor >= len(m.items) && len(m.items) > 0 {
				m.listCursor = len(m.items) - 1
			} else if len(m.items) == 0 {
				m.listCursor = 0
			}
		}
		return m, nil
	case addItemDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Ошибка: %v", msg.err)
		} else {
			if m.isEditing {
				m.statusMsg = "Данные успешно обновлены"
				m.isEditing = false
				m.editingItemID = ""
			} else {
				m.statusMsg = "Данные успешно добавлены и зашифрованы"
			}
			m.resetForms()
			m.screen = ScreenListItems
			m.items = m.keeper.ListItems()
			if m.listCursor >= len(m.items) && len(m.items) > 0 {
				m.listCursor = len(m.items) - 1
			}
		}
		return m, nil
	case logoutDoneMsg:
		m.loading = false
		m.screen, m.menuItems, m.menuCursor = ScreenAuth, defaultMenu(), 0
		m.items, m.decrypted = nil, make(map[string]any)
		m.statusMsg = "Сессия завершена"
		return m, nil
	}

	switch m.screen {
	case ScreenAuth, ScreenMainMenu:
		return m.updateMenu(msg)
	case ScreenLoginForm, ScreenRegisterForm:
		return m.updateForm(
			msg,
			[]*textinput.Model{&m.usernameInput, &m.passwordInput},
			func(m *modelTUI) error {
				if m.usernameInput.Value() == "" || m.passwordInput.Value() == "" {
					return fmt.Errorf("все поля должны быть заполнены")
				}
				return nil
			},
			func(m *modelTUI) tea.Cmd {
				m.loading, m.errMsg = true, ""
				if m.screen == ScreenLoginForm {
					return m.doing.Login(m.usernameInput.Value(), m.passwordInput.Value())
				}
				return m.doing.Register(m.usernameInput.Value(), m.passwordInput.Value())
			},
		)
	case ScreenAddLogin:
		return m.updateForm(
			msg,
			[]*textinput.Model{&m.loginInput, &m.passwordInput, &m.metaInput},
			func(m *modelTUI) error {
				if m.loginInput.Value() == "" || m.passwordInput.Value() == "" {
					return fmt.Errorf("логин и пароль обязательны")
				}
				return nil
			},
			func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.LoginPayload{Username: m.loginInput.Value(), Password: m.passwordInput.Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.LoginItem, payload, m.metaInput.Value())
				}
				return m.doing.AddItem(models.LoginItem, payload, m.metaInput.Value())
			},
		)
	case ScreenAddText:
		return m.updateForm(
			msg,
			[]*textinput.Model{&m.contentInput, &m.metaInput},
			func(m *modelTUI) error {
				if m.contentInput.Value() == "" {
					return fmt.Errorf("содержимое не может быть пустым")
				}
				return nil
			},
			func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.TextPayload{Content: m.contentInput.Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.TextItem, payload, m.metaInput.Value())
				}
				return m.doing.AddItem(models.TextItem, payload, m.metaInput.Value())
			},
		)
	case ScreenAddCard:
		return m.updateForm(
			msg,
			[]*textinput.Model{&m.cardNumber, &m.cardHolder, &m.cardExpiry, &m.cardCVV, &m.metaInput},
			func(m *modelTUI) error {
				if m.cardNumber.Value() == "" || m.cardHolder.Value() == "" || m.cardExpiry.Value() == "" || m.cardCVV.Value() == "" {
					return fmt.Errorf("все поля карты обязательны")
				}
				return nil
			},
			func(m *modelTUI) tea.Cmd {
				m.loading = true
				parts := strings.Split(m.cardExpiry.Value(), "/")
				expM, expY := "", ""
				if len(parts) == 2 {
					expM, expY = parts[0], parts[1]
				}
				payload := models.CardPayload{
					Number: m.cardNumber.Value(), Holder: m.cardHolder.Value(),
					ExpiryMonth: expM, ExpiryYear: expY, CVV: m.cardCVV.Value(),
				}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.CardItem, payload, m.metaInput.Value())
				}
				return m.doing.AddItem(models.CardItem, payload, m.metaInput.Value())
			},
		)
	case ScreenAddBinary:
		return m.updateForm(
			msg,
			[]*textinput.Model{&m.filePathInput, &m.metaInput},
			func(m *modelTUI) error {
				if m.filePathInput.Value() == "" {
					return fmt.Errorf("путь к файлу обязателен")
				}
				return nil
			},
			func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.BinaryPayload{FilePath: m.filePathInput.Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.BinaryItem, payload, m.metaInput.Value())
				}
				return m.doing.AddItem(models.BinaryItem, payload, m.metaInput.Value())
			},
		)
	case ScreenListItems:
		return m.updateListItems(msg)
	}
	return m, nil
}

// handleEscape обрабатывает нажатие клавиши Esc
func (m *modelTUI) handleEscape() (tea.Model, tea.Cmd) {
	if m.isEditing {
		m.isEditing = false
		m.editingItemID = ""
		m.resetForms()
		m.screen = ScreenListItems
		m.errMsg = ""
		m.statusMsg = "Редактирование отменено"
		return m, nil
	}

	m.errMsg, m.statusMsg = "", ""
	m.awaitingDeleteConfirm = false

	if m.screen == ScreenListItems || m.screen == ScreenAddLogin ||
		m.screen == ScreenAddText || m.screen == ScreenAddCard || m.screen == ScreenAddBinary {
		m.screen, m.menuItems, m.menuCursor = ScreenMainMenu, mainMenu(), 0
	} else {
		m.screen, m.menuItems, m.menuCursor = ScreenAuth, defaultMenu(), 0
	}
	m.blurAll()
	return m, nil
}

// blurAll снимает фокус со всех полей ввода в модели, чтобы предотвратить
// нежелательный ввод текста при навигации по меню или спискам.
func (m *modelTUI) blurAll() {
	m.usernameInput.Blur()
	m.passwordInput.Blur()
	m.loginInput.Blur()
	m.contentInput.Blur()
	m.metaInput.Blur()
	m.cardNumber.Blur()
	m.cardHolder.Blur()
	m.cardExpiry.Blur()
	m.cardCVV.Blur()
	m.filePathInput.Blur()
}

// updateMenu обрабатывает навигацию и выбор пунктов в меню (ScreenAuth и ScreenMainMenu).
func (m *modelTUI) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case up, "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case down, "j":
			if m.menuCursor < len(m.menuItems)-1 {
				m.menuCursor++
			}
		case "enter", " ":
			m.errMsg, m.statusMsg = "", ""
			if m.screen == ScreenAuth {
				switch m.menuCursor {
				case tabLogin:
					m.screen, m.activeField = ScreenLoginForm, 0
					m.usernameInput.Focus()
					m.passwordInput.Blur()
					return m, textinput.Blink
				case tabRegister:
					m.screen, m.activeField = ScreenRegisterForm, 0
					m.usernameInput.Focus()
					m.passwordInput.Blur()
					return m, textinput.Blink
				case tabQuit:
					m.quitting = true
					return m, tea.Quit
				}
			} else {
				switch m.menuCursor {
				case tabListItems:
					if err := m.keeper.LoadToken(); err != nil {
						m.errMsg = fmt.Sprintf("Ошибка: %v", err)
						return m, nil
					}
					m.screen = ScreenListItems
					m.listCursor = 0
					m.loading = true
					return m, m.doing.Sync()
				case tabAddLogin:
					m.isEditing = false
					m.screen, m.activeField = ScreenAddLogin, 0
					m.loginInput.Focus()
					m.blurAllExcept(&m.loginInput)
					return m, textinput.Blink
				case tabAddText:
					m.isEditing = false
					m.screen, m.activeField = ScreenAddText, 0
					m.contentInput.Focus()
					m.blurAllExcept(&m.contentInput)
					return m, textinput.Blink
				case tabAddCard:
					m.isEditing = false
					m.screen, m.activeField = ScreenAddCard, 0
					m.cardNumber.Focus()
					m.blurAllExcept(&m.cardNumber)
					return m, textinput.Blink
				case tabAddBinary:
					m.isEditing = false
					m.screen, m.activeField = ScreenAddBinary, 0
					m.filePathInput.Focus()
					m.blurAllExcept(&m.filePathInput)
					return m, textinput.Blink
				case tabLogout:
					m.loading = true
					return m, m.doing.Logout()
				}
			}
		}
	}
	return m, nil
}

// blurAllExcept снимает фокус со всех полей ввода, кроме переданного, и устанавливает фокус на него.
func (m *modelTUI) blurAllExcept(focused *textinput.Model) {
	m.blurAll()
	focused.Focus()
}

// updateForm является универсальным обработчиком для всех форм ввода.
func (m *modelTUI) updateForm(
	msg tea.Msg,
	fields []*textinput.Model,
	validate func(m *modelTUI) error,
	submit func(m *modelTUI) tea.Cmd,
) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", down:
			fields[m.activeField].Blur()
			m.activeField = (m.activeField + 1) % len(fields)
			fields[m.activeField].Focus()
			return m, textinput.Blink
		case "shift+tab", up:
			fields[m.activeField].Blur()
			m.activeField = (m.activeField - 1 + len(fields)) % len(fields)
			fields[m.activeField].Focus()
			return m, textinput.Blink
		case "enter":
			if m.activeField < len(fields)-1 {
				fields[m.activeField].Blur()
				m.activeField++
				fields[m.activeField].Focus()
				return m, textinput.Blink
			}
			if validate != nil {
				if err := validate(m); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
			}
			return m, submit(m)
		}
	}

	updated, cmd := fields[m.activeField].Update(msg)
	*fields[m.activeField] = updated
	return m, cmd
}

// updateListItems обрабатывает навигацию по списку элементов и действия с ними
func (m *modelTUI) updateListItems(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.awaitingDeleteConfirm {
			switch key.String() {
			case "y", "Y":
				m.awaitingDeleteConfirm = false
				if len(m.items) > 0 {
					m.loading = true
					return m, m.doing.DeleteItem(m.items[m.listCursor].ID)
				}
			case "n", "N", esc:
				m.awaitingDeleteConfirm = false
				return m, nil
			}
			return m, nil
		}

		switch key.String() {
		case up, "k":
			if m.listCursor > 0 {
				m.listCursor--
			}
		case down, "j":
			if m.listCursor < len(m.items)-1 {
				m.listCursor++
			}
		case "d":
			if len(m.items) > 0 {
				m.awaitingDeleteConfirm = true
			}
		case "e":
			if len(m.items) > 0 {
				return m.startEdit(m.items[m.listCursor])
			}
		case "s":
			m.loading = true
			return m, m.doing.Sync()
		case esc:
			return m.handleEscape()
		}
	}
	return m, nil
}

// startEdit инициализирует режим редактирования для выбранного элемента.
func (m *modelTUI) startEdit(item *models.Item) (tea.Model, tea.Cmd) {
	m.isEditing = true
	m.editingItemID = item.ID
	m.errMsg = ""
	m.awaitingDeleteConfirm = false
	m.resetForms()

	var payload any
	switch item.DataType {
	case models.LoginItem:
		payload = &models.LoginPayload{}
		if err := m.keeper.DecryptAndParse(item, payload); err == nil {
			p := payload.(*models.LoginPayload)
			m.loginInput.SetValue(p.Username)
			m.passwordInput.SetValue(p.Password)
		}
		m.metaInput.SetValue(item.MetaInfo)
		m.screen = ScreenAddLogin
		m.activeField = 0
		m.blurAllExcept(&m.loginInput)

	case models.TextItem:
		payload = &models.TextPayload{}
		if err := m.keeper.DecryptAndParse(item, payload); err == nil {
			p := payload.(*models.TextPayload)
			m.contentInput.SetValue(p.Content)
		}
		m.metaInput.SetValue(item.MetaInfo)
		m.screen = ScreenAddText
		m.activeField = 0
		m.blurAllExcept(&m.contentInput)

	case models.CardItem:
		payload = &models.CardPayload{}
		if err := m.keeper.DecryptAndParse(item, payload); err == nil {
			p := payload.(*models.CardPayload)
			m.cardNumber.SetValue(p.Number)
			m.cardHolder.SetValue(p.Holder)
			m.cardExpiry.SetValue(fmt.Sprintf("%s/%s", p.ExpiryMonth, p.ExpiryYear))
			m.cardCVV.SetValue(p.CVV)
		}
		m.metaInput.SetValue(item.MetaInfo)
		m.screen = ScreenAddCard
		m.activeField = 0
		m.blurAllExcept(&m.cardNumber)

	case models.BinaryItem:
		payload = &models.BinaryPayload{}
		if err := m.keeper.DecryptAndParse(item, payload); err == nil {
			p := payload.(*models.BinaryPayload)
			m.filePathInput.SetValue(p.FilePath)
		}
		m.metaInput.SetValue(item.MetaInfo)
		m.screen = ScreenAddBinary
		m.activeField = 0
		m.blurAllExcept(&m.filePathInput)
	}
	return m, textinput.Blink
}

// resetForms очищает значения всех полей ввода
func (m *modelTUI) resetForms() {
	for _, in := range []*textinput.Model{
		&m.loginInput, &m.passwordInput, &m.metaInput,
		&m.contentInput, &m.cardNumber, &m.cardHolder,
		&m.cardExpiry, &m.cardCVV, &m.filePathInput,
	} {
		in.SetValue("")
	}
}

type formField struct {
	label string
	input textinput.Model
}

// View генерирует строковое представление текущего состояния модели для отображения в терминале.
func (m *modelTUI) View() string {
	if m.quitting {
		return "До свидания!\n"
	}

	var content string
	switch m.screen {
	case ScreenAuth:
		content = m.viewMenu("GophKeeper",
			"Добро пожаловать в менеджер приватных данных.",
			"↑/↓ навигация • enter выбор • q выход")
	case ScreenLoginForm:
		content = m.viewForm("Вход в систему",
			[]formField{
				{"Имя пользователя:", m.usernameInput},
				{"Пароль:", m.passwordInput},
			})
	case ScreenRegisterForm:
		content = m.viewForm("Регистрация нового аккаунта",
			[]formField{
				{"Имя пользователя:", m.usernameInput},
				{"Пароль:", m.passwordInput},
			})
	case ScreenMainMenu:
		content = m.viewMenu("Главное меню", "", "↑/↓ навигация • enter выбор • q выход")
	case ScreenListItems:
		content = m.viewListItems("Мои данные", "", "")
	case ScreenAddLogin:
		title := "Добавить логин/пароль"
		if m.isEditing {
			title = "Редактировать логин/пароль"
		}
		content = m.viewForm(title,
			[]formField{
				{"Логин для сервиса:", m.loginInput},
				{"Пароль:", m.passwordInput},
				{"Метаинформация:", m.metaInput},
			})
	case ScreenAddText:
		title := "Добавить текст"
		if m.isEditing {
			title = "Редактировать текст"
		}
		content = m.viewForm(title, []formField{{"Содержимое:", m.contentInput}, {"Метаинформация:", m.metaInput}})
	case ScreenAddCard:
		title := "Добавить банковскую карту"
		if m.isEditing {
			title = "Редактировать банковскую карту"
		}
		content = m.viewForm(title,
			[]formField{
				{"Номер карты:", m.cardNumber},
				{"Владелец:", m.cardHolder},
				{"Срок действия (MM/YY):", m.cardExpiry},
				{"CVV:", m.cardCVV},
				{"Метаинформация:", m.metaInput},
			})
	case ScreenAddBinary:
		title := "Добавить бинарные данные"
		if m.isEditing {
			title = "Редактировать бинарные данные"
		}
		content = m.viewForm(title,
			[]formField{
				{"Путь к файлу:", m.filePathInput},
				{"Метаинформация:", m.metaInput},
			})
	}
	return contentStyle.Render(content)
}

// viewMenu рендерит экраны с меню (аутентификация и главное меню)
func (m *modelTUI) viewMenu(title, intro, help string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if intro != "" {
		b.WriteString(intro + "\n\n")
	}
	for i, item := range m.menuItems {
		cursor := " "
		if m.menuCursor == i {
			cursor = ">"
		}
		style := menuItemStyle
		if m.menuCursor == i {
			style = selectedItemStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, item.title)) + "\n")
		b.WriteString(menuItemStyle.Render(fmt.Sprintf("   %s", item.desc)) + "\n\n")
	}
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render(m.errMsg) + "\n")
	}
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n")
	}
	if m.loading {
		b.WriteString(statusStyle.Render("Загрузка...") + "\n")
	}
	b.WriteString(helpStyle.Render(help))
	return b.String()
}

// viewForm рендерит экраны с формами ввода данных, отображая заголовок,
// поля ввода с соответствующими стилями (активный/неактивный), сообщения
// об ошибках и подсказки по навигации.
func (m *modelTUI) viewForm(title string, fields []formField) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	for i := range fields {
		b.WriteString(fields[i].label + "\n")
		if m.activeField == i {
			b.WriteString(activeInputStyle.Render(fields[i].input.View()) + "\n")
		} else {
			b.WriteString(inputStyle.Render(fields[i].input.View()) + "\n")
		}
	}
	if m.errMsg != "" {
		b.WriteString("\n" + errorStyle.Render(m.errMsg) + "\n")
	}
	if m.loading {
		b.WriteString(statusStyle.Render("Сохранение...") + "\n")
	}
	b.WriteString(helpStyle.Render("tab переключение • enter далее/подтвердить • esc отмена"))
	return b.String()
}

// viewListItems рендерит экран списка сохраненных элементов.
func (m *modelTUI) viewListItems(title, intro, _ string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if intro != "" {
		b.WriteString(intro + "\n\n")
	}

	if len(m.items) == 0 {
		b.WriteString("Список пуст. Нажмите 's' для синхронизации с сервером.\n")
	} else {
		for i, item := range m.items {
			isSelected := i == m.listCursor
			prefix := "  "
			if isSelected {
				prefix = "> "
			}

			idStyle := lipgloss.NewStyle().Bold(true)
			if isSelected {
				idStyle = idStyle.Foreground(lipgloss.Color("#7D56F4"))
			}

			b.WriteString(idStyle.Render(fmt.Sprintf("%sID: %s", prefix, item.ID)) + "\n")

			infoStyle := lipgloss.NewStyle().PaddingLeft(2)
			if isSelected {
				infoStyle = infoStyle.Foreground(lipgloss.Color("#7D56F4"))
			}

			fmt.Fprintf(&b, infoStyle.Render("Тип: %s | Мета: %s | Создано: %s\n"),
				item.DataType, item.MetaInfo, item.CreatedAt.Format("2006-01-02 15:04"))

			if isSelected {
				var payload any
				switch item.DataType {
				case models.LoginItem:
					payload = &models.LoginPayload{}
				case models.TextItem:
					payload = &models.TextPayload{}
				case models.CardItem:
					payload = &models.CardPayload{}
				case models.BinaryItem:
					payload = &models.BinaryPayload{}
				}

				if err := m.keeper.DecryptAndParse(item, payload); err != nil {
					b.WriteString(errorStyle.Render(fmt.Sprintf("  Ошибка расшифровки: %v\n", err)))
				} else {
					switch p := payload.(type) {
					case *models.LoginPayload:
						fmt.Fprintf(&b, "  Логин: %s\n  Пароль: %s\n", p.Username, p.Password)
					case *models.TextPayload:
						fmt.Fprintf(&b, "  Содержание: %s\n", p.Content)
					case *models.CardPayload:
						fmt.Fprintf(&b, "  Карта: %s\n  Владелец: %s\n  Срок: %s/%s, CVV: %s\n",
							p.Number, p.Holder, p.ExpiryMonth, p.ExpiryYear, p.CVV)
					case *models.BinaryPayload:
						fmt.Fprintf(&b, "  Файл: %s\n  Размер: %d байт\n  MIME: %s\n",
							p.FilePath, p.Size, p.MimeType)
					}
				}
			}
			b.WriteString(strings.Repeat("─", 50) + "\n")
		}
	}

	if m.awaitingDeleteConfirm && len(m.items) > 0 {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("Вы уверены, что хотите удалить запись %s? (y/n)", m.items[m.listCursor].ID)) + "\n")
	}

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render(m.errMsg) + "\n")
	}
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n")
	}
	if m.loading {
		b.WriteString(statusStyle.Render("Загрузка...") + "\n")
	}

	helpText := "↑/↓ выбор • e редактировать • d удалить • s синхронизировать • esc назад"
	if m.awaitingDeleteConfirm {
		helpText = "y подтвердить удаление • n отмена"
	}
	b.WriteString(helpStyle.Render(helpText))
	return b.String()
}
