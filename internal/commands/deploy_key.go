package commands

import (
	"fmt"
	"strings"

	"github.com/Barestack-io/vaultenv/internal/config"
	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
	"github.com/Barestack-io/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var deployKeyEnvs string

var deployKeyCmd = &cobra.Command{
	Use:   "deploy-key",
	Short: "Manage deployment keys for CI/CD",
}

var deployKeyCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a deployment key for CI/CD",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeployKeyCreate,
}

var deployKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployment keys",
	RunE:  runDeployKeyList,
}

var deployKeyRevokeCmd = &cobra.Command{
	Use:   "revoke <name>",
	Short: "Revoke a deployment key",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeployKeyRevoke,
}

func init() {
	deployKeyCreateCmd.Flags().StringVar(&deployKeyEnvs, "environments", "", "Comma-separated list of environments (default: all)")
	deployKeyCmd.AddCommand(deployKeyCreateCmd)
	deployKeyCmd.AddCommand(deployKeyListCmd)
	deployKeyCmd.AddCommand(deployKeyRevokeCmd)
}

func runDeployKeyCreate(cmd *cobra.Command, args []string) error {
	keyName := args[0]

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

	if _, ok := vc.ApprovedUsers[cfg.Username]; !ok && vc.Owner != cfg.Username {
		return fmt.Errorf("you must be an approved user or the owner to create deployment keys")
	}

	if _, exists := vc.DeployKeys[keyName]; exists {
		return fmt.Errorf("deployment key %q already exists", keyName)
	}

	var environments []string
	if deployKeyEnvs != "" {
		for _, e := range strings.Split(deployKeyEnvs, ",") {
			environments = append(environments, strings.TrimSpace(e))
		}
	} else {
		environments = vc.Environments
	}

	if len(environments) == 0 {
		return fmt.Errorf("no environments specified and no shared environments exist")
	}

	privKey, pubKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generating deploy keypair: %w", err)
	}

	pubKeyStr := crypto.EncodePublicKeyString(pubKey)

	if vc.DeployKeys == nil {
		vc.DeployKeys = make(map[string]vault.DeployKey)
	}
	vc.DeployKeys[keyName] = vault.DeployKey{
		PublicKey:     pubKeyStr,
		Environments:  environments,
		CreatedBy:     cfg.Username,
		CreatedAt:     vault.Now(),
	}

	if err := vault.WriteVaultConfig(store, localCfg.VaultRepo, localCfg.Repo, vc); err != nil {
		return fmt.Errorf("updating vault config: %w", err)
	}

	// Wrap existing env symmetric keys for the new deploy key
	ownerPrivKey, err := crypto.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("loading your private key: %w", err)
	}
	engine := crypto.NewNaClEngine()

	for _, envName := range environments {
		basePath := fmt.Sprintf("%s/environments/shared/%s", localCfg.Repo, envName)
		envJSON, err := store.ReadFile(localCfg.VaultRepo, basePath+".json")
		if err != nil || envJSON == nil {
			continue
		}

		envelopes, err := vault.UnmarshalEnvelopes(envJSON)
		if err != nil {
			continue
		}

		myEnv, ok := envelopes[cfg.Username]
		if !ok {
			continue
		}

		symKey, err := engine.UnwrapKey(myEnv, ownerPrivKey)
		if err != nil {
			continue
		}

		dkEnvelope, err := engine.WrapKeyForRecipient(symKey, pubKey)
		if err != nil {
			continue
		}

		envelopes["dk:"+keyName] = dkEnvelope
		newJSON, _ := vault.MarshalEnvelopes(envelopes)
		_ = store.WriteFile(localCfg.VaultRepo, basePath+".json", newJSON)
	}

	token, err := vault.EncodeDeployKey(&vault.DeployKeyToken{
		VaultRepo:   localCfg.VaultRepo,
		SourceRepo:  localCfg.Repo,
		KeyName:     keyName,
		PrivateKey:  *privKey,
		Environments: environments,
	})
	if err != nil {
		return fmt.Errorf("encoding deploy key token: %w", err)
	}

	fmt.Println("Deployment key created successfully.")
	fmt.Printf("Name: %s\n", keyName)
	fmt.Printf("Environments: %s\n", strings.Join(environments, ", "))
	fmt.Println("\n--- DEPLOYMENT KEY TOKEN (store as CI secret) ---")
	fmt.Println(token)
	fmt.Println("--- END TOKEN ---")
	fmt.Println("\nThis token is shown ONCE and cannot be retrieved again.")
	fmt.Println("Set it as VAULTENV_DEPLOY_KEY in your CI/CD environment.")

	return nil
}

func runDeployKeyList(cmd *cobra.Command, args []string) error {
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

	if len(vc.DeployKeys) == 0 {
		fmt.Println("No deployment keys configured.")
		return nil
	}

	fmt.Printf("Deployment keys for %s:\n\n", localCfg.Repo)
	for name, dk := range vc.DeployKeys {
		fmt.Printf("  %-20s environments: %-30s created by: %s (%s)\n",
			name, strings.Join(dk.Environments, ","), dk.CreatedBy, dk.CreatedAt)
	}

	return nil
}

func runDeployKeyRevoke(cmd *cobra.Command, args []string) error {
	keyName := args[0]

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

	dk, exists := vc.DeployKeys[keyName]
	if !exists {
		return fmt.Errorf("deployment key %q not found", keyName)
	}

	if vc.Owner != cfg.Username && dk.CreatedBy != cfg.Username {
		return fmt.Errorf("only the vault owner or the key creator can revoke deployment keys")
	}

	affectedEnvs := dk.Environments
	delete(vc.DeployKeys, keyName)

	if err := vault.WriteVaultConfig(store, localCfg.VaultRepo, localCfg.Repo, vc); err != nil {
		return fmt.Errorf("updating vault config: %w", err)
	}

	fmt.Printf("Revoked deployment key: %s\n", keyName)
	fmt.Println("Rotating symmetric keys for affected environments...")

	ownerPrivKey, err := crypto.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("loading your private key: %w", err)
	}
	engine := crypto.NewNaClEngine()

	recipients := make(map[string][32]byte)
	for username, user := range vc.ApprovedUsers {
		pub, err := crypto.DecodePublicKeyString(user.PublicKey)
		if err != nil {
			continue
		}
		recipients[username] = *pub
	}

	for _, envName := range affectedEnvs {
		basePath := fmt.Sprintf("%s/environments/shared/%s", localCfg.Repo, envName)
		encData, err := store.ReadFile(localCfg.VaultRepo, basePath+".enc")
		if err != nil || encData == nil {
			continue
		}

		envJSON, err := store.ReadFile(localCfg.VaultRepo, basePath+".json")
		if err != nil {
			continue
		}

		envelopes, err := vault.UnmarshalEnvelopes(envJSON)
		if err != nil {
			continue
		}

		myEnv, ok := envelopes[cfg.Username]
		if !ok {
			continue
		}

		plaintext, err := engine.DecryptWithEnvelope(encData, myEnv, ownerPrivKey)
		if err != nil {
			continue
		}

		envRecipients := make(map[string][32]byte)
		for k, v := range recipients {
			envRecipients[k] = v
		}
		for dkName, dkInfo := range vc.DeployKeys {
			for _, dkEnv := range dkInfo.Environments {
				if dkEnv == envName {
					pub, err := crypto.DecodePublicKeyString(dkInfo.PublicKey)
					if err == nil {
						envRecipients["dk:"+dkName] = *pub
					}
					break
				}
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

		fmt.Printf("  Rotated keys for %s\n", envName)
	}

	fmt.Println("Done. The revoked token is now permanently useless.")
	return nil
}
