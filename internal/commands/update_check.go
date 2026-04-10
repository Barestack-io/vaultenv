package commands

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/Barestack-io/vaultenv/internal/update"
	"github.com/spf13/cobra"
)

var updateCheckWG sync.WaitGroup

func init() {
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if os.Getenv("VAULTENV_NO_UPDATE_CHECK") != "" {
			return
		}
		switch cmd.Name() {
		case "update", "version", "help", "completion":
			return
		}
		updateCheckWG.Add(1)
		go func() {
			defer updateCheckWG.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
			defer cancel()
			update.RunPeriodicVersionCheck(ctx, BuildVersion(), os.Stderr)
		}()
	}
}

func waitForBackgroundUpdateCheck() {
	done := make(chan struct{})
	go func() {
		updateCheckWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2500 * time.Millisecond):
	}
}
