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

// promptPassphrase is the passphrase prompt entry point used by setupKeys.
// It is a package-level var so tests can replace it with a stub that returns
// a deterministic passphrase without touching the terminal. Production code
// always uses crypto.PromptPassphrase.
var promptPassphrase = crypto.PromptPassphrase

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

// vaultState describes what's currently in the user's personal vault repo.
// It is the return value of determineVaultState, which is the pure decision
// function driving setupKeys. Keeping this logic in a pure function lets us
// exhaustively test the state machine without touching the passphrase prompt
// or local disk.
type vaultState int

const (
	// vaultStateFresh: the repo does not exist. Need to create it, write the
	// marker, and generate + upload keys.
	vaultStateFresh vaultState = iota

	// vaultStateIncomplete: the repo exists but has no marker and no keys.
	// This indicates a crashed first-time setup (e.g. CreateRepo succeeded
	// but a subsequent WriteFile hit the GitHub post-creation race and
	// returned 404). Recovery: write the marker, generate + upload keys.
	vaultStateIncomplete

	// vaultStateNeedsKeys: the repo and marker exist but there is no
	// encrypted key. Generate and upload keys without re-creating the repo
	// or marker.
	vaultStateNeedsKeys

	// vaultStateReady: repo, marker, and encrypted key are all present.
	// Prompt for the passphrase and unlock.
	vaultStateReady

	// vaultStateConflict: the repo exists and contains encrypted key material
	// but has no marker. This is a suspicious state (likely tampering or a
	// name collision with a non-vaultenv repo that happens to have a keys/
	// directory) and we refuse to touch it.
	vaultStateConflict
)

// determineVaultState inspects the user's personal vault repo and returns
// the state that setupKeys should act on. It performs only reads; no writes
// or prompts.
func determineVaultState(store storage.Provider, username string) (vaultState, error) {
	personalVault := personal.VaultRepo(username)

	exists, err := store.RepoExists(username, personal.SecretsRepoName)
	if err != nil {
		return 0, fmt.Errorf("checking personal vault: %w", err)
	}
	if !exists {
		return vaultStateFresh, nil
	}

	marker, err := store.ReadFile(personalVault, personal.MarkerPath)
	if err != nil {
		return 0, fmt.Errorf("reading vault marker: %w", err)
	}

	encKey, err := store.ReadFile(personalVault, personal.EncryptedPrivateKeyPath(username))
	if err != nil {
		return 0, fmt.Errorf("reading encrypted key: %w", err)
	}

	if marker == nil {
		if encKey != nil {
			// Keys without a marker: refuse, this is not a state we created.
			return vaultStateConflict, nil
		}
		// No marker, no keys: crashed first-time setup, recoverable.
		return vaultStateIncomplete, nil
	}

	if encKey == nil {
		return vaultStateNeedsKeys, nil
	}

	return vaultStateReady, nil
}

func setupKeys(store storage.Provider, username string) error {
	state, err := determineVaultState(store, username)
	if err != nil {
		return err
	}

	personalVault := personal.VaultRepo(username)

	switch state {
	case vaultStateConflict:
		return fmt.Errorf("repo %s exists but is not a vaultenv vault", personalVault)

	case vaultStateReady:
		return unlockExistingVault(store, username)

	case vaultStateFresh, vaultStateIncomplete, vaultStateNeedsKeys:
		return runFirstTimeSetup(store, username, state)
	}

	return fmt.Errorf("unexpected vault state: %v", state)
}

// runFirstTimeSetup prompts for a passphrase, generates keys, and uploads
// them. It also creates the repo (for vaultStateFresh) and writes the marker
// (for vaultStateFresh and vaultStateIncomplete). It is the only code path
// that performs the interactive passphrase prompt, so it is kept separate
// from the pure state-machine helper above.
func runFirstTimeSetup(store storage.Provider, username string, state vaultState) error {
	personalVault := personal.VaultRepo(username)

	fmt.Println("First-time setup: generating encryption keys.")
	fmt.Println("Choose a vault passphrase to protect your private key.")
	passphrase, err := promptPassphrase(true)
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

	if state == vaultStateFresh {
		if err := store.CreateRepo(username, personal.SecretsRepoName, true); err != nil {
			return fmt.Errorf("creating personal vault: %w", err)
		}
	}

	// Write the marker for Fresh and Incomplete. For NeedsKeys the marker
	// already exists and we skip this write entirely.
	if state == vaultStateFresh || state == vaultStateIncomplete {
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

// unlockExistingVault handles the vaultStateReady path: read the encrypted
// key from GitHub, prompt the user for the passphrase, and persist the
// decrypted keypair locally.
func unlockExistingVault(store storage.Provider, username string) error {
	personalVault := personal.VaultRepo(username)
	keyPath := personal.EncryptedPrivateKeyPath(username)

	encKey, err := store.ReadFile(personalVault, keyPath)
	if err != nil {
		return fmt.Errorf("reading encrypted key: %w", err)
	}

	crypto.PrintBlobFingerprint("encrypted key from GitHub", encKey)
	fmt.Println("Found existing encryption keys. Enter your vault passphrase to unlock.")
	var privKey *[32]byte
	for attempt := 1; attempt <= maxVaultPassphraseAttempts; attempt++ {
		passphrase, err := promptPassphrase(false)
		if err != nil {
			return err
		}
		crypto.PrintPassphraseFingerprint("unlock attempt", passphrase)
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
