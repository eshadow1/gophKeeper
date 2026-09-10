package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_Validate_SimpleItems(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		dataType    model.ItemType
		payload     any
		wantErr     bool
		errContains string
	}{
		{
			name:     "LoginItem: успешная валидация",
			dataType: model.LoginItem,
			payload: model.LoginPayload{
				Username: "testuser",
				Password: "securepassword123",
			},
			wantErr: false,
		},
		{
			name:     "LoginItem: ошибка, пароль слишком короткий",
			dataType: model.LoginItem,
			payload: model.LoginPayload{
				Username: "testuser",
				Password: "1234567",
			},
			wantErr:     true,
			errContains: "password is too short",
		},
		{
			name:     "LoginItem: ошибка, пустой username",
			dataType: model.LoginItem,
			payload: model.LoginPayload{
				Username: "",
				Password: "securepassword123",
			},
			wantErr:     true,
			errContains: "username is empty",
		},
		{
			name:     "TextItem: успешная валидация",
			dataType: model.TextItem,
			payload: model.TextPayload{
				Content: "Some important text",
			},
			wantErr: false,
		},
		{
			name:     "TextItem: ошибка, пустой контент",
			dataType: model.TextItem,
			payload: model.TextPayload{
				Content: "",
			},
			wantErr:     true,
			errContains: "content is empty",
		},
		{
			name:        "Неизвестный тип элемента",
			dataType:    model.ItemType("unknown_type"),
			payload:     model.TextPayload{Content: "test"},
			wantErr:     true,
			errContains: "invalid item type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(t.Context(), tt.dataType, tt.payload)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidator_Validate_BinaryItem(t *testing.T) {
	v := NewValidator()

	tempDir := t.TempDir()
	validFilePath := filepath.Join(tempDir, "valid_file.txt")
	errWrite := os.WriteFile(validFilePath, []byte("test data"), 0644)
	require.NoError(t, errWrite)

	tests := []struct {
		name        string
		payload     any
		setup       func()
		cleanup     func()
		wantErr     bool
		errContains string
	}{
		{
			name: "BinaryItem: успешная валидация существующего файла",
			payload: model.BinaryPayload{
				FilePath: validFilePath,
			},
			wantErr: false,
		},
		{
			name: "BinaryItem: ошибка, пустой путь к файлу",
			payload: model.BinaryPayload{
				FilePath: "",
			},
			wantErr:     true,
			errContains: "file path is empty",
		},
		{
			name: "BinaryItem: ошибка, файл не существует",
			payload: model.BinaryPayload{
				FilePath: filepath.Join(tempDir, "non_existent_file.txt"),
			},
			wantErr:     true,
			errContains: "file does not exist",
		},
		{
			name: "BinaryItem: ошибка, путь указывает на директорию, а не на файл",
			payload: model.BinaryPayload{
				FilePath: tempDir,
			},
			wantErr:     true,
			errContains: "file is not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			t.Cleanup(func() {
				if tt.cleanup != nil {
					tt.cleanup()
				}
			})

			errValid := v.Validate(t.Context(), model.BinaryItem, tt.payload)

			if tt.wantErr {
				require.Error(t, errValid)
				assert.Contains(t, errValid.Error(), tt.errContains)
			} else {
				require.NoError(t, errValid)
			}
		})
	}
}

func TestValidator_Validate_CardItem(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		payload     any
		wantErr     bool
		errContains string
	}{
		{
			name: "CardItem: успешная валидация",
			payload: model.CardPayload{
				Number:      "4111 1111-1111 1111",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr: false,
		},
		{
			name: "CardItem: ошибка, неверный номер карты (слишком короткий)",
			payload: model.CardPayload{
				Number:      "12345",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "номер карты: invalid card number",
		},
		{
			name: "CardItem: ошибка, неверный номер карты (буквы)",
			payload: model.CardPayload{
				Number:      "4111 1111 1111 ABCD",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "номер карты: invalid card number",
		},
		{
			name: "CardItem: ошибка, неверный держатель (слишком короткий)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "имя держателя: invalid holder",
		},
		{
			name: "CardItem: ошибка, неверный держатель (цифры в имени)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IVAN 123",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "имя держателя: invalid holder",
		},
		{
			name: "CardItem: ошибка, неверный месяц (не число)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "AB",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "месяц: invalid month",
		},
		{
			name: "CardItem: ошибка, неверный месяц (вне диапазона 1-12)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "13",
				ExpiryYear:  "2025",
				CVV:         "123",
			},
			wantErr:     true,
			errContains: "месяц: invalid month",
		},
		{
			name: "CardItem: ошибка, неверный CVV (буквы)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "12A",
			},
			wantErr:     true,
			errContains: "CVV: invalid CVV",
		},
		{
			name: "CardItem: ошибка, неверный CVV (длина 2)",
			payload: model.CardPayload{
				Number:      "4111111111111111",
				Holder:      "IVAN IVANOV",
				ExpiryMonth: "12",
				ExpiryYear:  "2025",
				CVV:         "12",
			},
			wantErr:     true,
			errContains: "CVV: invalid CVV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(t.Context(), model.CardItem, tt.payload)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
