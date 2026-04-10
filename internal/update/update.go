package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// DefaultReleasesLatestURL is the GitHub API URL for the latest release.
const DefaultReleasesLatestURL = "https://api.github.com/repos/Barestack-io/vaultenv/releases/latest"

// Latest describes the newest published release asset for this platform.
type Latest struct {
	Tag      string // e.g. v0.2.0
	AssetURL string
	Asset    string // e.g. vaultenv-darwin-arm64
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ExpectedAssetName returns the release filename for goos/goarch (same as install.sh).
func ExpectedAssetName(goos, goarch string) (string, error) {
	o := strings.ToLower(goos)
	a := strings.ToLower(goarch)
	switch a {
	case "amd64":
		a = "amd64"
	case "arm64":
		a = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture for self-update: %s/%s (supported: linux, darwin, windows on amd64, arm64)", goos, goarch)
	}
	switch o {
	case "linux", "darwin":
		return fmt.Sprintf("vaultenv-%s-%s", o, a), nil
	case "windows":
		return fmt.Sprintf("vaultenv-%s-%s.exe", o, a), nil
	default:
		return "", fmt.Errorf("unsupported OS for self-update: %s (supported: linux, darwin, windows)", goos)
	}
}

// FetchLatest returns the latest release tag and download URL for this OS/arch.
func FetchLatest(ctx context.Context, client *http.Client, releasesLatestURL string) (*Latest, error) {
	if releasesLatestURL == "" {
		releasesLatestURL = DefaultReleasesLatestURL
	}
	if client == nil {
		client = http.DefaultClient
	}

	assetName, err := ExpectedAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesLatestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("GitHub API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return latestFromReleaseJSON(body, assetName)
}

// LatestFromReleaseJSON parses a GitHub release API payload and finds the asset for goos/goarch.
func LatestFromReleaseJSON(data []byte, goos, goarch string) (*Latest, error) {
	assetName, err := ExpectedAssetName(goos, goarch)
	if err != nil {
		return nil, err
	}
	return latestFromReleaseJSON(data, assetName)
}

func latestFromReleaseJSON(data []byte, assetName string) (*Latest, error) {
	var gr githubRelease
	if err := json.Unmarshal(data, &gr); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	tag := strings.TrimSpace(gr.TagName)
	if tag == "" {
		return nil, fmt.Errorf("release has empty tag_name")
	}
	for _, a := range gr.Assets {
		if a.Name == assetName && a.BrowserDownloadURL != "" {
			return &Latest{Tag: tag, AssetURL: a.BrowserDownloadURL, Asset: assetName}, nil
		}
	}
	return nil, fmt.Errorf("no asset %q in release %s", assetName, tag)
}

// ResolveExecutablePath returns the absolute path to the current binary (symlinks resolved).
func ResolveExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// CanonicalVersion returns semver canonical form with v prefix, or false if not a valid release version.
func CanonicalVersion(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "dev" {
		return "", false
	}
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	if semver.IsValid(s) {
		return s, true
	}
	return "", false
}

// IsUpToDate reports whether currentVersion is the same or newer than releaseTag (both semver).
// If currentVersion is not valid semver (e.g. dev), returns false so callers may still offer an update.
func IsUpToDate(currentVersion, releaseTag string) bool {
	cur, ok := CanonicalVersion(currentVersion)
	if !ok {
		return false
	}
	rel, ok := CanonicalVersion(releaseTag)
	if !ok {
		return false
	}
	return semver.Compare(cur, rel) >= 0
}

// ReplaceExecutable downloads the release binary and replaces targetPath.
func ReplaceExecutable(ctx context.Context, client *http.Client, targetPath, downloadURL string) error {
	if client == nil {
		client = http.DefaultClient
	}

	dir := filepath.Dir(targetPath)
	if !dirWritable(dir) {
		return fmt.Errorf("cannot write to %q: permission denied (try sudo for %s, or install to ~/.local/bin via install.sh --user)", dir, targetPath)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("download failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	tmp, err := os.CreateTemp(dir, "vaultenv-update-*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("write download: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return fmt.Errorf("chmod temp binary: %w", err)
		}
	}

	// Replace target (Unix allows replacing a running binary).
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("install binary: %w\n\nOn Windows the running executable may be locked; download the latest .exe from https://github.com/Barestack-io/vaultenv/releases/latest or re-run install.sh", err)
	}
	cleanup = false
	return nil
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".vaultenv-writetest-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true
}

// DefaultHTTPClient returns an HTTP client with timeouts suitable for update checks.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute}
}
