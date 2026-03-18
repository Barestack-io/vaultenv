package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GlobalConfig stores auth tokens and user identity.
type GlobalConfig struct {
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
}

func configDir() string {
	if override := os.Getenv("VAULTENV_CONFIG_DIR"); override != "" {
		return override
	}
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "vaultenv")
}

func globalConfigPath() string {
	return filepath.Join(configDir(), "config.json")
}

// LoadGlobal loads the global config from ~/.config/vaultenv/config.json
func LoadGlobal() (*GlobalConfig, error) {
	data, err := os.ReadFile(globalConfigPath())
	if err != nil {
		return nil, fmt.Errorf("reading global config: %w", err)
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}

	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("no access token in config")
	}

	return &cfg, nil
}

// SaveGlobal persists the global config.
func SaveGlobal(cfg *GlobalConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(globalConfigPath(), data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
