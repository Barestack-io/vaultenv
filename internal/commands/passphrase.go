package commands

import (
	"fmt"

	"github.com/Barestack-io/vaultenv/internal/config"
	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/personal"
	"github.com/Barestack-io/vaultenv/internal/storage"
	"github.com/spf13/cobra"
)

var passphraseCmd = &cobra.Command{
	Use:   "passphrase",
	Short: "Manage your vault passphrase",
	Long:  "Commands to change the passphrase that encrypts your private key in your personal vault.",
}

var passphraseRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Change your vault passphrase",
	Long: `Re-encrypt your private key with a new passphrase and upload it to your personal GitHub vault.

You must already be logged in (vaultenv login). This only updates keys/<username>.key.enc in your
personal repo; team .env files in org vaults are not re-encrypted.

On success, your local ~/.config/vaultenv/keys/ files are updated to match.`,
	RunE: runPassphraseRotate,
}

func init() {
	passphraseCmd.AddCommand(passphraseRotateCmd)
}

func runPassphraseRotate(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'vaultenv login' first: %w", err)
	}

	fmt.Println("Rotate vault passphrase for", cfg.Username)
	fmt.Println("Your current passphrase decrypts the key stored on GitHub; you will choose a new one.")

	current, err := crypto.PromptPassphraseUnlock("Current vault passphrase: ")
	if err != nil {
		return err
	}

	newPass, err := crypto.PromptPassphraseCreate("New vault passphrase: ", "Confirm new passphrase: ")
	if err != nil {
		return err
	}

	store := storage.NewGitHubStorage(cfg.AccessToken)
	if err := personal.RotatePassphrase(store, cfg.Username, current, newPass); err != nil {
		return err
	}

	fmt.Println("Vault passphrase updated. Other machines can use vaultenv login with the new passphrase.")
	return nil
}
