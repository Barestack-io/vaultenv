package commands

import (
	"fmt"

	"github.com/scaler/vaultenv/internal/config"
	"github.com/scaler/vaultenv/internal/storage"
	"github.com/scaler/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <namespace>",
	Short: "Initialize a vault repo for an organization",
	Long:  "Create the vaultenv-secrets repo in a GitHub org. Requires repo creation permissions.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	namespace := args[0]

	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'vaultenv login' first: %w", err)
	}

	store := storage.NewGitHubStorage(cfg.AccessToken)

	repoName, err := vault.DiscoverOrCreateVault(store, namespace)
	if err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	fmt.Printf("Vault initialized: %s/%s\n", namespace, repoName)
	return nil
}
