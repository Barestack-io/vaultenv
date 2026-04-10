package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v68/github"
)

type GitHubStorage struct {
	client *github.Client
}

func NewGitHubStorage(token string) *GitHubStorage {
	client := github.NewClient(nil).WithAuthToken(token)
	return &GitHubStorage{client: client}
}

func (g *GitHubStorage) RepoExists(owner, name string) (bool, error) {
	ctx := context.Background()
	_, _, err := g.client.Repositories.Get(ctx, owner, name)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		if isUnauthorized(err) {
			return false, fmt.Errorf("GitHub token expired or revoked. Run 'vaultenv login' to re-authenticate, then try again")
		}
		return false, err
	}
	return true, nil
}

func (g *GitHubStorage) CreateRepo(owner, name string, private bool) error {
	ctx := context.Background()

	repo := &github.Repository{
		Name:        github.Ptr(name),
		Private:     github.Ptr(private),
		Description: github.Ptr("vaultenv encrypted secrets storage"),
		AutoInit:    github.Ptr(true),
	}

	// Check if we're creating for a user or an org
	authedUser, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("getting authenticated user: %w", err)
	}

	if strings.EqualFold(owner, authedUser.GetLogin()) {
		_, _, err = g.client.Repositories.Create(ctx, "", repo)
	} else {
		_, _, err = g.client.Repositories.Create(ctx, owner, repo)
	}

	if err != nil {
		return fmt.Errorf("creating repo %s/%s: %w", owner, name, err)
	}

	return nil
}

func (g *GitHubStorage) ReadFile(repo, path string) ([]byte, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	fc, _, _, err := g.client.Repositories.GetContents(ctx, owner, name, path, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		if isUnauthorized(err) {
			return nil, fmt.Errorf("GitHub token expired or revoked. Run 'vaultenv login' to re-authenticate, then try again")
		}
		return nil, fmt.Errorf("reading %s from %s: %w", path, repo, err)
	}

	if fc == nil {
		return nil, nil
	}

	// go-github v68 GetContent() silently corrupts binary files: it passes
	// GitHub's line-wrapped base64 to base64.StdEncoding which does not
	// tolerate whitespace. Use the raw download URL instead — it serves the
	// exact bytes without any encoding layer.
	if fc.DownloadURL == nil || *fc.DownloadURL == "" {
		return nil, fmt.Errorf("no download URL for %s in %s", path, repo)
	}

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, *fc.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request for %s: %w", path, err)
	}
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", path, err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", path, dlResp.StatusCode)
	}

	body, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading download body for %s: %w", path, err)
	}

	return body, nil
}

func (g *GitHubStorage) WriteFile(repo, path string, content []byte) error {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return err
	}

	ctx := context.Background()

	var sha *string
	existing, _, _, err := g.client.Repositories.GetContents(ctx, owner, name, path, nil)
	if err == nil && existing != nil {
		sha = existing.SHA
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.Ptr("vaultenv: update " + path),
		Content: content,
		SHA:     sha,
	}

	_, _, err = g.client.Repositories.CreateFile(ctx, owner, name, path, opts)
	if err != nil {
		if isUnauthorized(err) {
			return fmt.Errorf("GitHub token expired or revoked. Run 'vaultenv login' to re-authenticate, then try again")
		}
		return fmt.Errorf("writing %s to %s: %w", path, repo, err)
	}

	return nil
}

func (g *GitHubStorage) HasRepoAccess(owner, name string) (bool, error) {
	return g.RepoExists(owner, name)
}

func splitRepo(repo string) (string, string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s (expected owner/name)", repo)
	}
	return parts[0], parts[1], nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found")
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "401") || strings.Contains(errStr, "Bad credentials")
}

// WrapGitHubError returns a user-friendly error for common GitHub API failures
// (expired token, revoked token, etc.). Falls back to the original error otherwise.
func WrapGitHubError(err error) error {
	if err == nil {
		return nil
	}
	if isUnauthorized(err) {
		return fmt.Errorf("GitHub token expired or revoked. Run 'vaultenv login' to re-authenticate, then try again")
	}
	return err
}
