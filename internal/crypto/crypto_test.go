package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveKey(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantLen  int
		wantHash []byte
	}{
		{
			name:     "пустой пароль",
			password: "",
			wantLen:  32,
			wantHash: []byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		{
			name:     "обычный пароль",
			password: "my_secret_password",
			wantLen:  32,
			wantHash: []byte{
				0x65, 0x86, 0xbc, 0x3, 0x52, 0x2, 0xdf, 0xf9,
				0x8a, 0x67, 0xb8, 0x14, 0xac, 0xa6, 0x15, 0xe6,
				0x13, 0xcb, 0xbf, 0xae, 0x8f, 0xfa, 0x8f, 0x4a,
				0x47, 0x5d, 0xa0, 0xfa, 0xef, 0x7, 0x9c, 0x9d,
			},
		},
		{
			name:     "пароль со спецсимволами и пробелами",
			password: " p@ssw0rd! 123 ",
			wantLen:  32,
			wantHash: []byte{
				0x12, 0x96, 0x79, 0xc7, 0x59, 0x2f, 0x63, 0xf6,
				0xb2, 0xbc, 0xd3, 0xa5, 0xf1, 0x76, 0xbb, 0x9c,
				0x44, 0xa7, 0x1e, 0x0, 0xf1, 0xaf, 0x2, 0xf3,
				0x9b, 0x48, 0xf5, 0x77, 0xb6, 0x7c, 0x7e, 0xbc,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveKey(tt.password)

			assert.Len(t, got, tt.wantLen)
			assert.Equal(t, []byte(tt.wantHash), got)
		})
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c := New()
	validKey := DeriveKey("correct_password")

	tests := []struct {
		name      string
		plaintext []byte
		key       []byte
		wantErr   bool
	}{
		{
			name:      "успешное шифрование и дешифрование",
			plaintext: []byte("Hello, World! Это секретные данные."),
			key:       validKey,
			wantErr:   false,
		},
		{
			name:      "пустые данные",
			plaintext: []byte(nil),
			key:       validKey,
			wantErr:   false,
		},
		{
			name:      "невалидный ключ (слишком короткий)",
			plaintext: []byte("secret"),
			key:       []byte("short"),
			wantErr:   true,
		},
		{
			name:      "невалидный ключ (неверная длина, 30 байт)",
			plaintext: []byte("secret"),
			key:       make([]byte, 30),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := c.Encrypt(tt.plaintext, tt.key)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			decrypted, err := c.Decrypt(ciphertext, tt.key)
			require.NoError(t, err)

			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}
