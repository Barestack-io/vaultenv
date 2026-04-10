package personal

import (
	"fmt"

	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

// RotatePassphrase decrypts the user's encrypted private key with currentPassphrase,
// re-encrypts with newPassphrase, uploads it to the personal vault, and refreshes
// local key files. Fails without writing to GitHub if decryption or validation fails.
func RotatePassphrase(store storage.Provider, username, currentPassphrase, newPassphrase string) error {
	repo := VaultRepo(username)
	keyPath := EncryptedPrivateKeyPath(username)
	pubPath := PublicKeyPath(username)

	encKey, err := store.ReadFile(repo, keyPath)
	if err != nil {
		return fmt.Errorf("reading encrypted key: %w", err)
	}
	if encKey == nil {
		return fmt.Errorf("no encrypted private key found in %s (expected %s)", repo, keyPath)
	}

	pubKeyBytes, err := store.ReadFile(repo, pubPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}
	if pubKeyBytes == nil {
		return fmt.Errorf("no public key found in %s (expected %s)", repo, pubPath)
	}

	privKey, err := crypto.DecryptPrivateKey(encKey, currentPassphrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt key (wrong passphrase?): %w", err)
	}

	storedPub, err := crypto.DecodePublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("decoding public key: %w", err)
	}

	derivedPub := crypto.DerivePublicKey(privKey)
	if !crypto.PublicKeysEqual(storedPub, derivedPub) {
		return fmt.Errorf("public key in vault does not match decrypted private key (corrupted key material?)")
	}

	newEnc, err := crypto.EncryptPrivateKey(privKey, newPassphrase)
	if err != nil {
		return fmt.Errorf("encrypting private key: %w", err)
	}

	if err := store.WriteFile(repo, keyPath, newEnc); err != nil {
		return fmt.Errorf("uploading encrypted key: %w", err)
	}

	if err := crypto.SaveKeyPair(privKey, storedPub); err != nil {
		return fmt.Errorf("saving keys locally: %w", err)
	}

	return nil
}
