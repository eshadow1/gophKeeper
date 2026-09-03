package loggers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLogger(t *testing.T) {
	tests := []struct {
		name        string
		levelLog    string
		expectError error
	}{
		{
			name:        "success_create_logger",
			levelLog:    "debug",
			expectError: nil,
		},
		{
			name:        "success_create_logger",
			levelLog:    "unknown",
			expectError: errors.New("unrecognized level: \"unknown\""),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CreateLogger(tc.levelLog)
			if tc.expectError != nil {
				assert.EqualError(t, err, tc.expectError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
