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
	"time"

	"github.com/Barestack-io/vaultenv/internal/config"
)

// PeriodicCheckInterval is the minimum time between background update checks.
const PeriodicCheckInterval = 24 * time.Hour

// periodicCheckInterval is mutable for tests.
var periodicCheckInterval = PeriodicCheckInterval

// periodicReleasesLatestURL, when non-empty, overrides the GitHub API URL for RunPeriodicVersionCheck (tests only).
var periodicReleasesLatestURL string

// PeriodicCheckStatePath returns the path to the JSON file recording the last check time.
func PeriodicCheckStatePath() string {
	return filepath.Join(config.ConfigDir(), "update-check.json")
}

type periodicCheckState struct {
	LastCheck string `json:"last_check"` // RFC3339 UTC
}

func loadPeriodicState(path string) (periodicCheckState, error) {
	var st periodicCheckState
	data, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	return st, nil
}

func savePeriodicState(path string, when time.Time) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	st := periodicCheckState{LastCheck: when.UTC().Format(time.RFC3339)}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func shouldRunPeriodicCheck(lastCheck time.Time, now time.Time) bool {
	if lastCheck.IsZero() {
		return true
	}
	return now.Sub(lastCheck) >= periodicCheckInterval
}

// RunPeriodicVersionCheck contacts GitHub at most once per PeriodicCheckInterval, and writes
// a notice to w when a newer semver release exists than currentVersion. Failures are silent.
// Always updates the last-check timestamp after a fetch attempt (success or failure) so
// offline runs do not retry every invocation.
func RunPeriodicVersionCheck(ctx context.Context, currentVersion string, w io.Writer) {
	statePath := PeriodicCheckStatePath()
	now := time.Now()

	st, err := loadPeriodicState(statePath)
	var lastCheck time.Time
	if err == nil && st.LastCheck != "" {
		lastCheck, _ = time.Parse(time.RFC3339, st.LastCheck)
	}
	if !shouldRunPeriodicCheck(lastCheck, now) {
		return
	}

	if _, err := ExpectedAssetName(runtime.GOOS, runtime.GOARCH); err != nil {
		_ = savePeriodicState(statePath, now)
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	apiURL := periodicReleasesLatestURL
	latest, err := FetchLatest(ctx, client, apiURL)
	_ = savePeriodicState(statePath, time.Now())
	if err != nil {
		return
	}
	if IsUpToDate(currentVersion, latest.Tag) {
		return
	}
	_, _ = fmt.Fprintf(w, "\nA new vaultenv release is available: %s (you have %s). Run: vaultenv update\n\n", latest.Tag, currentVersion)
}
