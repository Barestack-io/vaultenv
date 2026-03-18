package commands

import (
	"fmt"
	"strings"

	"github.com/scaler/vaultenv/internal/config"
	"github.com/scaler/vaultenv/internal/crypto"
	"github.com/scaler/vaultenv/internal/storage"
	"github.com/scaler/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vaultenv status for the current repo",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		fmt.Println("Authentication: not logged in")
		return nil
	}
	fmt.Printf("Authentication: logged in as %s\n", cfg.Username)

	if crypto.HasLocalKeys() {
		fmt.Println("Encryption keys: present")
	} else {
		fmt.Println("Encryption keys: not found (run 'vaultenv login')")
	}

	localCfg, err := config.LoadLocal()
	if err != nil {
		fmt.Println("Link status: not linked (run 'vaultenv link' in a git repo)")
		return nil
	}

	fmt.Printf("Link status: linked\n")
	fmt.Printf("  Repo: %s/%s\n", localCfg.Namespace, localCfg.Repo)
	fmt.Printf("  Vault: %s\n", localCfg.VaultRepo)

	store := storage.NewGitHubStorage(cfg.AccessToken)

	vc, err := vault.LoadVaultConfig(store, localCfg.VaultRepo, localCfg.Repo)
	if err != nil {
		fmt.Printf("  Vault config: error loading (%v)\n", err)
		return nil
	}

	fmt.Printf("  Owner: %s\n", vc.Owner)

	role := "not authorized"
	if vc.Owner == cfg.Username {
		role = "owner"
	} else if _, ok := vc.ApprovedUsers[cfg.Username]; ok {
		role = "approved"
	} else if _, ok := vc.PendingRequests[cfg.Username]; ok {
		role = "pending approval"
	}
	fmt.Printf("  Your role: %s\n", role)

	if len(vc.Environments) > 0 {
		fmt.Printf("  Environments: %s\n", strings.Join(vc.Environments, ", "))
	} else {
		fmt.Println("  Environments: none")
	}

	fmt.Printf("  Approved users: %d\n", len(vc.ApprovedUsers))
	fmt.Printf("  Pending requests: %d\n", len(vc.PendingRequests))
	fmt.Printf("  Deployment keys: %d\n", len(vc.DeployKeys))

	return nil
}
