package storage

// Provider defines the storage backend interface.
// Implementations exist per git provider (GitHub, GitLab, etc.).
type Provider interface {
	// RepoExists checks whether a repository exists and is accessible.
	RepoExists(owner, name string) (bool, error)

	// CreateRepo creates a new repository.
	CreateRepo(owner, name string, private bool) error

	// ReadFile reads a file from a repository. Returns nil, nil if not found.
	ReadFile(repo, path string) ([]byte, error)

	// WriteFile creates or updates a file in a repository.
	WriteFile(repo, path string, content []byte) error

	// HasRepoAccess checks if the authenticated user has access to a repo.
	HasRepoAccess(owner, name string) (bool, error)
}
