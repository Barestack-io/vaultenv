package storage

import (
	"fmt"
	"sync"
)

// MockStorage is an in-memory Provider implementation for tests.
type MockStorage struct {
	mu    sync.RWMutex
	Repos map[string]bool              // "owner/name" -> exists
	Files map[string]map[string][]byte // "owner/name" -> path -> content
}

// NewMockStorage creates an empty mock storage.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		Repos: make(map[string]bool),
		Files: make(map[string]map[string][]byte),
	}
}

func (m *MockStorage) RepoExists(owner, name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Repos[owner+"/"+name], nil
}

func (m *MockStorage) CreateRepo(owner, name string, private bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := owner + "/" + name
	if m.Repos[key] {
		return fmt.Errorf("repo %s already exists", key)
	}
	m.Repos[key] = true
	m.Files[key] = make(map[string][]byte)
	return nil
}

func (m *MockStorage) ReadFile(repo, path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	files, ok := m.Files[repo]
	if !ok {
		return nil, nil
	}
	data, ok := files[path]
	if !ok {
		return nil, nil
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (m *MockStorage) WriteFile(repo, path string, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Files[repo] == nil {
		m.Files[repo] = make(map[string][]byte)
	}
	cp := make([]byte, len(content))
	copy(cp, content)
	m.Files[repo][path] = cp
	return nil
}

func (m *MockStorage) HasRepoAccess(owner, name string) (bool, error) {
	return m.RepoExists(owner, name)
}
