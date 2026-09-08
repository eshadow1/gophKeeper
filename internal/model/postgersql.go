package model

import "time"

// UserUnsave представляет собой полезную нагрузку для регистрации или входа пользователя.
type UserUnsave struct {
	// Username — уникальный идентификатор, выбранный пользователем для аутентификации.
	Username string `json:"username"`

	// Password — пароль в открытом виде, предоставленный пользователем.
	Password string `json:"password"`
}

// User представляет собой сущность зарегистрированного пользователя в системе.
type User struct {
	// ID — уникальный идентификатор пользователя в системе (обычно UUID).
	ID string `json:"id"`

	// Username — уникальное имя для входа, связанное с учетной записью пользователя.
	Username string `json:"username"`

	// PasswordHash — безопасно захешированная версия пароля пользователя (например, с использованием bcrypt).
	PasswordHash string `json:"password_hash"`

	// CreatedAt — точная временная метка (UTC) создания учетной записи пользователя.
	CreatedAt time.Time `json:"created_at"`
}

// ItemDB представляет собой универсальный элемент зашифрованных данных,
// хранящийся в базе данных. Структура предназначена для хранения конфиденциальных
// данных пользователя в зашифрованном виде, а также метаданных, описывающих
// тип данных и принадлежность.
type ItemDB struct {
	// ID — уникальный идентификатор элемента данных в системе (обычно UUID).
	ID string `json:"id"`

	// UserID — внешний ключ, ссылающийся на пользователя (User), которому принадлежит данный элемент.
	UserID string `json:"user_id"`

	// DataType — категория зашифрованных данных ('login', 'text', 'binary', 'card').
	DataType string `json:"data_type"`

	// EncryptedData — фактическая конфиденциальная полезная нагрузка в зашифрованном виде.
	EncryptedData []byte `json:"encrypted_data"`

	// MetaInfo — дополнительные метаданные или теги, связанные с элементом
	// (например, заголовки, URL-адреса или описания), которые могут потребоваться
	// для поиска, сортировки или отображения в UI и не требуют строгого шифрования.
	MetaInfo string `json:"meta_info"`

	// CreatedAt — точная временная метка (UTC) создания элемента данных.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt — точная временная метка (UTC) обноваления элемента данных.
	UpdatedAt time.Time `json:"updated_at"`
}
