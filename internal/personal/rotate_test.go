package personal

import (
	"testing"

	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

func TestRotatePassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", tmpDir)

	const user = "alice"
	oldPass := "OldP@ssphrase9"
	newPass := "NewP@ssphrase9"

	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := crypto.EncryptPrivateKey(priv, oldPass)
	if err != nil {
		t.Fatal(err)
	}

	mock := storage.NewMockStorage()
	repo := VaultRepo(user)
	if err := mock.CreateRepo(user, SecretsRepoName, true); err != nil {
		t.Fatal(err)
	}
	if err := mock.WriteFile(repo, EncryptedPrivateKeyPath(user), enc); err != nil {
		t.Fatal(err)
	}
	if err := mock.WriteFile(repo, PublicKeyPath(user), crypto.EncodePublicKey(pub)); err != nil {
		t.Fatal(err)
	}

	if err := RotatePassphrase(mock, user, oldPass, newPass); err != nil {
		t.Fatalf("RotatePassphrase: %v", err)
	}

	gotEnc, err := mock.ReadFile(repo, EncryptedPrivateKeyPath(user))
	if err != nil || gotEnc == nil {
		t.Fatalf("read back enc: err=%v len=%d", err, len(gotEnc))
	}
	decPriv, err := crypto.DecryptPrivateKey(gotEnc, newPass)
	if err != nil {
		t.Fatalf("Decrypt with new pass: %v", err)
	}
	if *decPriv != *priv {
		t.Fatal("decrypted private key mismatch")
	}

	localPriv, err := crypto.LoadPrivateKey()
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	if *localPriv != *priv {
		t.Fatal("local private key should match after rotate")
	}
	localPub, err := crypto.LoadPublicKey()
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if *localPub != *pub {
		t.Fatal("local public key should match stored pub")
	}
}

func TestRotatePassphrasePublicKeyMismatch(t *testing.T) {
	t.Setenv("VAULTENV_CONFIG_DIR", t.TempDir())

	const user = "carol"
	priv, _, _ := crypto.GenerateKeyPair()
	_, wrongPub, _ := crypto.GenerateKeyPair()
	enc, _ := crypto.EncryptPrivateKey(priv, "GoodP@ssphrase1")

	mock := storage.NewMockStorage()
	_ = mock.CreateRepo(user, SecretsRepoName, true)
	repo := VaultRepo(user)
	_ = mock.WriteFile(repo, EncryptedPrivateKeyPath(user), enc)
	_ = mock.WriteFile(repo, PublicKeyPath(user), crypto.EncodePublicKey(wrongPub))

	err := RotatePassphrase(mock, user, "GoodP@ssphrase1", "OtherP@ssphrase1")
	if err == nil {
		t.Fatal("expected error when stored public key does not match private key")
	}
}

func TestRotatePassphraseWrongPassphrase(t *testing.T) {
	t.Setenv("VAULTENV_CONFIG_DIR", t.TempDir())

	const user = "bob"
	priv, pub, _ := crypto.GenerateKeyPair()
	enc, _ := crypto.EncryptPrivateKey(priv, "CorrectP@ss1")

	mock := storage.NewMockStorage()
	_ = mock.CreateRepo(user, SecretsRepoName, true)
	repo := VaultRepo(user)
	_ = mock.WriteFile(repo, EncryptedPrivateKeyPath(user), enc)
	_ = mock.WriteFile(repo, PublicKeyPath(user), crypto.EncodePublicKey(pub))

	err := RotatePassphrase(mock, user, "WrongP@ssphrase9", "OtherP@ssphrase9")
	if err == nil {
		t.Fatal("expected error for wrong current passphrase")
	}
}
