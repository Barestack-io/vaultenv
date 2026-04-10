package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/Barestack-io/vaultenv/internal/auth"
	"github.com/Barestack-io/vaultenv/internal/config"
	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/personal"
	"github.com/Barestack-io/vaultenv/internal/storage"
	"github.com/spf13/cobra"
)

const maxVaultPassphraseAttempts = 5

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GitHub using device flow",
	RunE:  runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	provider := auth.NewGitHubAuth()

	fmt.Println("Authenticating with GitHub...")
	token, err := provider.Login()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	user, err := provider.GetUser(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	fmt.Printf("Authenticated as %s\n", user.Username)

	cfg := &config.GlobalConfig{
		AccessToken: token.AccessToken,
		Username:    user.Username,
	}
	if err := config.SaveGlobal(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	store := storage.NewGitHubStorage(token.AccessToken)
	if err := setupKeys(store, user.Username); err != nil {
		return fmt.Errorf("key setup failed: %w", err)
	}

	fmt.Println("Login complete. Run 'vaultenv link' in a git repo to get started.")
	return nil
}

func setupKeys(store storage.Provider, username string) error {
	personalVault := personal.VaultRepo(username)

	exists, err := store.RepoExists(username, personal.SecretsRepoName)
	if err != nil {
		return fmt.Errorf("checking personal vault: %w", err)
	}

	if exists {
		marker, err := store.ReadFile(personalVault, personal.MarkerPath)
		if err != nil {
			return fmt.Errorf("personal vault exists but can't read marker: %w", err)
		}
		if marker == nil {
			return fmt.Errorf("repo %s exists but is not a vaultenv vault", personalVault)
		}

		keyPath := personal.EncryptedPrivateKeyPath(username)
		encKey, err := store.ReadFile(personalVault, keyPath)
		if err != nil {
			return fmt.Errorf("reading encrypted key: %w", err)
		}
		if encKey != nil {
			fmt.Println("Found existing encryption keys. Enter your vault passphrase to unlock.")
			var privKey *[32]byte
			for attempt := 1; attempt <= maxVaultPassphraseAttempts; attempt++ {
				passphrase, err := crypto.PromptPassphrase(false)
				if err != nil {
					return err
				}
				privKey, err = crypto.DecryptPrivateKey(encKey, passphrase)
				if err == nil {
					break
				}
				if !errors.Is(err, crypto.ErrWrongPrivateKeyPassphrase) {
					return fmt.Errorf("failed to decrypt key: %w", err)
				}
				remaining := maxVaultPassphraseAttempts - attempt
				if remaining > 0 {
					fmt.Fprintf(os.Stderr, "Incorrect passphrase. %d attempt(s) remaining.\n", remaining)
					continue
				}
				return fmt.Errorf("vault passphrase incorrect after %d attempts; run vaultenv login to try again", maxVaultPassphraseAttempts)
			}
			pubPath := personal.PublicKeyPath(username)
			pubKeyBytes, err := store.ReadFile(personalVault, pubPath)
			if err != nil {
				return fmt.Errorf("reading public key: %w", err)
			}
			pubKey, err := crypto.DecodePublicKey(pubKeyBytes)
			if err != nil {
				return fmt.Errorf("decoding public key: %w", err)
			}
			return crypto.SaveKeyPair(privKey, pubKey)
		}
	}

	// First-time setup
	fmt.Println("First-time setup: generating encryption keys.")
	fmt.Println("Choose a vault passphrase to protect your private key.")
	passphrase, err := crypto.PromptPassphrase(true)
	if err != nil {
		return err
	}
	crypto.PrintPassphraseFingerprint("first-time vault passphrase", passphrase)

	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generating keypair: %w", err)
	}

	encPriv, err := crypto.EncryptPrivateKey(priv, passphrase)
	if err != nil {
		return fmt.Errorf("encrypting private key: %w", err)
	}

	if err := crypto.SaveKeyPair(priv, pub); err != nil {
		return fmt.Errorf("saving keys locally: %w", err)
	}

	if !exists {
		if err := store.CreateRepo(username, personal.SecretsRepoName, true); err != nil {
			return fmt.Errorf("creating personal vault: %w", err)
		}

		if err := store.WriteFile(personalVault, personal.MarkerPath, []byte(personal.MarkerJSON(username))); err != nil {
			return fmt.Errorf("writing vault marker: %w", err)
		}
	}

	keyPath := personal.EncryptedPrivateKeyPath(username)
	if err := store.WriteFile(personalVault, keyPath, encPriv); err != nil {
		return fmt.Errorf("uploading encrypted key: %w", err)
	}

	pubPath := personal.PublicKeyPath(username)
	if err := store.WriteFile(personalVault, pubPath, crypto.EncodePublicKey(pub)); err != nil {
		return fmt.Errorf("uploading public key: %w", err)
	}

	fmt.Println("Encryption keys generated and stored.")
	return nil
}
