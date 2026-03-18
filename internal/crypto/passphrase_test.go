package crypto

import (
	"strings"
	"testing"
)

func TestValidatePassphrase(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
		wantErr    bool
		errContains string
	}{
		{
			name:       "valid passphrase",
			passphrase: "MyP@ssw0rd123",
			wantErr:    false,
		},
		{
			name:       "exactly 12 chars valid",
			passphrase: "Abcdef1234!@",
			wantErr:    false,
		},
		{
			name:        "too short",
			passphrase:  "Short1!",
			wantErr:     true,
			errContains: "at least 12 characters",
		},
		{
			name:        "missing uppercase",
			passphrase:  "alllowercase1!",
			wantErr:     true,
			errContains: "one uppercase letter",
		},
		{
			name:        "missing digit",
			passphrase:  "NoDigitsHere!!",
			wantErr:     true,
			errContains: "one digit",
		},
		{
			name:        "missing special char",
			passphrase:  "NoSpecial12345",
			wantErr:     true,
			errContains: "one special character",
		},
		{
			name:        "missing multiple requirements",
			passphrase:  "alllowercase!",
			wantErr:     true,
			errContains: "one uppercase letter",
		},
		{
			name:        "empty string",
			passphrase:  "",
			wantErr:     true,
			errContains: "at least 12 characters",
		},
		{
			name:       "unicode special chars count",
			passphrase: "MyPassword1 ",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassphrase(tt.passphrase)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
