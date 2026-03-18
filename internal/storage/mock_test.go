package storage

import (
	"bytes"
	"testing"
)

func TestMockCreateRepoThenExists(t *testing.T) {
	m := NewMockStorage()

	exists, _ := m.RepoExists("org", "repo")
	if exists {
		t.Error("repo should not exist before creation")
	}

	if err := m.CreateRepo("org", "repo", true); err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}

	exists, _ = m.RepoExists("org", "repo")
	if !exists {
		t.Error("repo should exist after creation")
	}
}

func TestMockCreateRepoDuplicate(t *testing.T) {
	m := NewMockStorage()
	m.CreateRepo("org", "repo", true)

	err := m.CreateRepo("org", "repo", true)
	if err == nil {
		t.Error("expected error when creating duplicate repo")
	}
}

func TestMockWriteReadFileRoundtrip(t *testing.T) {
	m := NewMockStorage()
	m.CreateRepo("org", "repo", true)

	content := []byte("hello world")
	if err := m.WriteFile("org/repo", "path/to/file.txt", content); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	read, err := m.ReadFile("org/repo", "path/to/file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !bytes.Equal(read, content) {
		t.Errorf("content mismatch: got %q, want %q", read, content)
	}
}

func TestMockReadFileNonexistent(t *testing.T) {
	m := NewMockStorage()

	data, err := m.ReadFile("org/repo", "missing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for nonexistent file, got %v", data)
	}
}

func TestMockReadFileExistingRepoMissingFile(t *testing.T) {
	m := NewMockStorage()
	m.CreateRepo("org", "repo", true)

	data, err := m.ReadFile("org/repo", "missing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for missing file, got %v", data)
	}
}

func TestMockHasRepoAccess(t *testing.T) {
	m := NewMockStorage()
	m.CreateRepo("org", "repo", true)

	has, _ := m.HasRepoAccess("org", "repo")
	if !has {
		t.Error("should have access to existing repo")
	}

	has, _ = m.HasRepoAccess("org", "nonexistent")
	if has {
		t.Error("should not have access to nonexistent repo")
	}
}

func TestMockWriteFileOverwrite(t *testing.T) {
	m := NewMockStorage()
	m.CreateRepo("org", "repo", true)

	m.WriteFile("org/repo", "file.txt", []byte("first"))
	m.WriteFile("org/repo", "file.txt", []byte("second"))

	data, _ := m.ReadFile("org/repo", "file.txt")
	if string(data) != "second" {
		t.Errorf("expected overwritten content, got %q", data)
	}
}

func TestMockWriteFileWithoutRepo(t *testing.T) {
	m := NewMockStorage()

	// WriteFile should work even if repo wasn't formally created
	err := m.WriteFile("org/repo", "file.txt", []byte("data"))
	if err != nil {
		t.Fatalf("WriteFile should succeed: %v", err)
	}

	data, _ := m.ReadFile("org/repo", "file.txt")
	if string(data) != "data" {
		t.Errorf("expected 'data', got %q", data)
	}
}

func TestMockDataIsolation(t *testing.T) {
	m := NewMockStorage()

	original := []byte("original")
	m.WriteFile("org/repo", "file.txt", original)

	// Mutate the original slice
	original[0] = 'X'

	data, _ := m.ReadFile("org/repo", "file.txt")
	if data[0] == 'X' {
		t.Error("mock should copy data, not store references")
	}

	// Mutate the read result
	data[0] = 'Y'

	data2, _ := m.ReadFile("org/repo", "file.txt")
	if data2[0] == 'Y' {
		t.Error("mock should return copies, not references")
	}
}
