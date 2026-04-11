package commands

import (
	"strings"
	"testing"

	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/personal"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

// stubPrompter replaces promptPassphrase for the duration of a test with a
// function that returns a fixed passphrase, so setupKeys can be driven end-
// to-end without blocking on terminal input.
func stubPrompter(t *testing.T, passphrase string) {
	t.Helper()
	orig := promptPassphrase
	promptPassphrase = func(create bool) (string, error) {
		return passphrase, nil
	}
	t.Cleanup(func() { promptPassphrase = orig })
}

// redirectKeyStorage points crypto.SaveKeyPair (and friends) at a per-test
// temp directory via the VAULTENV_CONFIG_DIR env var, so tests don't stomp
// on the developer's real vaultenv config.
func redirectKeyStorage(t *testing.T) {
	t.Helper()
	t.Setenv("VAULTENV_CONFIG_DIR", t.TempDir())
}

// TestDetermineVaultState exhaustively covers the state-machine decisions
// that setupKeys makes. The key row is "incomplete_crashed_first_time_setup"
// — that's the regression guard for the bug where a repo created but left
// without its marker would permanently lock the user out.
func TestDetermineVaultState(t *testing.T) {
	const user = "alice"
	vault := personal.VaultRepo(user)
	keyPath := personal.EncryptedPrivateKeyPath(user)
	pubPath := personal.PublicKeyPath(user)

	tests := []struct {
		name  string
		setup func(*storage.MockStorage)
		want  vaultState
	}{
		{
			name:  "fresh_no_repo",
			setup: func(m *storage.MockStorage) {},
			want:  vaultStateFresh,
		},
		{
			name: "incomplete_crashed_first_time_setup",
			setup: func(m *storage.MockStorage) {
				// Repo was created but the marker write failed (the bug).
				// No marker, no keys — must be classified as Incomplete so
				// setupKeys can recover by running first-time setup.
				_ = m.CreateRepo(user, personal.SecretsRepoName, true)
			},
			want: vaultStateIncomplete,
		},
		{
			name: "needs_keys_marker_present_no_encrypted_key",
			setup: func(m *storage.MockStorage) {
				_ = m.CreateRepo(user, personal.SecretsRepoName, true)
				_ = m.WriteFile(vault, personal.MarkerPath, []byte(personal.MarkerJSON(user)))
			},
			want: vaultStateNeedsKeys,
		},
		{
			name: "ready_marker_and_encrypted_key_present",
			setup: func(m *storage.MockStorage) {
				_ = m.CreateRepo(user, personal.SecretsRepoName, true)
				_ = m.WriteFile(vault, personal.MarkerPath, []byte(personal.MarkerJSON(user)))
				_ = m.WriteFile(vault, keyPath, []byte("encrypted-key-bytes"))
				_ = m.WriteFile(vault, pubPath, []byte("public-key-bytes"))
			},
			want: vaultStateReady,
		},
		{
			name: "conflict_no_marker_but_encrypted_key_present",
			setup: func(m *storage.MockStorage) {
				// Suspicious: no marker but key material is present.
				// Likely a name collision or tampering; refuse rather
				// than write over existing keys.
				_ = m.CreateRepo(user, personal.SecretsRepoName, true)
				_ = m.WriteFile(vault, keyPath, []byte("encrypted-key-bytes"))
			},
			want: vaultStateConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := storage.NewMockStorage()
			tt.setup(mock)

			got, err := determineVaultState(mock, user)
			if err != nil {
				t.Fatalf("determineVaultState: %v", err)
			}
			if got != tt.want {
				t.Errorf("state = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSetupKeys_ConflictReturnsError checks that setupKeys surfaces the
// Conflict state as a clear error without prompting for a passphrase or
// touching local disk.
func TestSetupKeys_ConflictReturnsError(t *testing.T) {
	const user = "alice"
	vault := personal.VaultRepo(user)
	keyPath := personal.EncryptedPrivateKeyPath(user)

	mock := storage.NewMockStorage()
	_ = mock.CreateRepo(user, personal.SecretsRepoName, true)
	_ = mock.WriteFile(vault, keyPath, []byte("encrypted-key-bytes"))

	err := setupKeys(mock, user)
	if err == nil {
		t.Fatal("expected error for Conflict state, got nil")
	}
	if !strings.Contains(err.Error(), "is not a vaultenv vault") {
		t.Errorf("error = %q, want substring %q", err.Error(), "is not a vaultenv vault")
	}
}

// TestSetupKeys_IncompleteRecovery is the regression guard for the bug this
// branch fixes. It simulates the state a crashed first-time setup leaves
// behind (repo created but no marker and no keys), drives setupKeys through
// the recovery path, and verifies that the vault is fully initialized on
// the other side.
//
// Crucially, this test exercises the *action* side of the state machine:
// if a future refactor reintroduces the original "if !exists" gate around
// the marker write, this test will fail.
func TestSetupKeys_IncompleteRecovery(t *testing.T) {
	const user = "alice"
	vault := personal.VaultRepo(user)

	redirectKeyStorage(t)
	stubPrompter(t, "correct-horse-battery-staple")

	mock := storage.NewMockStorage()
	// Crashed first-time setup: repo exists, nothing else.
	_ = mock.CreateRepo(user, personal.SecretsRepoName, true)

	// Sanity-check the precondition.
	if got, _ := determineVaultState(mock, user); got != vaultStateIncomplete {
		t.Fatalf("precondition: state = %v, want vaultStateIncomplete", got)
	}

	if err := setupKeys(mock, user); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	// Post-state assertions: marker, encrypted key, public key all present.
	marker, _ := mock.ReadFile(vault, personal.MarkerPath)
	if string(marker) != personal.MarkerJSON(user) {
		t.Errorf("marker = %q, want %q", marker, personal.MarkerJSON(user))
	}

	encKey, _ := mock.ReadFile(vault, personal.EncryptedPrivateKeyPath(user))
	if len(encKey) == 0 {
		t.Error("encrypted private key was not uploaded")
	}

	pubKey, _ := mock.ReadFile(vault, personal.PublicKeyPath(user))
	if len(pubKey) == 0 {
		t.Error("public key was not uploaded")
	}

	// The encrypted key should actually decrypt with our stubbed passphrase,
	// which proves the upload and the key-gen pipeline are wired together.
	if _, err := crypto.DecryptPrivateKey(encKey, "correct-horse-battery-staple"); err != nil {
		t.Errorf("uploaded encrypted key does not decrypt: %v", err)
	}

	// And we should now be in the Ready state.
	if got, _ := determineVaultState(mock, user); got != vaultStateReady {
		t.Errorf("post-state = %v, want vaultStateReady", got)
	}
}

// TestSetupKeys_FreshInstall covers the happy path: no repo exists, setupKeys
// creates it, writes the marker, and uploads keys. Included for parity with
// the Incomplete recovery test so both branches of the marker-write guard
// are exercised.
func TestSetupKeys_FreshInstall(t *testing.T) {
	const user = "bob"
	vault := personal.VaultRepo(user)

	redirectKeyStorage(t)
	stubPrompter(t, "another-strong-passphrase")

	mock := storage.NewMockStorage()

	if got, _ := determineVaultState(mock, user); got != vaultStateFresh {
		t.Fatalf("precondition: state = %v, want vaultStateFresh", got)
	}

	if err := setupKeys(mock, user); err != nil {
		t.Fatalf("fresh install failed: %v", err)
	}

	// The mock's CreateRepo errors on duplicate, so success here proves we
	// called it exactly once.
	if exists, _ := mock.RepoExists(user, personal.SecretsRepoName); !exists {
		t.Error("repo was not created")
	}

	marker, _ := mock.ReadFile(vault, personal.MarkerPath)
	if string(marker) != personal.MarkerJSON(user) {
		t.Errorf("marker = %q, want %q", marker, personal.MarkerJSON(user))
	}

	if got, _ := determineVaultState(mock, user); got != vaultStateReady {
		t.Errorf("post-state = %v, want vaultStateReady", got)
	}
}

