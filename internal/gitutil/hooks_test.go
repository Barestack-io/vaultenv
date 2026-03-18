package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPrePushHook_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := InstallPrePushHook(dir); err != nil {
		t.Fatalf("InstallPrePushHook: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-push")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook: %v", err)
	}

	if !strings.Contains(string(content), "vaultenv") {
		t.Error("hook should contain vaultenv")
	}

	info, _ := os.Stat(hookPath)
	if info.Mode()&0100 == 0 {
		t.Error("hook should be executable")
	}
}

func TestInstallPrePushHook_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	hookPath := filepath.Join(hooksDir, "pre-push")
	existing := "#!/bin/sh\necho 'existing hook'\n"
	os.WriteFile(hookPath, []byte(existing), 0755)

	if err := InstallPrePushHook(dir); err != nil {
		t.Fatalf("InstallPrePushHook: %v", err)
	}

	content, _ := os.ReadFile(hookPath)
	s := string(content)

	if !strings.Contains(s, "existing hook") {
		t.Error("should preserve existing hook content")
	}
	if !strings.Contains(s, "vaultenv") {
		t.Error("should append vaultenv hook")
	}
}

func TestInstallPrePushHook_Idempotent(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	InstallPrePushHook(dir)
	content1, _ := os.ReadFile(filepath.Join(hooksDir, "pre-push"))

	InstallPrePushHook(dir)
	content2, _ := os.ReadFile(filepath.Join(hooksDir, "pre-push"))

	if string(content1) != string(content2) {
		t.Error("second install should not modify the hook")
	}
}

func TestInstallPrePushHook_NoGitDir(t *testing.T) {
	dir := t.TempDir()
	err := InstallPrePushHook(dir)
	if err == nil {
		t.Error("expected error when .git/hooks doesn't exist")
	}
}

func TestEnsureGitignoreEntries_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")

	err := EnsureGitignoreEntries(gitignorePath, []string{".env", ".env.*", ".vaultenv"})
	if err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	s := string(content)

	for _, pattern := range []string{".env", ".env.*", ".vaultenv"} {
		if !strings.Contains(s, pattern) {
			t.Errorf("gitignore should contain %q", pattern)
		}
	}
}

func TestEnsureGitignoreEntries_AppendsMissing(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n.env\n"), 0644)

	err := EnsureGitignoreEntries(gitignorePath, []string{".env", ".env.*", ".vaultenv"})
	if err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	s := string(content)

	if !strings.Contains(s, "node_modules/") {
		t.Error("should preserve existing content")
	}
	if !strings.Contains(s, ".env.*") {
		t.Error("should add missing .env.*")
	}
	if !strings.Contains(s, ".vaultenv") {
		t.Error("should add missing .vaultenv")
	}
}

func TestEnsureGitignoreEntries_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(".env\n.env.*\n.vaultenv\n"), 0644)

	before, _ := os.ReadFile(gitignorePath)

	err := EnsureGitignoreEntries(gitignorePath, []string{".env", ".env.*", ".vaultenv"})
	if err != nil {
		t.Fatalf("EnsureGitignoreEntries: %v", err)
	}

	after, _ := os.ReadFile(gitignorePath)

	if string(before) != string(after) {
		t.Error("should not modify file when all entries already present")
	}
}
