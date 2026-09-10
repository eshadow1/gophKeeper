// Package models описывает модели данных, используемые в менеджере паролей GophKeeper.
// Пакет содержит определения типов хранимой информации: логины, тексты, бинарные данные,
// банковские карты, а также структуру элемента с метаинформацией.
package model

import (
	"fmt"
	"time"
)

// ItemType представляет тип хранимой информации.
// Поддерживаемые значения: Login, Text, Binary, Card.
type ItemType string

const (
	// LoginItem тип для хранения пары логин/пароль.
	LoginItem ItemType = "login"
	// TextItem тип для хранения произвольного текста.
	TextItem ItemType = "text"
	// BinaryItem тип для хранения бинарных данных.
	BinaryItem ItemType = "binary"
	// CardItem тип для хранения данных банковской карты.
	CardItem ItemType = "card"
)

// LoginPayload описывает полезную нагрузку для элемента типа Login.
// Содержит пару логин/пароль для доступа к сервису.
type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TextPayload описывает полезную нагрузку для элемента типа Text.
// Содержит произвольный текст.
type TextPayload struct {
	Content string `json:"content"`
}

// BinaryPayload описывает полезную нагрузку для элемента типа Binary.
// Содержит путь к файлу и его MIME-тип.
type BinaryPayload struct {
	FilePath string `json:"file_path"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Data     []byte `json:"data"`
}

// CardPayload описывает полезную нагрузку для элемента типа Card.
// Содержит данные банковской карты.
type CardPayload struct {
	Number      string `json:"number"`
	Holder      string `json:"holder"`
	ExpiryMonth string `json:"expiry_month"`
	ExpiryYear  string `json:"expiry_year"`
	CVV         string `json:"cvv"`
}

// Metadata представляет произвольную метаинформацию об элементе.
// Может содержать принадлежность к веб-сайту, личности, банку,
// списки одноразовых кодов и прочую текстовую информацию.
type Metadata map[string]string

// Item представляет собой единицу хранимой информации.
// Содержит тип, имя, полезную нагрузку (в виде JSON) и метаинформацию.
type Item struct {
	// ID уникальный идентификатор элемента на сервере.
	ID string
	// DataType тип хранимых данных (login, text, binary, card).
	DataType ItemType
	// EncryptedData зашифрованное содержимое элемента.
	EncryptedData []byte
	// MetaInfo произвольная текстовая метаинформация (открытый текст).
	MetaInfo string
	// CreatedAt время создания элемента.
	CreatedAt time.Time
	// UpdateAt время обновления элемента
	UpdatedAt time.Time
}

// Validate проверяет корректность типа элемента.
// Возвращает ошибку, если тип не входит в список допустимых.
func (t ItemType) Validate() error {
	switch t {
	case LoginItem, TextItem, BinaryItem, CardItem:
		return nil
	default:
		return fmt.Errorf("unknown item type: %s", t)
	}
}

// String возвращает строковое представление типа элемента.
func (t ItemType) String() string {
	return string(t)
}
