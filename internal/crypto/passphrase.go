package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
	"golang.org/x/text/unicode/norm"
)

const minPassphraseLen = 12

// NormalizePassphrase prepares a passphrase read from the terminal for use with
// Argon2 and storage. It:
//  1. Trims leading/trailing Unicode whitespace (strings.TrimSpace).
//  2. Strips a leading UTF-8 BOM (U+FEFF) if present (not removed by TrimSpace).
//  3. Applies Unicode NFC so the same visual string yields the same UTF-8 bytes
//     across OS/IME normalization differences (NFD vs NFC).
//
// Intentional leading/trailing spaces in a passphrase are not supported.
// Rare passphrases that depend on a specific non-NFC byte sequence for non-ASCII
// characters may need passphrase rotate after upgrading.
func NormalizePassphrase(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "\ufeff") {
		s = strings.TrimPrefix(s, "\ufeff")
	}
	return norm.NFC.String(s)
}

// PassphraseFingerprint returns the UTF-8 byte length and hex-encoded SHA-256 of
// the passphrase bytes (after the caller has applied NormalizePassphrase).
func PassphraseFingerprint(passphrase string) (length int, sha256Hex string) {
	b := []byte(passphrase)
	sum := sha256.Sum256(b)
	return len(b), hex.EncodeToString(sum[:])
}

// PrintPassphraseFingerprint writes length and SHA-256 of the passphrase to stderr
// for cross-machine verification without printing the secret.
func PrintPassphraseFingerprint(label string, passphrase string) {
	n, h := PassphraseFingerprint(passphrase)
	fmt.Fprintf(os.Stderr, "Passphrase fingerprint [%s]: length=%d sha256=%s\n", label, n, h)
}

// PrintBlobFingerprint writes length and SHA-256 of an arbitrary byte slice to stderr.
// Use to verify encrypted key blobs match between write (recover/rotate) and read (login).
func PrintBlobFingerprint(label string, data []byte) {
	sum := sha256.Sum256(data)
	fmt.Fprintf(os.Stderr, "Blob fingerprint [%s]: length=%d sha256=%s\n", label, len(data), hex.EncodeToString(sum[:]))
}

// ValidatePassphrase checks that a passphrase meets the minimum requirements:
// - at least 12 characters
// - at least one uppercase letter
// - at least one digit
// - at least one special character
func ValidatePassphrase(passphrase string) error {
	if len(passphrase) < minPassphraseLen {
		return fmt.Errorf("passphrase must be at least %d characters (got %d)", minPassphraseLen, len(passphrase))
	}

	var hasUpper, hasDigit, hasSpecial bool
	for _, r := range passphrase {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			hasSpecial = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "one uppercase letter")
	}
	if !hasDigit {
		missing = append(missing, "one digit")
	}
	if !hasSpecial {
		missing = append(missing, "one special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("passphrase must contain at least %s", strings.Join(missing, ", "))
	}

	return nil
}

func readPasswordNormalized(prompt string) (string, error) {
	fmt.Print(prompt)
	pass, err := term.ReadPassword(int(0))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	return NormalizePassphrase(string(pass)), nil
}

// PromptPassphraseUnlock prompts once for the vault passphrase (normalized).
// If prompt is empty, uses "Vault passphrase: ".
func PromptPassphraseUnlock(prompt string) (string, error) {
	if prompt == "" {
		prompt = "Vault passphrase: "
	}
	return readPasswordNormalized(prompt)
}

// PromptPassphraseCreate prompts for a new passphrase with validation and confirmation.
// Empty primaryPrompt / confirmPrompt use defaults "Vault passphrase: " and "Confirm passphrase: ".
func PromptPassphraseCreate(primaryPrompt, confirmPrompt string) (string, error) {
	fmt.Printf("Requirements: min %d chars, 1 uppercase, 1 digit, 1 special character\n", minPassphraseLen)
	if primaryPrompt == "" {
		primaryPrompt = "Vault passphrase: "
	}
	if confirmPrompt == "" {
		confirmPrompt = "Confirm passphrase: "
	}
	for {
		passphrase, err := readPasswordNormalized(primaryPrompt)
		if err != nil {
			return "", err
		}
		if err := ValidatePassphrase(passphrase); err != nil {
			fmt.Printf("  %v. Try again.\n", err)
			continue
		}

		confirm, err := readPasswordNormalized(confirmPrompt)
		if err != nil {
			return "", fmt.Errorf("reading confirmation: %w", err)
		}
		if confirm != passphrase {
			fmt.Println("  Passphrases don't match. Try again.")
			continue
		}
		return passphrase, nil
	}
}

// PromptPassphrase prompts the user for a passphrase via the terminal.
// If create is true, asks for confirmation and validates the passphrase.
func PromptPassphrase(create bool) (string, error) {
	if create {
		return PromptPassphraseCreate("", "")
	}
	return PromptPassphraseUnlock("")
}
