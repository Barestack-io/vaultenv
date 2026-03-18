package commands

import (
	"fmt"
	"os"

	"github.com/Barestack-io/vaultenv/internal/config"
	"github.com/Barestack-io/vaultenv/internal/crypto"
	"github.com/Barestack-io/vaultenv/internal/storage"
	"github.com/Barestack-io/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [environment]",
	Short: "Push .env file to the vault",
	Long: `Push an environment file to the encrypted vault.

Without arguments, pushes .env as your personal environment (only you can decrypt).
With an argument, pushes .env.<environment> as a shared environment.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPush,
}

func runPush(cmd *cobra.Command, args []string) error {
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

	privKey, err := crypto.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("loading private key: %w", err)
	}

	pubKey, err := crypto.LoadPublicKey()
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}

	var envName string
	var localFile string
	var isPersonal bool

	if len(args) == 0 {
		envName = ""
		localFile = ".env"
		isPersonal = true
	} else {
		envName = args[0]
		localFile = ".env." + envName
		isPersonal = false
	}

	plaintext, err := os.ReadFile(localFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", localFile, err)
	}

	if len(plaintext) == 0 {
		return fmt.Errorf("%s is empty", localFile)
	}

	engine := crypto.NewNaClEngine()

	if isPersonal {
		encData, envelope, err := engine.EncryptForRecipients(plaintext, map[string][32]byte{
			cfg.Username: *pubKey,
		})
		if err != nil {
			return fmt.Errorf("encrypting: %w", err)
		}
		_ = privKey

		basePath := fmt.Sprintf("%s/environments/personal/%s", localCfg.Repo, cfg.Username)
		if err := store.WriteFile(localCfg.VaultRepo, basePath+".enc", encData); err != nil {
			return fmt.Errorf("uploading encrypted file: %w", err)
		}
		envJSON, err := vault.MarshalEnvelopes(envelope)
		if err != nil {
			return fmt.Errorf("marshaling envelopes: %w", err)
		}
		if err := store.WriteFile(localCfg.VaultRepo, basePath+".json", envJSON); err != nil {
			return fmt.Errorf("uploading envelopes: %w", err)
		}

		fmt.Printf("Pushed personal .env (%d bytes)\n", len(plaintext))
	} else {
		recipients := make(map[string][32]byte)
		for username, user := range vc.ApprovedUsers {
			pub, err := crypto.DecodePublicKeyString(user.PublicKey)
			if err != nil {
				return fmt.Errorf("decoding public key for %s: %w", username, err)
			}
			recipients[username] = *pub
		}

		for dkName, dk := range vc.DeployKeys {
			envAllowed := false
			for _, e := range dk.Environments {
				if e == envName {
					envAllowed = true
					break
				}
			}
			if envAllowed {
				pub, err := crypto.DecodePublicKeyString(dk.PublicKey)
				if err != nil {
					return fmt.Errorf("decoding deploy key %s: %w", dkName, err)
				}
				recipients["dk:"+dkName] = *pub
			}
		}

		encData, envelope, err := engine.EncryptForRecipients(plaintext, recipients)
		if err != nil {
			return fmt.Errorf("encrypting: %w", err)
		}

		basePath := fmt.Sprintf("%s/environments/shared/%s", localCfg.Repo, envName)
		if err := store.WriteFile(localCfg.VaultRepo, basePath+".enc", encData); err != nil {
			return fmt.Errorf("uploading encrypted file: %w", err)
		}
		envJSON, err := vault.MarshalEnvelopes(envelope)
		if err != nil {
			return fmt.Errorf("marshaling envelopes: %w", err)
		}
		if err := store.WriteFile(localCfg.VaultRepo, basePath+".json", envJSON); err != nil {
			return fmt.Errorf("uploading envelopes: %w", err)
		}

		found := false
		for _, e := range vc.Environments {
			if e == envName {
				found = true
				break
			}
		}
		if !found {
			vc.Environments = append(vc.Environments, envName)
			if err := vault.WriteVaultConfig(store, localCfg.VaultRepo, localCfg.Repo, vc); err != nil {
				return fmt.Errorf("updating vault config: %w", err)
			}
		}

		fmt.Printf("Pushed .env.%s (%d bytes, encrypted for %d recipients)\n", envName, len(plaintext), len(recipients))
	}

	return nil
}
