package personal

import "testing"

func TestVaultRepoPaths(t *testing.T) {
	if got := VaultRepo("alice"); got != "alice/vaultenv-secrets" {
		t.Errorf("VaultRepo: %q", got)
	}
	if got := EncryptedPrivateKeyPath("alice"); got != "keys/alice.key.enc" {
		t.Errorf("EncryptedPrivateKeyPath: %q", got)
	}
	if got := PublicKeyPath("alice"); got != "keys/alice.pub" {
		t.Errorf("PublicKeyPath: %q", got)
	}
	if SecretsRepoName != "vaultenv-secrets" {
		t.Errorf("SecretsRepoName: %q", SecretsRepoName)
	}
}
