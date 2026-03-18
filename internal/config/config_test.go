package config

import (
	"os"
	"testing"
)

func TestSaveLoadLocalRoundtrip(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg := &LocalConfig{
		Namespace: "myorg",
		Repo:      "myrepo",
		VaultRepo: "myorg/vaultenv-secrets",
	}

	if err := SaveLocal(cfg); err != nil {
		t.Fatalf("SaveLocal: %v", err)
	}

	loaded, err := LoadLocal()
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}

	if loaded.Namespace != cfg.Namespace {
		t.Errorf("Namespace: got %s, want %s", loaded.Namespace, cfg.Namespace)
	}
	if loaded.Repo != cfg.Repo {
		t.Errorf("Repo: got %s, want %s", loaded.Repo, cfg.Repo)
	}
	if loaded.VaultRepo != cfg.VaultRepo {
		t.Errorf("VaultRepo: got %s, want %s", loaded.VaultRepo, cfg.VaultRepo)
	}
}

func TestLoadLocalMissing(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, err := LoadLocal()
	if err == nil {
		t.Error("expected error when .vaultenv doesn't exist")
	}
}

func TestSaveLoadGlobalRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", tmpDir)

	cfg := &GlobalConfig{
		AccessToken: "ghp_testtoken123",
		Username:    "testuser",
	}

	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}

	if loaded.AccessToken != cfg.AccessToken {
		t.Errorf("AccessToken: got %s, want %s", loaded.AccessToken, cfg.AccessToken)
	}
	if loaded.Username != cfg.Username {
		t.Errorf("Username: got %s, want %s", loaded.Username, cfg.Username)
	}
}

func TestLoadGlobalMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", tmpDir)

	_, err := LoadGlobal()
	if err == nil {
		t.Error("expected error when config doesn't exist")
	}
}

func TestLoadGlobalEmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VAULTENV_CONFIG_DIR", tmpDir)

	cfg := &GlobalConfig{
		AccessToken: "",
		Username:    "testuser",
	}

	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	_, err := LoadGlobal()
	if err == nil {
		t.Error("expected error when token is empty")
	}
}
