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

var authorizeCmd = &cobra.Command{
	Use:   "authorize",
	Short: "Approve pending access requests (vault owner only)",
	RunE:  runAuthorize,
}

func runAuthorize(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'vaultenv login' first")
	}

	localCfg, err := config.LoadLocal()
	if err != nil {
		return fmt.Errorf("not linked. Run 'vaultenv link' first: %w", err)
	}

	store := storage.NewGitHubStorage(cfg.AccessToken)

	vc, err := vault.LoadVaultConfig(store, localCfg.VaultRepo, localCfg.Repo)
	if err != nil {
		return fmt.Errorf("loading vault config: %w", err)
	}

	if vc.Owner != cfg.Username {
		return fmt.Errorf("only the vault owner (%s) can authorize users", vc.Owner)
	}

	if len(vc.PendingRequests) == 0 {
		fmt.Println("No pending access requests.")
		return nil
	}

	fmt.Printf("Pending access requests for %s:\n\n", localCfg.Repo)
	requesters := make([]string, 0, len(vc.PendingRequests))
	for username, req := range vc.PendingRequests {
		fmt.Printf("  %s (requested %s)\n", username, req.RequestedAt)
		requesters = append(requesters, username)
	}

	fmt.Printf("\nApprove all? [y/N] Or enter comma-separated usernames: ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(answer)

	if answer == "" || strings.EqualFold(answer, "n") {
		fmt.Println("No users approved.")
		return nil
	}

	var toApprove []string
	if strings.EqualFold(answer, "y") {
		toApprove = requesters
	} else {
		for _, name := range strings.Split(answer, ",") {
			name = strings.TrimSpace(name)
			if _, ok := vc.PendingRequests[name]; ok {
				toApprove = append(toApprove, name)
			} else {
				fmt.Printf("Warning: %s not in pending requests, skipping\n", name)
			}
		}
	}

	if len(toApprove) == 0 {
		fmt.Println("No valid users to approve.")
		return nil
	}

	for _, username := range toApprove {
		req := vc.PendingRequests[username]
		vc.ApprovedUsers[username] = vault.ApprovedUser{
			PublicKey:  req.PublicKey,
			ApprovedAt: vault.Now(),
		}
		delete(vc.PendingRequests, username)
		fmt.Printf("Approved: %s\n", username)
	}

	if err := vault.WriteVaultConfig(store, localCfg.VaultRepo, localCfg.Repo, vc); err != nil {
		return fmt.Errorf("updating vault config: %w", err)
	}

	// Re-encrypt shared environments for the new recipients
	if len(vc.Environments) > 0 {
		fmt.Println("\nRe-encrypting shared environments for new users...")
		engine := crypto.NewNaClEngine()
		privKey, err := crypto.LoadPrivateKey()
		if err != nil {
			return fmt.Errorf("loading private key: %w", err)
		}

		recipients := make(map[string][32]byte)
		for username, user := range vc.ApprovedUsers {
			pub, err := crypto.DecodePublicKeyString(user.PublicKey)
			if err != nil {
				return fmt.Errorf("decoding key for %s: %w", username, err)
			}
			recipients[username] = *pub
		}
		for dkName, dk := range vc.DeployKeys {
			pub, err := crypto.DecodePublicKeyString(dk.PublicKey)
			if err != nil {
				return fmt.Errorf("decoding deploy key %s: %w", dkName, err)
			}
			for _, envName := range vc.Environments {
				for _, dkEnv := range dk.Environments {
					if dkEnv == envName {
						recipients["dk:"+dkName] = *pub
						break
					}
				}
			}
		}

		for _, envName := range vc.Environments {
			basePath := fmt.Sprintf("%s/environments/shared/%s", localCfg.Repo, envName)
			encData, err := store.ReadFile(localCfg.VaultRepo, basePath+".enc")
			if err != nil || encData == nil {
				fmt.Printf("  Skipping %s (no data)\n", envName)
				continue
			}

			envJSON, err := store.ReadFile(localCfg.VaultRepo, basePath+".json")
			if err != nil {
				fmt.Printf("  Skipping %s (no envelopes)\n", envName)
				continue
			}

			envelopes, err := vault.UnmarshalEnvelopes(envJSON)
			if err != nil {
				fmt.Printf("  Skipping %s (bad envelopes)\n", envName)
				continue
			}

			myEnv, ok := envelopes[cfg.Username]
			if !ok {
				fmt.Printf("  Skipping %s (no envelope for you)\n", envName)
				continue
			}

			plaintext, err := engine.DecryptWithEnvelope(encData, myEnv, privKey)
			if err != nil {
				fmt.Printf("  Skipping %s (decryption failed)\n", envName)
				continue
			}

			envRecipients := make(map[string][32]byte)
			for k, v := range recipients {
				if strings.HasPrefix(k, "dk:") {
					dkName := strings.TrimPrefix(k, "dk:")
					if dk, ok := vc.DeployKeys[dkName]; ok {
						for _, dkEnv := range dk.Environments {
							if dkEnv == envName {
								envRecipients[k] = v
								break
							}
						}
					}
				} else {
					envRecipients[k] = v
				}
			}

			newEnc, newEnvelopes, err := engine.EncryptForRecipients(plaintext, envRecipients)
			if err != nil {
				fmt.Printf("  Error re-encrypting %s: %v\n", envName, err)
				continue
			}

			if err := store.WriteFile(localCfg.VaultRepo, basePath+".enc", newEnc); err != nil {
				fmt.Printf("  Error uploading %s: %v\n", envName, err)
				continue
			}
			newJSON, _ := vault.MarshalEnvelopes(newEnvelopes)
			if err := store.WriteFile(localCfg.VaultRepo, basePath+".json", newJSON); err != nil {
				fmt.Printf("  Error uploading envelopes for %s: %v\n", envName, err)
				continue
			}
			fmt.Printf("  Re-encrypted %s for %d recipients\n", envName, len(envRecipients))
		}
	}

	fmt.Println("\nDone.")
	return nil
}
