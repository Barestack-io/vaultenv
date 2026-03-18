package storage

import (
	"context"
	"fmt"
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
		return nil, fmt.Errorf("reading %s from %s: %w", path, repo, err)
	}

	if fc == nil {
		return nil, nil
	}

	content, err := fc.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decoding content: %w", err)
	}

	return []byte(content), nil
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
