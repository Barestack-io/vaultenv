package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const localConfigFile = ".vaultenv"

// LocalConfig stores per-repo link configuration.
type LocalConfig struct {
	Namespace string `json:"namespace"`
	Repo      string `json:"repo"`
	VaultRepo string `json:"vault_repo"`
}

// LoadLocal loads the per-repo .vaultenv config from the current directory.
func LoadLocal() (*LocalConfig, error) {
	data, err := os.ReadFile(localConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading local config: %w", err)
	}

	var cfg LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing local config: %w", err)
	}

	return &cfg, nil
}

// SaveLocal persists the per-repo .vaultenv config.
func SaveLocal(cfg *LocalConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling local config: %w", err)
	}

	return os.WriteFile(localConfigFile, data, 0600)
}
