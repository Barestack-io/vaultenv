package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
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
			name:       "unicode punctuation counts as special",
			passphrase: "MyPassword1·",
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

func TestNormalizePassphrase(t *testing.T) {
	if got := NormalizePassphrase("  MyP@ssw0rd123  "); got != "MyP@ssw0rd123" {
		t.Errorf("got %q want %q", got, "MyP@ssw0rd123")
	}
	if got := NormalizePassphrase("x\r\n"); got != "x" {
		t.Errorf("got %q want %q", got, "x")
	}
	ascii := "c5dpzvimJvrx98a!"
	if got := NormalizePassphrase(ascii); got != ascii {
		t.Errorf("ASCII passphrase unchanged: got %q want %q", got, ascii)
	}
}

func TestNormalizePassphraseStripsBOM(t *testing.T) {
	withBOM := "\ufeffMyP@ssw0rd123"
	got := NormalizePassphrase(withBOM)
	want := "MyP@ssw0rd123"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if err := ValidatePassphrase(got); err != nil {
		t.Errorf("ValidatePassphrase after BOM strip: %v", err)
	}
}

func TestNormalizePassphraseNFCEquivalence(t *testing.T) {
	// NFC: precomposed é; NFD: e + combining acute
	nfc := "MyP@ssw0rd1" + "\u00e9" + "3"
	nfd := "MyP@ssw0rd1" + "e\u0301" + "3"
	gotNFC := NormalizePassphrase(nfc)
	gotNFD := NormalizePassphrase(nfd)
	if gotNFC != gotNFD {
		t.Errorf("NFC and NFD forms should normalize equal: %q vs %q", gotNFC, gotNFD)
	}
	if gotNFC != norm.NFC.String(nfd) {
		t.Errorf("expected NFC canonical form, got %q", gotNFC)
	}
	if err := ValidatePassphrase(gotNFC); err != nil {
		t.Errorf("ValidatePassphrase: %v", err)
	}
}

func TestPassphraseFingerprint(t *testing.T) {
	s := "MyP@ssw0rd123"
	n, h := PassphraseFingerprint(s)
	if n != len([]byte(s)) {
		t.Errorf("length: got %d want %d", n, len([]byte(s)))
	}
	sum := sha256.Sum256([]byte(s))
	want := hex.EncodeToString(sum[:])
	if h != want {
		t.Errorf("sha256: got %s want %s", h, want)
	}
}
