package update

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestShouldRunPeriodicCheck(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !shouldRunPeriodicCheck(time.Time{}, t0) {
		t.Fatal("zero last should run")
	}
	if shouldRunPeriodicCheck(t0, t0.Add(23*time.Hour)) {
		t.Fatal("23h should not run")
	}
	if !shouldRunPeriodicCheck(t0, t0.Add(25*time.Hour)) {
		t.Fatal("25h should run")
	}
}

func TestRunPeriodicVersionCheckNotifiesOnce(t *testing.T) {
	name, err := ExpectedAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	payload := fmt.Sprintf(`{"tag_name":"v99.0.0","assets":[{"name":%q,"browser_download_url":"https://x/y"}]}`, name)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	t.Setenv("VAULTENV_CONFIG_DIR", t.TempDir())
	periodicCheckInterval = 0
	periodicReleasesLatestURL = srv.URL
	t.Cleanup(func() {
		periodicCheckInterval = PeriodicCheckInterval
		periodicReleasesLatestURL = ""
	})

	var buf bytes.Buffer
	ctx := context.Background()
	RunPeriodicVersionCheck(ctx, "v0.0.1", &buf)
	if buf.Len() == 0 {
		t.Fatal("expected notification for older version")
	}

	buf.Reset()
	// State was just written; restore normal interval so a second run in the same "day" does nothing.
	periodicCheckInterval = PeriodicCheckInterval
	RunPeriodicVersionCheck(ctx, "v0.0.1", &buf)
	if buf.Len() != 0 {
		t.Fatalf("expected no second notification same period, got %q", buf.String())
	}
}

func TestRunPeriodicVersionCheckSkipsWhenUpToDate(t *testing.T) {
	name, err := ExpectedAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	payload := fmt.Sprintf(`{"tag_name":"v1.0.0","assets":[{"name":%q,"browser_download_url":"https://x/y"}]}`, name)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	t.Setenv("VAULTENV_CONFIG_DIR", t.TempDir())
	periodicCheckInterval = 0
	periodicReleasesLatestURL = srv.URL
	t.Cleanup(func() {
		periodicCheckInterval = PeriodicCheckInterval
		periodicReleasesLatestURL = ""
	})

	var buf bytes.Buffer
	ctx := context.Background()
	RunPeriodicVersionCheck(ctx, "v1.0.0", &buf)
	if buf.Len() != 0 {
		t.Fatalf("unexpected notify: %s", buf.String())
	}
	st, err := loadPeriodicState(PeriodicCheckStatePath())
	if err != nil || st.LastCheck == "" {
		t.Fatalf("state should be written: %+v %v", st, err)
	}
}

func TestSavePeriodicStateCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "update-check.json")
	if err := savePeriodicState(path, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
