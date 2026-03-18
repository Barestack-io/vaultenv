package crypto

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/term"
)

const minPassphraseLen = 12

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

// PromptPassphrase prompts the user for a passphrase via the terminal.
// If create is true, asks for confirmation and validates the passphrase.
func PromptPassphrase(create bool) (string, error) {
	if create {
		fmt.Printf("Requirements: min %d chars, 1 uppercase, 1 digit, 1 special character\n", minPassphraseLen)
	}

	for {
		fmt.Print("Vault passphrase: ")
		pass, err := term.ReadPassword(int(0))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("reading passphrase: %w", err)
		}

		passphrase := string(pass)

		if create {
			if err := ValidatePassphrase(passphrase); err != nil {
				fmt.Printf("  %v. Try again.\n", err)
				continue
			}

			fmt.Print("Confirm passphrase: ")
			confirm, err := term.ReadPassword(int(0))
			fmt.Println()
			if err != nil {
				return "", fmt.Errorf("reading confirmation: %w", err)
			}

			if string(confirm) != passphrase {
				fmt.Println("  Passphrases don't match. Try again.")
				continue
			}
		}

		return passphrase, nil
	}
}
