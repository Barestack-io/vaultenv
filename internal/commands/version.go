package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("vaultenv %s\n", BuildVersion())
		fmt.Printf("commit: %s\n", BuildCommit())
		fmt.Printf("built:  %s\n", BuildDate())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.SetVersionTemplate("vaultenv {{.Version}}\n")
}
