package personal

import (
	"fmt"

	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

// RecoverPassphraseFromLocalKey re-encrypts the given raw private key with newPassphrase,
// uploads keys/<username>.key.enc to the personal vault, and refreshes local key files.
// It does not read or decrypt the existing .key.enc on GitHub. The local private key must
// match the public key already stored in the personal repo.
func RecoverPassphraseFromLocalKey(store storage.Provider, username string, privKey *[32]byte, newPassphrase string) error {
	repo := VaultRepo(username)
	keyPath := EncryptedPrivateKeyPath(username)
	pubPath := PublicKeyPath(username)

	pubKeyBytes, err := store.ReadFile(repo, pubPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}
	if pubKeyBytes == nil {
		return fmt.Errorf("no public key found in %s (expected %s)", repo, pubPath)
	}

	storedPub, err := crypto.DecodePublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("decoding public key: %w", err)
	}

	derivedPub := crypto.DerivePublicKey(privKey)
	if !crypto.PublicKeysEqual(storedPub, derivedPub) {
		return fmt.Errorf("local private.key does not match the public key in your personal vault (wrong backup or GitHub user?)")
	}

	newEnc, err := crypto.EncryptPrivateKey(privKey, newPassphrase)
	if err != nil {
		return fmt.Errorf("encrypting private key: %w", err)
	}
	crypto.PrintBlobFingerprint("encrypted key before upload", newEnc)

	if err := store.WriteFile(repo, keyPath, newEnc); err != nil {
		return fmt.Errorf("uploading encrypted key: %w", err)
	}

	verifyEnc, err := store.ReadFile(repo, keyPath)
	if err != nil {
		return fmt.Errorf("verifying uploaded key: %w", err)
	}
	crypto.PrintBlobFingerprint("encrypted key read back from GitHub", verifyEnc)

	if err := crypto.SaveKeyPair(privKey, storedPub); err != nil {
		return fmt.Errorf("saving keys locally: %w", err)
	}

	return nil
}
