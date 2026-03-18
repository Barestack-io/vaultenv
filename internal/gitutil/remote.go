package gitutil

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// Matches HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	httpsPattern = regexp.MustCompile(`https?://[^/]+/([^/]+)/([^/.]+)(?:\.git)?$`)
	// Matches SSH: git@github.com:owner/repo.git
	sshPattern = regexp.MustCompile(`git@[^:]+:([^/]+)/([^/.]+)(?:\.git)?$`)
)

// GetRemoteURL returns the URL of the 'origin' remote for the git repo at dir.
func GetRemoteURL(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting git remote: %w (is this a git repository?)", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ParseRemote extracts the namespace (owner/org) and repo name from a git remote URL.
func ParseRemote(remote string) (namespace, repo string, err error) {
	if m := httpsPattern.FindStringSubmatch(remote); m != nil {
		return m[1], m[2], nil
	}
	if m := sshPattern.FindStringSubmatch(remote); m != nil {
		return m[1], m[2], nil
	}
	return "", "", fmt.Errorf("unrecognized remote URL format: %s", remote)
}
