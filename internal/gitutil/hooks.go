package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const prePushHookContent = `#!/bin/sh
# vaultenv: auto-push personal .env on git push
if command -v vaultenv >/dev/null 2>&1; then
    if [ -f .vaultenv ] && [ -f .env ]; then
        echo "[vaultenv] Syncing personal .env..."
        vaultenv push 2>/dev/null || echo "[vaultenv] Warning: failed to sync .env"
    fi
fi
`

// InstallPrePushHook installs a git pre-push hook in the repo at dir.
func InstallPrePushHook(dir string) error {
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf(".git/hooks directory not found")
	}

	hookPath := filepath.Join(hooksDir, "pre-push")

	if existing, err := os.ReadFile(hookPath); err == nil {
		content := string(existing)
		if strings.Contains(content, "vaultenv") {
			return nil // already installed
		}
		// Append to existing hook
		f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("\n" + prePushHookContent)
		return err
	}

	return os.WriteFile(hookPath, []byte(prePushHookContent), 0755)
}

// EnsureGitignoreEntries ensures that the given patterns exist in .gitignore.
func EnsureGitignoreEntries(gitignorePath string, patterns []string) error {
	var existing string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, pattern := range patterns {
		if !strings.Contains(existing, pattern) {
			toAdd = append(toAdd, pattern)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if existing != "" && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	header := "\n# vaultenv\n"
	if _, err := f.WriteString(header); err != nil {
		return err
	}

	for _, pattern := range toAdd {
		if _, err := f.WriteString(pattern + "\n"); err != nil {
			return err
		}
	}

	return nil
}
