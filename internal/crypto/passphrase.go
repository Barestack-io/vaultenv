package crypto

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/term"
)

const minPassphraseLen = 12

// NormalizePassphrase trims leading and trailing Unicode whitespace as defined by
// strings.TrimSpace, including \r and \n. Use on every passphrase read from the
// terminal so pasted passphrases and Windows line endings do not break unlock.
// Intentional leading/trailing spaces in a passphrase are not supported after normalization.
func NormalizePassphrase(s string) string {
	return strings.TrimSpace(s)
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
