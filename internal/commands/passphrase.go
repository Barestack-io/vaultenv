package commands

import (
	"fmt"
	"os"

	"github.com/Barestack-io/vaultenv/internal/config"
	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/personal"
	"github.com/Barestack-io/vaultenv/internal/storage"
	"github.com/spf13/cobra"
)

var passphraseCmd = &cobra.Command{
	Use:   "passphrase",
	Short: "Manage your vault passphrase",
	Long:  "Commands to change or recover the passphrase that encrypts your private key in your personal vault (see rotate and recover).",
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

var passphraseRecoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Set a new vault passphrase using your local private.key",
	Long: `Use when you forgot your vault passphrase but still have the raw private key on this machine
(e.g. under your vaultenv config directory keys/private.key after a successful pull).

Reads keys/<username>.pub from your personal GitHub repo, verifies it matches your local private.key,
then uploads a new keys/<username>.key.enc encrypted with a new passphrase. Org .env ciphertext is
unchanged. After this, other machines can run vaultenv login with the new passphrase.

Requires vaultenv login (valid token). This overwrites the encrypted key on GitHub — only run if you
are sure this machine's private.key belongs to the logged-in GitHub user.`,
	RunE: runPassphraseRecover,
}

func init() {
	passphraseCmd.AddCommand(passphraseRotateCmd)
	passphraseCmd.AddCommand(passphraseRecoverCmd)
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
	crypto.PrintPassphraseFingerprint("current vault passphrase", current)

	newPass, err := crypto.PromptPassphraseCreate("New vault passphrase: ", "Confirm new passphrase: ")
	if err != nil {
		return err
	}
	crypto.PrintPassphraseFingerprint("new vault passphrase", newPass)

	store := storage.NewGitHubStorage(cfg.AccessToken)
	if err := personal.RotatePassphrase(store, cfg.Username, current, newPass); err != nil {
		return storage.WrapGitHubError(err)
	}

	fmt.Println("Vault passphrase updated. Other machines can use vaultenv login with the new passphrase.")
	return nil
}

func runPassphraseRecover(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'vaultenv login' first: %w", err)
	}

	privKey, err := crypto.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("could not load local private key: %w\n\n"+
			"Expected a 32-byte keys/private.key under your vaultenv config directory.\n"+
			"Default paths: Linux ~/.config/vaultenv/keys; macOS ~/Library/Application Support/vaultenv/keys.\n"+
			"Override with VAULTENV_CONFIG_DIR.", err)
	}

	fmt.Fprintf(os.Stderr, "WARNING: This will replace keys/%s.key.enc on GitHub with a new passphrase wrap.\n", cfg.Username)
	fmt.Fprintln(os.Stderr, "Only continue if this machine's private.key belongs to GitHub user", cfg.Username+".")
	fmt.Println()

	newPass, err := crypto.PromptPassphraseCreate("New vault passphrase: ", "Confirm new passphrase: ")
	if err != nil {
		return err
	}
	crypto.PrintPassphraseFingerprint("new vault passphrase", newPass)

	store := storage.NewGitHubStorage(cfg.AccessToken)
	if err := personal.RecoverPassphraseFromLocalKey(store, cfg.Username, privKey, newPass); err != nil {
		return storage.WrapGitHubError(err)
	}

	fmt.Println("Recovery complete. Use this new passphrase when you run vaultenv login on other machines.")
	fmt.Println("Your existing vault repos and .env ciphertext are unchanged.")
	return nil
}
