package commands

import (
	"testing"

	"github.com/Barestack-io/vaultenv/internal/personal"
	"github.com/Barestack-io/vaultenv/internal/storage"
)

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
	want := "repo alice/vaultenv-secrets exists but is not a vaultenv vault"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
