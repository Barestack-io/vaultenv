package vault

import "time"

// VaultConfig represents the vault.json for a linked repo.
type VaultConfig struct {
	Version         int                       `json:"version"`
	Repo            string                    `json:"repo"`
	Owner           string                    `json:"owner"`
	ApprovedUsers   map[string]ApprovedUser   `json:"approved_users"`
	PendingRequests map[string]PendingRequest `json:"pending_requests"`
	DeployKeys      map[string]DeployKey      `json:"deploy_keys,omitempty"`
	Environments    []string                  `json:"environments"`
}

// ApprovedUser is a user who has been granted access.
type ApprovedUser struct {
	PublicKey  string `json:"public_key"`
	ApprovedAt string `json:"approved_at"`
}

// PendingRequest is an unapproved access request.
type PendingRequest struct {
	PublicKey   string `json:"public_key"`
	RequestedAt string `json:"requested_at"`
}

// DeployKey is a CI/CD deployment key entry.
type DeployKey struct {
	PublicKey    string   `json:"public_key"`
	Environments []string `json:"environments"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
}

// Envelope holds the encrypted symmetric key for one recipient.
type Envelope struct {
	EncryptedKey    string `json:"encrypted_key"`
	Nonce           string `json:"nonce"`
	EphemeralPublic string `json:"ephemeral_public"`
}

// DeployKeyToken is the decoded form of a deployment key token.
type DeployKeyToken struct {
	VaultRepo    string   `json:"vault_repo"`
	SourceRepo   string   `json:"source_repo"`
	KeyName      string   `json:"key_name"`
	PrivateKey   [32]byte `json:"-"`
	PrivateKeyB64 string  `json:"private_key"`
	Environments []string `json:"environments"`
}

// RepoMarker identifies a repo as a vaultenv vault.
type RepoMarker struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
}

// Now returns the current time as an RFC3339 string.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
