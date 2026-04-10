package personal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

// TestRecoverPassphraseLocalKeyDownloadDecryptRoundtrip simulates a real workflow:
// 1) A raw private.key already exists under the vaultenv config dir (like ~/.config/vaultenv/keys).
// 2) Recover re-encrypts and uploads keys/<user>.key.enc to the personal vault.
// 3) The encrypted blob is written to a separate temp dir (as if downloaded from GitHub).
// 4) Decrypting that file with the new passphrase yields the same private key material.
func TestRecoverPassphraseLocalKeyDownloadDecryptRoundtrip(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", configDir)

	const user = "roundtrip_user"
	newPass := "N3wP@ssphrase!9"

	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.SaveKeyPair(priv, pub); err != nil {
		t.Fatalf("SaveKeyPair (simulate existing private.key): %v", err)
	}

	loadedPriv, err := crypto.LoadPrivateKey()
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	if *loadedPriv != *priv {
		t.Fatal("loaded private key should match saved key")
	}

	mock := storage.NewMockStorage()
	repo := VaultRepo(user)
	if err := mock.CreateRepo(user, SecretsRepoName, true); err != nil {
		t.Fatal(err)
	}
	if err := mock.WriteFile(repo, PublicKeyPath(user), crypto.EncodePublicKey(pub)); err != nil {
		t.Fatal(err)
	}

	if err := RecoverPassphraseFromLocalKey(mock, user, loadedPriv, newPass); err != nil {
		t.Fatalf("RecoverPassphraseFromLocalKey: %v", err)
	}

	encOnServer, err := mock.ReadFile(repo, EncryptedPrivateKeyPath(user))
	if err != nil || encOnServer == nil {
		t.Fatalf("read encrypted key from storage: err=%v len=%d", err, len(encOnServer))
	}

	downloadDir := t.TempDir()
	downloadedPath := filepath.Join(downloadDir, user+".key.enc")
	if err := os.WriteFile(downloadedPath, encOnServer, 0600); err != nil {
		t.Fatalf("write downloaded blob: %v", err)
	}

	downloadedBytes, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}

	decPriv, err := crypto.DecryptPrivateKey(downloadedBytes, newPass)
	if err != nil {
		t.Fatalf("DecryptPrivateKey(downloaded, newPass): %v", err)
	}
	if *decPriv != *priv {
		t.Fatal("decrypted key from downloaded blob does not match original private key")
	}
}
