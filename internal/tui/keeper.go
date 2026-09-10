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
	del  = "delete"
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
)

const (
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

// formFieldDef описывает конфигурацию одного поля ввода
type formFieldDef struct {
	id          string
	label       string
	placeholder string
	charLimit   int
	width       int
	echoPwd     bool
}

// formConfig описывает поведение целой формы
type formConfig struct {
	title    string
	fields   []formFieldDef
	validate func(m *modelTUI) error
	submit   func(m *modelTUI) tea.Cmd
}

type modelTUI struct {
	keeper     KeeperClient
	doing      DoingModel
	screen     Screen
	menuCursor int
	menuItems  []menuOption

	inputs         map[string]*textinput.Model
	currentForm    []string
	activeFieldIdx int

	items             []*models.Item
	decrypted         map[string]any
	statusMsg, errMsg string
	loading, quitting bool
	width, height     int

	listCursor            int
	isEditing             bool
	editingItemID         string
	awaitingDeleteConfirm bool

	searchInput    textinput.Model
	isSearching    bool
	isActiveSearch bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
	menuItemStyle     = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8FF60")).
			Italic(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			Bold(true)
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)
	activeInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFD700")).
				Padding(0, 1).
				MarginBottom(1)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)
	contentStyle = lipgloss.NewStyle().Padding(1, 2)
)

func newInput(placeholder string, charLimit, width int, echoPwd bool) *textinput.Model {
	i := textinput.New()
	i.Placeholder = placeholder
	i.CharLimit = charLimit
	i.Width = width
	if echoPwd {
		i.EchoMode = textinput.EchoPassword
	}
	return &i
}

// NewModel создает и инициализирует новый экземпляр основной модели TUI.
func NewModel(keeper KeeperClient, doing DoingModel) *modelTUI {
	m := &modelTUI{
		keeper:    keeper,
		doing:     doing,
		screen:    ScreenAuth,
		menuItems: defaultMenu(),
		inputs:    make(map[string]*textinput.Model),
		decrypted: make(map[string]any),
	}

	m.inputs["username"] = newInput("Имя пользователя", 64, 40, false)
	m.inputs["password"] = newInput("Пароль", 64, 40, true)
	m.inputs["login"] = newInput("Логин для сервиса", 128, 40, false)
	m.inputs["cardNumber"] = newInput("Номер карты (1234 5678 9012 3456)", 19, 40, false)
	m.inputs["cardHolder"] = newInput("Владелец карты", 64, 40, false)
	m.inputs["cardExpiry"] = newInput("Срок действия (MM/YY)", 5, 20, false)
	m.inputs["cardCVV"] = newInput("CVV", 4, 10, true)
	m.inputs["content"] = newInput("Текстовое содержимое", 1024, 60, false)
	m.inputs["meta"] = newInput("Метаинформация (необязательно)", 256, 60, false)
	m.inputs["filePath"] = newInput("Путь к файлу", 512, 60, false)

	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "Поиск по метаинформации..."
	m.searchInput.CharLimit = 128
	m.searchInput.Width = 50

	return m
}

// Init возвращает начальную команду для запуска анимации мигания курсора
// в первом активном поле ввода при старте приложения.
func (*modelTUI) Init() tea.Cmd { return textinput.Blink }

func (m *modelTUI) blurAll() {
	for k := range m.inputs {
		m.inputs[k].Blur()
	}
	m.searchInput.Blur()
}

func (m *modelTUI) focusField(id string) {
	m.blurAll()
	if inp, ok := m.inputs[id]; ok {
		inp.Focus()
		m.inputs[id] = inp
	}
}

func (m *modelTUI) resetForms() {
	for k := range m.inputs {
		m.inputs[k].SetValue("")
	}
}

func (m *modelTUI) setupForm(config formConfig) {
	m.currentForm = make([]string, len(config.fields))
	for i, field := range config.fields {
		m.currentForm[i] = field.id
	}
	m.activeFieldIdx = 0
	if len(m.currentForm) > 0 {
		m.focusField(m.currentForm[0])
	}
}

func (m *modelTUI) getFormConfig() formConfig {
	baseValidate := func(requiredFields ...string) func(m *modelTUI) error {
		return func(m *modelTUI) error {
			for _, field := range requiredFields {
				if m.inputs[field].Value() == "" {
					return fmt.Errorf("поле обязательно для заполнения")
				}
			}
			return nil
		}
	}

	switch m.screen {
	case ScreenLoginForm, ScreenRegisterForm:
		return formConfig{
			title:    "Вход в систему",
			fields:   []formFieldDef{{id: "username", label: "Имя пользователя:"}, {id: "password", label: "Пароль:"}},
			validate: baseValidate("username", "password"),
			submit: func(m *modelTUI) tea.Cmd {
				m.loading, m.errMsg = true, ""
				u, p := m.inputs["username"].Value(), m.inputs["password"].Value()
				if m.screen == ScreenLoginForm {
					return m.doing.Login(u, p)
				}
				return m.doing.Register(u, p)
			},
		}
	case ScreenAddLogin:
		title := "Добавить логин/пароль"
		if m.isEditing {
			title = "Редактировать логин/пароль"
		}
		return formConfig{
			title:    title,
			fields:   []formFieldDef{{id: "login", label: "Логин для сервиса:"}, {id: "password", label: "Пароль:"}, {id: "meta", label: "Метаинформация:"}},
			validate: baseValidate("login", "password"),
			submit: func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.LoginPayload{Username: m.inputs["login"].Value(), Password: m.inputs["password"].Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.LoginItem, payload, m.inputs["meta"].Value())
				}
				return m.doing.AddItem(models.LoginItem, payload, m.inputs["meta"].Value())
			},
		}
	case ScreenAddText:
		title := "Добавить текст"
		if m.isEditing {
			title = "Редактировать текст"
		}
		return formConfig{
			title:    title,
			fields:   []formFieldDef{{id: "content", label: "Содержимое:"}, {id: "meta", label: "Метаинформация:"}},
			validate: baseValidate("content"),
			submit: func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.TextPayload{Content: m.inputs["content"].Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.TextItem, payload, m.inputs["meta"].Value())
				}
				return m.doing.AddItem(models.TextItem, payload, m.inputs["meta"].Value())
			},
		}
	case ScreenAddCard:
		title := "Добавить банковскую карту"
		if m.isEditing {
			title = "Редактировать банковскую карту"
		}
		return formConfig{
			title: title,
			fields: []formFieldDef{
				{id: "cardNumber", label: "Номер карты:"}, {id: "cardHolder", label: "Владелец:"},
				{id: "cardExpiry", label: "Срок действия (MM/YY):"}, {id: "cardCVV", label: "CVV:"}, {id: "meta", label: "Метаинформация:"},
			},
			validate: baseValidate("cardNumber", "cardHolder", "cardExpiry", "cardCVV"),
			submit: func(m *modelTUI) tea.Cmd {
				m.loading = true
				parts := strings.Split(m.inputs["cardExpiry"].Value(), "/")
				expM, expY := "", ""
				if len(parts) == 2 {
					expM, expY = parts[0], parts[1]
				}
				payload := models.CardPayload{
					Number: m.inputs["cardNumber"].Value(), Holder: m.inputs["cardHolder"].Value(),
					ExpiryMonth: expM, ExpiryYear: expY, CVV: m.inputs["cardCVV"].Value(),
				}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.CardItem, payload, m.inputs["meta"].Value())
				}
				return m.doing.AddItem(models.CardItem, payload, m.inputs["meta"].Value())
			},
		}
	case ScreenAddBinary:
		title := "Добавить бинарные данные"
		if m.isEditing {
			title = "Редактировать бинарные данные"
		}
		return formConfig{
			title:    title,
			fields:   []formFieldDef{{id: "filePath", label: "Путь к файлу:"}, {id: "meta", label: "Метаинформация:"}},
			validate: baseValidate("filePath"),
			submit: func(m *modelTUI) tea.Cmd {
				m.loading = true
				payload := models.BinaryPayload{FilePath: m.inputs["filePath"].Value()}
				if m.isEditing {
					return m.doing.UpdateItem(m.editingItemID, models.BinaryItem, payload, m.inputs["meta"].Value())
				}
				return m.doing.AddItem(models.BinaryItem, payload, m.inputs["meta"].Value())
			},
		}
	}
	return formConfig{}
}

// Update является основным методом обработки сообщений в архитектуре Bubble Tea.
func (m *modelTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen == ScreenAuth || (m.screen == ScreenMainMenu && m.menuCursor == 5) {
				m.quitting = true
				return m, tea.Quit
			}
			if m.screen != ScreenAuth && m.screen != ScreenMainMenu {
				return m.handleEscape()
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
			m.adjustListCursor()
		}
		return m, nil
	case addItemDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Ошибка: %v", msg.err)
		} else {
			if m.isEditing {
				m.statusMsg = "Данные успешно обновлены"
				m.isEditing, m.editingItemID = false, ""
			} else {
				m.statusMsg = "Данные успешно добавлены и зашифрованы"
			}
			m.resetForms()
			m.screen = ScreenListItems
			m.items = m.keeper.ListItems()
			m.adjustListCursor()
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
	case ScreenLoginForm, ScreenRegisterForm, ScreenAddLogin, ScreenAddText, ScreenAddCard, ScreenAddBinary:
		return m.updateActiveForm(msg)
	case ScreenListItems:
		return m.updateListItems(msg)
	}
	return m, nil
}

func (m *modelTUI) updateActiveForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	config := m.getFormConfig()
	if key, ok := msg.(tea.KeyMsg); ok {
		if len(m.currentForm) == 0 {
			return m, nil
		}
		activeFieldID := m.currentForm[m.activeFieldIdx]
		activeInput := m.inputs[activeFieldID]

		switch key.String() {
		case "tab", down:
			activeInput.Blur()
			m.inputs[activeFieldID] = activeInput
			m.activeFieldIdx = (m.activeFieldIdx + 1) % len(m.currentForm)
			m.focusField(m.currentForm[m.activeFieldIdx])
			return m, textinput.Blink
		case "shift+tab", up:
			activeInput.Blur()
			m.inputs[activeFieldID] = activeInput
			m.activeFieldIdx = (m.activeFieldIdx - 1 + len(m.currentForm)) % len(m.currentForm)
			m.focusField(m.currentForm[m.activeFieldIdx])
			return m, textinput.Blink
		case "enter":
			if m.activeFieldIdx < len(m.currentForm)-1 {
				activeInput.Blur()
				m.inputs[activeFieldID] = activeInput
				m.activeFieldIdx++
				m.focusField(m.currentForm[m.activeFieldIdx])
				return m, textinput.Blink
			}
			if config.validate != nil {
				if err := config.validate(m); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
			}
			return m, config.submit(m)
		}

		updated, cmd := activeInput.Update(msg)
		m.inputs[activeFieldID] = &updated
		return m, cmd
	}
	return m, nil
}

func (m *modelTUI) handleEscape() (tea.Model, tea.Cmd) {
	if m.isEditing {
		m.isEditing, m.editingItemID = false, ""
		m.resetForms()
		m.screen = ScreenListItems
		m.errMsg, m.statusMsg = "", "Редактирование отменено"
		return m, nil
	}

	if m.isSearching {
		m.isSearching = false
		m.isActiveSearch = false
		m.searchInput.SetValue("")
		m.searchInput.Blur()
		m.adjustListCursor()
		return m, nil
	}

	m.errMsg, m.statusMsg, m.awaitingDeleteConfirm = "", "", false

	if m.screen >= ScreenListItems {
		m.screen, m.menuItems, m.menuCursor = ScreenMainMenu, mainMenu(), 0
	} else {
		m.screen, m.menuItems, m.menuCursor = ScreenAuth, defaultMenu(), 0
	}
	m.blurAll()
	return m, nil
}

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
					m.screen = ScreenLoginForm
					m.setupForm(m.getFormConfig())
					return m, textinput.Blink
				case tabRegister:
					m.screen = ScreenRegisterForm
					m.setupForm(m.getFormConfig())
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
				case tabAddLogin, tabAddText, tabAddCard, tabAddBinary:
					m.isEditing = false
					m.screen = Screen(m.menuCursor + 4)
					m.setupForm(m.getFormConfig())
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

func (m *modelTUI) getFilteredItems() []*models.Item {
	if !m.isSearching || m.searchInput.Value() == "" {
		return m.items
	}
	query := strings.ToLower(m.searchInput.Value())
	var filtered []*models.Item
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.MetaInfo), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *modelTUI) adjustListCursor() {
	filtered := m.getFilteredItems()
	if len(filtered) == 0 {
		m.listCursor = 0
	} else if m.listCursor >= len(filtered) {
		m.listCursor = len(filtered) - 1
	}
}

func (m *modelTUI) updateListItems(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.isSearching && m.isActiveSearch {
			switch key.String() {
			case "enter":
				m.isActiveSearch = false
				m.searchInput.Blur()
				m.adjustListCursor()
				return m, nil
			case up:
				filtered := m.getFilteredItems()
				if m.listCursor > 0 {
					m.listCursor--
				} else if len(filtered) > 0 {
					m.listCursor = len(filtered) - 1
				}
				return m, nil
			case down:
				filtered := m.getFilteredItems()
				if m.listCursor < len(filtered)-1 {
					m.listCursor++
				} else if len(filtered) > 0 {
					m.listCursor = 0
				}
				return m, nil
			}

			updatedInput, cmd := m.searchInput.Update(msg)
			m.searchInput = updatedInput
			m.adjustListCursor()
			return m, cmd
		}

		switch key.String() {
		case "/":
			m.isSearching = true
			m.isActiveSearch = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case up, "k":
			filtered := m.getFilteredItems()
			if m.listCursor > 0 {
				m.listCursor--
			} else if len(filtered) > 0 {
				m.listCursor = len(filtered) - 1
			}
		case down, "j":
			filtered := m.getFilteredItems()
			if m.listCursor < len(filtered)-1 {
				m.listCursor++
			} else if len(filtered) > 0 {
				m.listCursor = 0
			}
		case del, "d":
			filtered := m.getFilteredItems()
			if len(filtered) > 0 {
				m.awaitingDeleteConfirm = true
			}
		case "e":
			filtered := m.getFilteredItems()
			if len(filtered) > 0 {
				return m.startEdit(filtered[m.listCursor])
			}
		case "s":
			m.loading = true
			return m, m.doing.Sync()
		case "y":
			if m.awaitingDeleteConfirm {
				filtered := m.getFilteredItems()
				if len(filtered) > 0 {
					m.loading = true
					m.awaitingDeleteConfirm = false
					return m, m.doing.DeleteItem(filtered[m.listCursor].ID)
				}
			}
		case "n":
			if m.awaitingDeleteConfirm {
				m.awaitingDeleteConfirm = false
			}
		}
	}
	return m, nil
}

func (m *modelTUI) startEdit(item *models.Item) (tea.Model, tea.Cmd) {
	m.isEditing = true
	m.editingItemID = item.ID
	m.errMsg, m.awaitingDeleteConfirm = "", false
	m.resetForms()

	switch item.DataType {
	case models.LoginItem:
		m.screen = ScreenAddLogin
		m.setupForm(m.getFormConfig())
		m.fillLoginPayload(item)
	case models.TextItem:
		m.screen = ScreenAddText
		m.setupForm(m.getFormConfig())
		m.fillTextPayload(item)
	case models.CardItem:
		m.screen = ScreenAddCard
		m.setupForm(m.getFormConfig())
		m.fillCardPayload(item)
	case models.BinaryItem:
		m.screen = ScreenAddBinary
		m.setupForm(m.getFormConfig())
		m.fillBinaryPayload(item)
	default:
		m.errMsg = "Неизвестный тип данных для редактирования"
		return m, nil
	}

	return m, textinput.Blink
}

func (m *modelTUI) fillLoginPayload(item *models.Item) {
	payload := &models.LoginPayload{}
	if err := m.keeper.DecryptAndParse(item, payload); err == nil {
		m.inputs["login"].SetValue(payload.Username)
		m.inputs["password"].SetValue(payload.Password)
	}
	m.inputs["meta"].SetValue(item.MetaInfo)
}

func (m *modelTUI) fillTextPayload(item *models.Item) {
	payload := &models.TextPayload{}
	if err := m.keeper.DecryptAndParse(item, payload); err == nil {
		m.inputs["content"].SetValue(payload.Content)
	}
	m.inputs["meta"].SetValue(item.MetaInfo)
}

func (m *modelTUI) fillCardPayload(item *models.Item) {
	payload := &models.CardPayload{}
	if err := m.keeper.DecryptAndParse(item, payload); err == nil {
		m.inputs["cardNumber"].SetValue(payload.Number)
		m.inputs["cardHolder"].SetValue(payload.Holder)
		m.inputs["cardExpiry"].SetValue(fmt.Sprintf("%s/%s", payload.ExpiryMonth, payload.ExpiryYear))
		m.inputs["cardCVV"].SetValue(payload.CVV)
	}
	m.inputs["meta"].SetValue(item.MetaInfo)
}

func (m *modelTUI) fillBinaryPayload(item *models.Item) {
	payload := &models.BinaryPayload{}
	if err := m.keeper.DecryptAndParse(item, payload); err == nil {
		m.inputs["filePath"].SetValue(payload.FilePath)
	}
	m.inputs["meta"].SetValue(item.MetaInfo)
}

// View генерирует строковое представление текущего состояния модели для отображения в терминале.
func (m *modelTUI) View() string {
	if m.quitting {
		return "До свидания!\n"
	}

	var content string
	switch m.screen {
	case ScreenAuth:
		content = m.viewMenu("GophKeeper", "Добро пожаловать в менеджер приватных данных.", "↑/↓ навигация • enter выбор • q выход")
	case ScreenLoginForm, ScreenRegisterForm, ScreenAddLogin, ScreenAddText, ScreenAddCard, ScreenAddBinary:
		config := m.getFormConfig()
		content = m.viewForm(config)
	case ScreenMainMenu:
		content = m.viewMenu("Главное меню", "", "↑/↓ навигация • enter выбор • q выход")
	case ScreenListItems:
		content = m.viewListItems("Мои данные", "", "")
	}
	return contentStyle.Render(content)
}

func (m *modelTUI) viewMenu(title, intro, help string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if intro != "" {
		b.WriteString(intro + "\n\n")
	}
	for i, item := range m.menuItems {
		cursor := " "
		style := menuItemStyle
		if m.menuCursor == i {
			cursor = ">"
			style = selectedItemStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, item.title)) + "\n")
		b.WriteString(menuItemStyle.Render(fmt.Sprintf("   %s", item.desc)) + "\n\n")
	}
	m.writeStatusMessages(&b)
	b.WriteString(helpStyle.Render(help))
	return b.String()
}

func (m *modelTUI) viewForm(config formConfig) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(config.title) + "\n\n")

	for i, fieldDef := range config.fields {
		b.WriteString(fieldDef.label + "\n")
		inp := m.inputs[fieldDef.id]
		if m.activeFieldIdx == i {
			b.WriteString(activeInputStyle.Render(inp.View()) + "\n")
		} else {
			b.WriteString(inputStyle.Render(inp.View()) + "\n")
		}
	}

	m.writeStatusMessages(&b)
	b.WriteString(helpStyle.Render("tab переключение • enter далее/подтвердить • esc отмена"))
	return b.String()
}

func (m *modelTUI) viewListItems(title, intro, _ string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if intro != "" {
		b.WriteString(intro + "\n\n")
	}

	if m.isSearching {
		b.WriteString("Поиск: " + activeInputStyle.Render(m.searchInput.View()) + "\n\n")
	} else {
		b.WriteString("Нажмите '/' для поиска по метаинформации\n\n")
	}

	filtered := m.getFilteredItems()

	if len(filtered) == 0 {
		if len(m.items) == 0 {
			b.WriteString("Список пуст.\n")
		} else {
			b.WriteString("Ничего не найдено по вашему запросу.\n")
		}
	} else {
		for i, item := range filtered {
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
			fmt.Fprintf(&b, infoStyle.Render("Тип: %s | Мета: %s | Создано: %s | Обновлено: %s\n"),
				item.DataType, item.MetaInfo, item.CreatedAt.Format("2006-01-02 15:04"), item.UpdatedAt.Format("2006-01-02 15:04"))

			if isSelected {
				m.renderDecryptedItem(&b, item)
			}
			b.WriteString(strings.Repeat("─", 50) + "\n")
		}
	}

	if m.awaitingDeleteConfirm && len(filtered) > 0 {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("Вы уверены, что хотите удалить запись %s? (y/n)", filtered[m.listCursor].ID)) + "\n")
	}

	m.writeStatusMessages(&b)

	helpText := "↑/↓ выбор • e редактировать • d удалить • s синхронизировать • / поиск • esc назад"
	if m.isSearching {
		helpText = "введите запрос • enter применить • esc отменить поиск"
	} else if m.awaitingDeleteConfirm {
		helpText = "y подтвердить удаление • n отмена"
	}
	b.WriteString(helpStyle.Render(helpText))
	return b.String()
}

func (m *modelTUI) renderDecryptedItem(b *strings.Builder, item *models.Item) {
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
	default:
		return
	}

	if err := m.keeper.DecryptAndParse(item, payload); err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Ошибка расшифровки: %v\n", err)))
		return
	}

	switch p := payload.(type) {
	case *models.LoginPayload:
		fmt.Fprintf(b, "\n\t\tЛогин: %s\n\t\tПароль: %s\n\n", p.Username, p.Password)
	case *models.TextPayload:
		fmt.Fprintf(b, "\n\t\tСодержание: %s\n\n", p.Content)
	case *models.CardPayload:
		fmt.Fprintf(b, "\n\t\tКарта: %s\n\t\tВладелец: %s\n\t\tСрок: %s/%s, CVV: %s\n\n",
			p.Number, p.Holder, p.ExpiryMonth, p.ExpiryYear, p.CVV)
	case *models.BinaryPayload:
		fmt.Fprintf(b, "\n\t\tФайл: %s\n\t\tРазмер: %d байт\n\t\tMIME: %s\n\n",
			p.FilePath, p.Size, p.MimeType)
	}
}

func (m *modelTUI) writeStatusMessages(b *strings.Builder) {
	if m.errMsg != "" {
		b.WriteString("\n" + errorStyle.Render(m.errMsg) + "\n")
	}
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n")
	}
	if m.loading {
		b.WriteString(statusStyle.Render("Загрузка...") + "\n")
	}
}
