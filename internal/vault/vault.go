package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Barestack-io/vaultenv/internal/storage"
)

const deployKeyPrefix = "vaultenv_dk_v1_"

// NewVaultConfig creates a new vault config with the given owner.
func NewVaultConfig(repo, owner, ownerPubKey string) *VaultConfig {
	return &VaultConfig{
		Version: 1,
		Repo:    repo,
		Owner:   owner,
		ApprovedUsers: map[string]ApprovedUser{
			owner: {
				PublicKey:  ownerPubKey,
				ApprovedAt: Now(),
			},
		},
		PendingRequests: make(map[string]PendingRequest),
		DeployKeys:      make(map[string]DeployKey),
		Environments:    []string{},
	}
}

// ParseVaultConfig parses vault.json bytes.
func ParseVaultConfig(data []byte) (*VaultConfig, error) {
	var vc VaultConfig
	if err := json.Unmarshal(data, &vc); err != nil {
		return nil, fmt.Errorf("parsing vault config: %w", err)
	}
	if vc.ApprovedUsers == nil {
		vc.ApprovedUsers = make(map[string]ApprovedUser)
	}
	if vc.PendingRequests == nil {
		vc.PendingRequests = make(map[string]PendingRequest)
	}
	if vc.DeployKeys == nil {
		vc.DeployKeys = make(map[string]DeployKey)
	}
	return &vc, nil
}

// LoadVaultConfig loads a vault config from the storage backend.
func LoadVaultConfig(store storage.Provider, vaultRepo, repoName string) (*VaultConfig, error) {
	path := repoName + "/vault.json"
	data, err := store.ReadFile(vaultRepo, path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("vault config not found for %s", repoName)
	}
	return ParseVaultConfig(data)
}

// WriteVaultConfig writes vault config to the storage backend.
func WriteVaultConfig(store storage.Provider, vaultRepo, repoName string, vc *VaultConfig) error {
	data, err := json.MarshalIndent(vc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling vault config: %w", err)
	}
	path := repoName + "/vault.json"
	return store.WriteFile(vaultRepo, path, data)
}

// MarshalEnvelopes serializes envelopes to JSON.
func MarshalEnvelopes(envelopes map[string]Envelope) ([]byte, error) {
	wrapper := struct {
		Envelopes map[string]Envelope `json:"envelopes"`
	}{Envelopes: envelopes}
	return json.MarshalIndent(wrapper, "", "  ")
}

// UnmarshalEnvelopes deserializes envelopes from JSON.
func UnmarshalEnvelopes(data []byte) (map[string]Envelope, error) {
	var wrapper struct {
		Envelopes map[string]Envelope `json:"envelopes"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Envelopes, nil
}

// EncodeDeployKey encodes a deploy key token to an opaque string.
func EncodeDeployKey(dk *DeployKeyToken) (string, error) {
	dk.PrivateKeyB64 = base64.StdEncoding.EncodeToString(dk.PrivateKey[:])
	data, err := json.Marshal(dk)
	if err != nil {
		return "", err
	}
	return deployKeyPrefix + base64.StdEncoding.EncodeToString(data), nil
}

// DecodeDeployKey decodes a deploy key token string.
func DecodeDeployKey(token string) (*DeployKeyToken, error) {
	if !strings.HasPrefix(token, deployKeyPrefix) {
		return nil, fmt.Errorf("invalid deployment key token format")
	}

	encoded := strings.TrimPrefix(token, deployKeyPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}

	var dk DeployKeyToken
	if err := json.Unmarshal(data, &dk); err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(dk.PrivateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decoding private key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key size in token")
	}
	copy(dk.PrivateKey[:], keyBytes)

	return &dk, nil
}

// DiscoverOrCreateVault finds an existing vault repo or creates one.
// Returns the repo name (not the full owner/name path).
func DiscoverOrCreateVault(store storage.Provider, namespace string) (string, error) {
	candidates := []string{"vaultenv-secrets", "vaultenv-vault"}

	for _, name := range candidates {
		exists, err := store.RepoExists(namespace, name)
		if err != nil {
			return "", fmt.Errorf("checking %s/%s: %w", namespace, name, err)
		}
		if exists {
			marker, err := store.ReadFile(namespace+"/"+name, ".vaultenv-repo.json")
			if err == nil && marker != nil {
				return name, nil
			}
			// Name collision: repo exists but is not a vaultenv vault
			continue
		}

		// Repo doesn't exist, try to create it
		if err := store.CreateRepo(namespace, name, true); err != nil {
			if isPermissionError(err) {
				return "", fmt.Errorf(
					"you don't have permission to create repos in %s. "+
						"Ask an org admin to run 'vaultenv init %s' to set up the vault",
					namespace, namespace)
			}
			return "", fmt.Errorf("creating %s/%s: %w", namespace, name, err)
		}

		markerJSON := fmt.Sprintf(
			`{"version":1,"type":"org","namespace":"%s","created_at":"%s","created_by":"vaultenv"}`,
			namespace, Now())
		if err := store.WriteFile(namespace+"/"+name, ".vaultenv-repo.json", []byte(markerJSON)); err != nil {
			return "", fmt.Errorf("writing vault marker: %w", err)
		}

		return name, nil
	}

	return "", fmt.Errorf(
		"could not create vault repo: both 'vaultenv-secrets' and 'vaultenv-vault' exist in %s but are not vaultenv vaults. "+
			"Please create a private repo manually and run 'vaultenv init %s'",
		namespace, namespace)
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "403") || strings.Contains(s, "Forbidden") || strings.Contains(s, "permission")
}
