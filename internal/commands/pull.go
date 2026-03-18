package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/scaler/vaultenv/internal/config"
	"github.com/scaler/vaultenv/internal/crypto"
	"github.com/scaler/vaultenv/internal/storage"
	"github.com/scaler/vaultenv/internal/vault"
	"github.com/spf13/cobra"
)

var (
	pullExport    bool
	pullFormat    string
	pullDeployKey string
)

var pullCmd = &cobra.Command{
	Use:   "pull [environment]",
	Short: "Pull .env file from the vault",
	Long: `Pull and decrypt an environment file from the vault.

Without arguments, pulls your personal .env.
With an argument, pulls .env.<environment>.

In CI/CD mode (VAULTENV_DEPLOY_KEY env var set), operates non-interactively.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullExport, "export", false, "Output as 'export KEY=VALUE' lines to stdout")
	pullCmd.Flags().StringVar(&pullFormat, "format", "", "Output format: 'github-env' writes to $GITHUB_ENV")
	pullCmd.Flags().StringVar(&pullDeployKey, "deploy-key", "", "Deployment key token (overrides VAULTENV_DEPLOY_KEY)")
}

func runPull(cmd *cobra.Command, args []string) error {
	deployToken := pullDeployKey
	if deployToken == "" {
		deployToken = os.Getenv("VAULTENV_DEPLOY_KEY")
	}

	if deployToken != "" {
		return runPullCICD(args, deployToken)
	}

	return runPullInteractive(args)
}

func runPullInteractive(args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'vaultenv login' first")
	}

	localCfg, err := config.LoadLocal()
	if err != nil {
		return fmt.Errorf("not linked. Run 'vaultenv link' first: %w", err)
	}

	store := storage.NewGitHubStorage(cfg.AccessToken)
	privKey, err := crypto.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("loading private key: %w", err)
	}

	engine := crypto.NewNaClEngine()

	var basePath, localFile, recipientKey string
	if len(args) == 0 {
		basePath = fmt.Sprintf("%s/environments/personal/%s", localCfg.Repo, cfg.Username)
		localFile = ".env"
		recipientKey = cfg.Username
	} else {
		envName := args[0]
		basePath = fmt.Sprintf("%s/environments/shared/%s", localCfg.Repo, envName)
		localFile = ".env." + envName
		recipientKey = cfg.Username
	}

	encData, err := store.ReadFile(localCfg.VaultRepo, basePath+".enc")
	if err != nil {
		return fmt.Errorf("downloading encrypted file: %w", err)
	}
	if encData == nil {
		return fmt.Errorf("no encrypted file found at %s.enc", basePath)
	}

	envJSON, err := store.ReadFile(localCfg.VaultRepo, basePath+".json")
	if err != nil {
		return fmt.Errorf("downloading envelopes: %w", err)
	}

	envelopes, err := vault.UnmarshalEnvelopes(envJSON)
	if err != nil {
		return fmt.Errorf("parsing envelopes: %w", err)
	}

	env, ok := envelopes[recipientKey]
	if !ok {
		return fmt.Errorf("no envelope found for user %s", recipientKey)
	}

	plaintext, err := engine.DecryptWithEnvelope(encData, env, privKey)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	return outputResult(plaintext, localFile)
}

func runPullCICD(args []string, deployToken string) error {
	dk, err := vault.DecodeDeployKey(deployToken)
	if err != nil {
		return fmt.Errorf("invalid deployment key: %w", err)
	}

	ghToken := os.Getenv("VAULTENV_GITHUB_TOKEN")
	if ghToken == "" {
		ghToken = os.Getenv("GITHUB_TOKEN")
	}
	if ghToken == "" {
		return fmt.Errorf("GITHUB_TOKEN or VAULTENV_GITHUB_TOKEN env var required for vault repo access")
	}

	store := storage.NewGitHubStorage(ghToken)
	engine := crypto.NewNaClEngine()

	if len(args) == 0 {
		return fmt.Errorf("environment name required in CI/CD mode (e.g., 'vaultenv pull staging')")
	}

	envName := args[0]

	allowed := false
	for _, e := range dk.Environments {
		if e == envName {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("deployment key %q is not authorized for environment %q (allowed: %s)",
			dk.KeyName, envName, strings.Join(dk.Environments, ", "))
	}

	basePath := fmt.Sprintf("%s/environments/shared/%s", dk.SourceRepo, envName)
	localFile := ".env." + envName

	encData, err := store.ReadFile(dk.VaultRepo, basePath+".enc")
	if err != nil {
		return fmt.Errorf("downloading encrypted file: %w", err)
	}
	if encData == nil {
		return fmt.Errorf("no encrypted file found for environment %q", envName)
	}

	envJSON, err := store.ReadFile(dk.VaultRepo, basePath+".json")
	if err != nil {
		return fmt.Errorf("downloading envelopes: %w", err)
	}

	envelopes, err := vault.UnmarshalEnvelopes(envJSON)
	if err != nil {
		return fmt.Errorf("parsing envelopes: %w", err)
	}

	recipientKey := "dk:" + dk.KeyName
	env, ok := envelopes[recipientKey]
	if !ok {
		return fmt.Errorf("no envelope found for deployment key %q (key may have been revoked)", dk.KeyName)
	}

	privKey := dk.PrivateKey
	plaintext, err := engine.DecryptWithEnvelope(encData, env, &privKey)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	return outputResult(plaintext, localFile)
}

func outputResult(plaintext []byte, defaultFile string) error {
	if pullExport {
		lines := strings.Split(strings.TrimSpace(string(plaintext)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fmt.Printf("export %s\n", line)
		}
		return nil
	}

	if pullFormat == "github-env" {
		ghEnvFile := os.Getenv("GITHUB_ENV")
		if ghEnvFile == "" {
			return fmt.Errorf("$GITHUB_ENV not set; are you running in GitHub Actions?")
		}
		f, err := os.OpenFile(ghEnvFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("opening $GITHUB_ENV: %w", err)
		}
		defer f.Close()
		lines := strings.Split(strings.TrimSpace(string(plaintext)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fmt.Fprintln(f, line)
		}
		fmt.Println("Environment variables written to $GITHUB_ENV")
		return nil
	}

	if err := os.WriteFile(defaultFile, plaintext, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", defaultFile, err)
	}
	fmt.Printf("Pulled %s (%d bytes)\n", defaultFile, len(plaintext))
	return nil
}
