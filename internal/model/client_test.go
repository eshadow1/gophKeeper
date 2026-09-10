package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemType_Validate(t *testing.T) {
	tests := []struct {
		name     string
		itemType ItemType
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "валидный тип: login",
			itemType: LoginItem,
			wantErr:  false,
		},
		{
			name:     "валидный тип: text",
			itemType: TextItem,
			wantErr:  false,
		},
		{
			name:     "валидный тип: binary",
			itemType: BinaryItem,
			wantErr:  false,
		},
		{
			name:     "валидный тип: card",
			itemType: CardItem,
			wantErr:  false,
		},
		{
			name:     "невалидный тип: пустая строка",
			itemType: ItemType(""),
			wantErr:  true,
			errMsg:   "unknown item type: ",
		},
		{
			name:     "невалидный тип: произвольная строка",
			itemType: ItemType("video"),
			wantErr:  true,
			errMsg:   "unknown item type: video",
		},
		{
			name:     "невалидный тип: неверный регистр (Title Case)",
			itemType: ItemType("Login"),
			wantErr:  true,
			errMsg:   "unknown item type: Login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.itemType.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestItemType_String(t *testing.T) {
	tests := []struct {
		name     string
		itemType ItemType
		want     string
	}{
		{
			name:     "строковое представление login",
			itemType: LoginItem,
			want:     "login",
		},
		{
			name:     "строковое представление text",
			itemType: TextItem,
			want:     "text",
		},
		{
			name:     "строковое представление binary",
			itemType: BinaryItem,
			want:     "binary",
		},
		{
			name:     "строковое представление card",
			itemType: CardItem,
			want:     "card",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.itemType.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
