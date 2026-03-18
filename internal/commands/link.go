package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/scaler/vaultenv/internal/config"
	"github.com/scaler/vaultenv/internal/crypto"
	"github.com/scaler/vaultenv/internal/gitutil"
	"github.com/scaler/vaultenv/internal/storage"
	"github.com/scaler/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link the current git repo to vaultenv",
	RunE:  runLink,
}

func runLink(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		fmt.Println("Not logged in. Starting login flow...")
		if err := runLogin(cmd, nil); err != nil {
			return err
		}
		cfg, err = config.LoadGlobal()
		if err != nil {
			return fmt.Errorf("login succeeded but failed to load config: %w", err)
		}
	}

	if !crypto.HasLocalKeys() {
		return fmt.Errorf("encryption keys not found. Run 'vaultenv login' first")
	}

	remote, err := gitutil.GetRemoteURL(".")
	if err != nil {
		return fmt.Errorf("failed to detect git remote: %w", err)
	}

	namespace, repoName, err := gitutil.ParseRemote(remote)
	if err != nil {
		return fmt.Errorf("failed to parse remote URL: %w", err)
	}

	fmt.Printf("Detected repo: %s/%s\n", namespace, repoName)

	store := storage.NewGitHubStorage(cfg.AccessToken)

	hasAccess, err := store.HasRepoAccess(namespace, repoName)
	if err != nil {
		return fmt.Errorf("checking repo access: %w", err)
	}
	if !hasAccess {
		return fmt.Errorf("you don't have access to %s/%s on GitHub", namespace, repoName)
	}

	vaultRepoName, err := vault.DiscoverOrCreateVault(store, namespace)
	if err != nil {
		return fmt.Errorf("vault setup failed: %w", err)
	}
	vaultRepo := namespace + "/" + vaultRepoName

	vaultCfgPath := repoName + "/vault.json"
	vaultData, err := store.ReadFile(vaultRepo, vaultCfgPath)
	if err != nil {
		return fmt.Errorf("reading vault config: %w", err)
	}

	pubKey, err := crypto.LoadPublicKey()
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}
	pubKeyStr := crypto.EncodePublicKeyString(pubKey)

	if vaultData == nil {
		fmt.Printf("No vault config found for %s. Initializing you as the owner.\n", repoName)
		vc := vault.NewVaultConfig(namespace+"/"+repoName, cfg.Username, pubKeyStr)
		if err := vault.WriteVaultConfig(store, vaultRepo, repoName, vc); err != nil {
			return fmt.Errorf("writing vault config: %w", err)
		}
		fmt.Println("Vault configuration created.")
	} else {
		vc, err := vault.ParseVaultConfig(vaultData)
		if err != nil {
			return fmt.Errorf("parsing vault config: %w", err)
		}

		if _, ok := vc.ApprovedUsers[cfg.Username]; ok {
			fmt.Println("You are an approved user for this vault.")
			if len(vc.Environments) > 0 {
				fmt.Println("Available environments:")
				for _, env := range vc.Environments {
					fmt.Printf("  - %s\n", env)
				}
				fmt.Println("Use 'vaultenv pull <environment>' to download.")
			}
		} else if vc.Owner == cfg.Username {
			fmt.Println("You are the owner of this vault.")
		} else if _, ok := vc.PendingRequests[cfg.Username]; ok {
			fmt.Println("Your access request is pending approval by the vault owner.")
		} else {
			fmt.Printf("You are not authorized for this vault. Requesting access from owner (%s)...\n", vc.Owner)
			vc.PendingRequests[cfg.Username] = vault.PendingRequest{
				PublicKey:   pubKeyStr,
				RequestedAt: vault.Now(),
			}
			if err := vault.WriteVaultConfig(store, vaultRepo, repoName, vc); err != nil {
				return fmt.Errorf("submitting access request: %w", err)
			}
			fmt.Println("Access request submitted. The vault owner can approve it with 'vaultenv authorize'.")
		}
	}

	localCfg := &config.LocalConfig{
		Namespace: namespace,
		Repo:      repoName,
		VaultRepo: vaultRepo,
	}
	if err := config.SaveLocal(localCfg); err != nil {
		return fmt.Errorf("saving local config: %w", err)
	}

	gitignorePath := filepath.Join(".", ".gitignore")
	if err := gitutil.EnsureGitignoreEntries(gitignorePath, []string{".env", ".env.*", ".vaultenv"}); err != nil {
		fmt.Printf("Warning: could not update .gitignore: %v\n", err)
	}

	fmt.Print("\nWould you like to install a git hook to auto-push your personal .env on git push? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if strings.EqualFold(strings.TrimSpace(answer), "y") {
		if err := gitutil.InstallPrePushHook("."); err != nil {
			fmt.Printf("Warning: failed to install hook: %v\n", err)
		} else {
			fmt.Println("Pre-push hook installed.")
		}
	}

	fmt.Println("\nLinked! Use 'vaultenv push' and 'vaultenv pull' to sync .env files.")
	return nil
}
