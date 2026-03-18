package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vaultenv",
	Short: "Securely share and store .env files with your team",
	Long: `vaultenv lets teams securely share environment variables using GitHub
for authentication and encrypted cloud storage. Each user's secrets are
protected with NaCl envelope encryption and can only be decrypted by
authorized team members.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(authorizeCmd)
	rootCmd.AddCommand(deployKeyCmd)
	rootCmd.AddCommand(statusCmd)
}

func exitError(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	os.Exit(1)
}
