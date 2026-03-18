package vault

import (
	"encoding/json"
	"testing"

	"github.com/scaler/vaultenv/internal/storage"
)

func TestNewVaultConfig(t *testing.T) {
	vc := NewVaultConfig("org/repo", "octocat", "pubkey123")

	if vc.Version != 1 {
		t.Errorf("expected version 1, got %d", vc.Version)
	}
	if vc.Repo != "org/repo" {
		t.Errorf("expected repo org/repo, got %s", vc.Repo)
	}
	if vc.Owner != "octocat" {
		t.Errorf("expected owner octocat, got %s", vc.Owner)
	}

	user, ok := vc.ApprovedUsers["octocat"]
	if !ok {
		t.Fatal("owner should be in approved users")
	}
	if user.PublicKey != "pubkey123" {
		t.Errorf("expected public key pubkey123, got %s", user.PublicKey)
	}

	if len(vc.PendingRequests) != 0 {
		t.Errorf("pending requests should be empty")
	}
	if len(vc.DeployKeys) != 0 {
		t.Errorf("deploy keys should be empty")
	}
	if len(vc.Environments) != 0 {
		t.Errorf("environments should be empty")
	}
}

func TestParseVaultConfigValid(t *testing.T) {
	data := []byte(`{
		"version": 1,
		"repo": "org/repo",
		"owner": "alice",
		"approved_users": {
			"alice": {"public_key": "key1", "approved_at": "2026-01-01T00:00:00Z"}
		},
		"environments": ["staging", "production"]
	}`)

	vc, err := ParseVaultConfig(data)
	if err != nil {
		t.Fatalf("ParseVaultConfig: %v", err)
	}

	if vc.Owner != "alice" {
		t.Errorf("expected owner alice, got %s", vc.Owner)
	}
	if len(vc.ApprovedUsers) != 1 {
		t.Errorf("expected 1 approved user, got %d", len(vc.ApprovedUsers))
	}
	if len(vc.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(vc.Environments))
	}
	// Nil maps should be initialized
	if vc.PendingRequests == nil {
		t.Error("PendingRequests should be initialized to empty map")
	}
	if vc.DeployKeys == nil {
		t.Error("DeployKeys should be initialized to empty map")
	}
}

func TestParseVaultConfigInvalid(t *testing.T) {
	_, err := ParseVaultConfig([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalUnmarshalEnvelopesRoundtrip(t *testing.T) {
	original := map[string]Envelope{
		"alice": {
			EncryptedKey:    "enckey1",
			Nonce:           "nonce1",
			EphemeralPublic: "eph1",
		},
		"dk:ci-staging": {
			EncryptedKey:    "enckey2",
			Nonce:           "nonce2",
			EphemeralPublic: "eph2",
		},
	}

	data, err := MarshalEnvelopes(original)
	if err != nil {
		t.Fatalf("MarshalEnvelopes: %v", err)
	}

	result, err := UnmarshalEnvelopes(data)
	if err != nil {
		t.Fatalf("UnmarshalEnvelopes: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 envelopes, got %d", len(result))
	}

	for key, orig := range original {
		got, ok := result[key]
		if !ok {
			t.Errorf("missing envelope for %s", key)
			continue
		}
		if got.EncryptedKey != orig.EncryptedKey || got.Nonce != orig.Nonce || got.EphemeralPublic != orig.EphemeralPublic {
			t.Errorf("envelope for %s doesn't match", key)
		}
	}
}

func TestEncodeDecodeDeployKeyRoundtrip(t *testing.T) {
	var privKey [32]byte
	copy(privKey[:], []byte("32-byte-test-private-key-value!!"))

	original := &DeployKeyToken{
		VaultRepo:    "org/vaultenv-secrets",
		SourceRepo:   "customer-portal",
		KeyName:      "ci-staging",
		PrivateKey:   privKey,
		Environments: []string{"staging", "production"},
	}

	token, err := EncodeDeployKey(original)
	if err != nil {
		t.Fatalf("EncodeDeployKey: %v", err)
	}

	if token == "" {
		t.Fatal("token should not be empty")
	}
	if len(token) < len(deployKeyPrefix) {
		t.Fatal("token should start with prefix")
	}

	decoded, err := DecodeDeployKey(token)
	if err != nil {
		t.Fatalf("DecodeDeployKey: %v", err)
	}

	if decoded.VaultRepo != original.VaultRepo {
		t.Errorf("VaultRepo: got %s, want %s", decoded.VaultRepo, original.VaultRepo)
	}
	if decoded.SourceRepo != original.SourceRepo {
		t.Errorf("SourceRepo: got %s, want %s", decoded.SourceRepo, original.SourceRepo)
	}
	if decoded.KeyName != original.KeyName {
		t.Errorf("KeyName: got %s, want %s", decoded.KeyName, original.KeyName)
	}
	if decoded.PrivateKey != original.PrivateKey {
		t.Error("PrivateKey should match original")
	}
	if len(decoded.Environments) != 2 || decoded.Environments[0] != "staging" || decoded.Environments[1] != "production" {
		t.Errorf("Environments: got %v, want [staging production]", decoded.Environments)
	}
}

func TestDecodeDeployKeyInvalidPrefix(t *testing.T) {
	_, err := DecodeDeployKey("bad_prefix_abc123")
	if err == nil {
		t.Error("expected error for invalid prefix")
	}
}

func TestDecodeDeployKeyInvalidBase64(t *testing.T) {
	_, err := DecodeDeployKey(deployKeyPrefix + "not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestLoadWriteVaultConfigRoundtrip(t *testing.T) {
	mock := storage.NewMockStorage()
	_ = mock.CreateRepo("org", "vaultenv-secrets", true)

	vc := NewVaultConfig("org/repo", "alice", "pubkey")
	vc.Environments = []string{"staging"}

	err := WriteVaultConfig(mock, "org/vaultenv-secrets", "repo", vc)
	if err != nil {
		t.Fatalf("WriteVaultConfig: %v", err)
	}

	loaded, err := LoadVaultConfig(mock, "org/vaultenv-secrets", "repo")
	if err != nil {
		t.Fatalf("LoadVaultConfig: %v", err)
	}

	if loaded.Owner != "alice" {
		t.Errorf("expected owner alice, got %s", loaded.Owner)
	}
	if len(loaded.Environments) != 1 || loaded.Environments[0] != "staging" {
		t.Errorf("environments mismatch: %v", loaded.Environments)
	}
}

func TestLoadVaultConfigNotFound(t *testing.T) {
	mock := storage.NewMockStorage()
	_, err := LoadVaultConfig(mock, "org/vaultenv-secrets", "nonexistent")
	if err == nil {
		t.Error("expected error when vault config not found")
	}
}

func TestDiscoverOrCreateVault_CreatesDefault(t *testing.T) {
	mock := storage.NewMockStorage()

	name, err := DiscoverOrCreateVault(mock, "myorg")
	if err != nil {
		t.Fatalf("DiscoverOrCreateVault: %v", err)
	}
	if name != "vaultenv-secrets" {
		t.Errorf("expected vaultenv-secrets, got %s", name)
	}

	exists, _ := mock.RepoExists("myorg", "vaultenv-secrets")
	if !exists {
		t.Error("repo should exist after creation")
	}

	marker, _ := mock.ReadFile("myorg/vaultenv-secrets", ".vaultenv-repo.json")
	if marker == nil {
		t.Error("marker file should exist")
	}

	var m RepoMarker
	if err := json.Unmarshal(marker, &m); err != nil {
		t.Fatalf("parsing marker: %v", err)
	}
	if m.Type != "org" {
		t.Errorf("expected type org, got %s", m.Type)
	}
}

func TestDiscoverOrCreateVault_FindsExisting(t *testing.T) {
	mock := storage.NewMockStorage()
	_ = mock.CreateRepo("myorg", "vaultenv-secrets", true)
	_ = mock.WriteFile("myorg/vaultenv-secrets", ".vaultenv-repo.json",
		[]byte(`{"version":1,"type":"org","namespace":"myorg"}`))

	name, err := DiscoverOrCreateVault(mock, "myorg")
	if err != nil {
		t.Fatalf("DiscoverOrCreateVault: %v", err)
	}
	if name != "vaultenv-secrets" {
		t.Errorf("expected vaultenv-secrets, got %s", name)
	}
}

func TestDiscoverOrCreateVault_FallsBackWhenNameTaken(t *testing.T) {
	mock := storage.NewMockStorage()
	// Create vaultenv-secrets but WITHOUT the marker (name collision)
	_ = mock.CreateRepo("myorg", "vaultenv-secrets", true)

	name, err := DiscoverOrCreateVault(mock, "myorg")
	if err != nil {
		t.Fatalf("DiscoverOrCreateVault: %v", err)
	}
	if name != "vaultenv-vault" {
		t.Errorf("expected vaultenv-vault fallback, got %s", name)
	}
}

func TestDiscoverOrCreateVault_ErrorWhenBothTaken(t *testing.T) {
	mock := storage.NewMockStorage()
	// Both names taken without markers
	_ = mock.CreateRepo("myorg", "vaultenv-secrets", true)
	_ = mock.CreateRepo("myorg", "vaultenv-vault", true)

	_, err := DiscoverOrCreateVault(mock, "myorg")
	if err == nil {
		t.Error("expected error when both names are taken")
	}
}
