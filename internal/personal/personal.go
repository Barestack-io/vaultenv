package personal

import "fmt"

// SecretsRepoName is the GitHub repo name for a user's personal vaultenv storage.
const SecretsRepoName = "vaultenv-secrets"

// MarkerPath is the file that marks a repo as a vaultenv personal vault.
const MarkerPath = ".vaultenv-repo.json"

// VaultRepo returns the full "owner/name" repo slug for the personal secrets repo.
func VaultRepo(username string) string {
	return username + "/" + SecretsRepoName
}

// EncryptedPrivateKeyPath is the path to the passphrase-wrapped private key in the personal repo.
func EncryptedPrivateKeyPath(username string) string {
	return fmt.Sprintf("keys/%s.key.enc", username)
}

// PublicKeyPath is the path to the base64-encoded public key in the personal repo.
func PublicKeyPath(username string) string {
	return fmt.Sprintf("keys/%s.pub", username)
}

// MarkerJSON returns the JSON marker written when creating a personal vault.
func MarkerJSON(username string) string {
	return fmt.Sprintf(`{"version":1,"type":"personal","namespace":"%s","created_by":"%s"}`, username, username)
}
