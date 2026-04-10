package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Barestack-io/vaultenv/internal/update"
	"github.com/spf13/cobra"
)

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vaultenv to the latest release",
	Long: `Download the latest vaultenv binary from GitHub releases and replace the current executable.

Requires write permission on the directory containing vaultenv (e.g. use install.sh --user for
~/.local/bin, or run with sudo if installed to /usr/local/bin).

On Windows, replacing the running .exe may fail if the file is locked; download manually from
GitHub releases in that case.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVarP(&updateCheckOnly, "check", "n", false, "only check for updates; exit 1 if a newer release exists")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	client := update.DefaultHTTPClient()
	latest, err := update.FetchLatest(ctx, client, "")
	if err != nil {
		return err
	}

	current := BuildVersion()
	upToDate := update.IsUpToDate(current, latest.Tag)

	if updateCheckOnly {
		if upToDate {
			fmt.Printf("vaultenv %s is up to date (latest release %s)\n", current, latest.Tag)
			return nil
		}
		fmt.Printf("A newer release is available: %s (you have %s)\n", latest.Tag, current)
		cancel()
		os.Exit(1)
	}

	if upToDate {
		fmt.Printf("vaultenv %s is up to date (latest release %s)\n", current, latest.Tag)
		return nil
	}

	target, err := update.ResolveExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	fmt.Printf("Updating from %s to %s...\n", current, latest.Tag)
	fmt.Printf("Downloading %s...\n", latest.Asset)

	if err := update.ReplaceExecutable(ctx, client, target, latest.AssetURL); err != nil {
		return err
	}

	fmt.Printf("Updated to %s. Run '%s version' to confirm.\n", latest.Tag, os.Args[0])
	return nil
}
