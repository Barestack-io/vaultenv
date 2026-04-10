package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpectedAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
		wantErr            bool
	}{
		{"linux", "amd64", "vaultenv-linux-amd64", false},
		{"linux", "arm64", "vaultenv-linux-arm64", false},
		{"darwin", "arm64", "vaultenv-darwin-arm64", false},
		{"darwin", "amd64", "vaultenv-darwin-amd64", false},
		{"windows", "amd64", "vaultenv-windows-amd64.exe", false},
		{"windows", "arm64", "vaultenv-windows-arm64.exe", false},
		{"freebsd", "amd64", "", true},
		{"linux", "386", "", true},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%s/%s", tt.goos, tt.goarch)
		t.Run(name, func(t *testing.T) {
			got, err := ExpectedAssetName(tt.goos, tt.goarch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalVersion(t *testing.T) {
	if _, ok := CanonicalVersion("dev"); ok {
		t.Error("dev should not canonicalize")
	}
	v, ok := CanonicalVersion("0.2.0")
	if !ok || v != "v0.2.0" {
		t.Errorf("got %q %v", v, ok)
	}
	v, ok = CanonicalVersion("v1.0.0")
	if !ok || v != "v1.0.0" {
		t.Errorf("got %q %v", v, ok)
	}
}

func TestIsUpToDate(t *testing.T) {
	if !IsUpToDate("v0.2.0", "v0.2.0") {
		t.Error("same version should be up to date")
	}
	if !IsUpToDate("v0.3.0", "v0.2.0") {
		t.Error("newer local should be up to date")
	}
	if IsUpToDate("v0.1.0", "v0.2.0") {
		t.Error("older local should not be up to date")
	}
	if IsUpToDate("v0.2.0", "v0.10.0") {
		t.Error("v0.2.0 should be older than v0.10.0 (semver, not string order)")
	}
	if IsUpToDate("dev", "v0.2.0") {
		t.Error("dev should not count as up to date")
	}
}

func TestLatestFromReleaseJSON(t *testing.T) {
	const payload = `{
  "tag_name": "v0.5.0",
  "assets": [
    {"name": "vaultenv-linux-amd64", "browser_download_url": "https://example.com/linux"},
    {"name": "vaultenv-darwin-arm64", "browser_download_url": "https://example.com/mac"}
  ]
}`
	li, err := LatestFromReleaseJSON([]byte(payload), "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if li.Tag != "v0.5.0" || li.Asset != "vaultenv-linux-amd64" || li.AssetURL != "https://example.com/linux" {
		t.Fatalf("unexpected: %+v", li)
	}
	_, err = LatestFromReleaseJSON([]byte(payload), "freebsd", "amd64")
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
}

func TestFetchLatestMockServer(t *testing.T) {
	name, err := ExpectedAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	payload := fmt.Sprintf(`{"tag_name":"v0.9.0","assets":[{"name":%q,"browser_download_url":"https://dl.example/x"}]}`, name)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	ctx := context.Background()
	latest, err := FetchLatest(ctx, http.DefaultClient, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Tag != "v0.9.0" || latest.Asset != name || latest.AssetURL != "https://dl.example/x" {
		t.Fatalf("%+v", latest)
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "vaultenv")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new-binary-content"))
	}))
	defer srv.Close()

	ctx := context.Background()
	err := ReplaceExecutable(ctx, http.DefaultClient, target, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new-binary-content" {
		t.Fatalf("got %q", b)
	}
}
