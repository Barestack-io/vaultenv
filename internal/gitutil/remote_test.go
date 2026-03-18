package gitutil

import "testing"

func TestParseRemote(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		wantNS    string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:     "HTTPS with .git",
			remote:   "https://github.com/owner/repo.git",
			wantNS:   "owner",
			wantRepo: "repo",
		},
		{
			name:     "HTTPS without .git",
			remote:   "https://github.com/owner/repo",
			wantNS:   "owner",
			wantRepo: "repo",
		},
		{
			name:     "SSH with .git",
			remote:   "git@github.com:owner/repo.git",
			wantNS:   "owner",
			wantRepo: "repo",
		},
		{
			name:     "SSH without .git",
			remote:   "git@github.com:owner/repo",
			wantNS:   "owner",
			wantRepo: "repo",
		},
		{
			name:     "GitLab HTTPS",
			remote:   "https://gitlab.com/myorg/myproject.git",
			wantNS:   "myorg",
			wantRepo: "myproject",
		},
		{
			name:     "GitLab SSH",
			remote:   "git@gitlab.com:myorg/myproject.git",
			wantNS:   "myorg",
			wantRepo: "myproject",
		},
		{
			name:     "hyphenated names",
			remote:   "https://github.com/my-org/my-repo.git",
			wantNS:   "my-org",
			wantRepo: "my-repo",
		},
		{
			name:    "invalid URL",
			remote:  "not-a-url",
			wantErr: true,
		},
		{
			name:    "empty string",
			remote:  "",
			wantErr: true,
		},
		{
			name:    "just a host",
			remote:  "https://github.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, repo, err := ParseRemote(tt.remote)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for remote %q", tt.remote)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ns != tt.wantNS {
				t.Errorf("namespace: got %q, want %q", ns, tt.wantNS)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}
