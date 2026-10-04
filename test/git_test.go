// git_test.go: git commit identity inside the container, from GIT_* env vars or ~/.config/git/config.

package container_test

import (
	"strings"
	"testing"
)

// TestGit_CommitUsesEnvIdentity starts a container with explicit git env vars,
// creates a commit, and verifies git log shows the configured identity.
func TestGit_CommitUsesEnvIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	c := startContainer(t, map[string]string{
		"HOST_USER":           hostUser,
		"APP_NAME":            "test",
		"GIT_AUTHOR_NAME":     "Test Author",
		"GIT_AUTHOR_EMAIL":    "test@devcell.io",
		"GIT_COMMITTER_NAME":  "Test Committer",
		"GIT_COMMITTER_EMAIL": "committer@devcell.io",
	})

	// Init repo, make a commit
	out, code := asUser(t, c, `
		cd /tmp &&
		mkdir gittest && cd gittest &&
		git init &&
		touch file.txt &&
		git add file.txt &&
		git commit -m "test commit" &&
		git log -1 --format='%an|%ae|%cn|%ce'
	`)
	if code != 0 {
		t.Fatalf("FAIL: git commit failed (exit %d):\n%s", code, out)
	}

	// Parse last line of output (git log format)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	logLine := lines[len(lines)-1]
	parts := strings.Split(logLine, "|")
	if len(parts) != 4 {
		t.Fatalf("FAIL: unexpected git log format: %q", logLine)
	}

	if parts[0] != "Test Author" {
		t.Errorf("author name: want %q, got %q", "Test Author", parts[0])
	}
	if parts[1] != "test@devcell.io" {
		t.Errorf("author email: want %q, got %q", "test@devcell.io", parts[1])
	}
	if parts[2] != "Test Committer" {
		t.Errorf("committer name: want %q, got %q", "Test Committer", parts[2])
	}
	if parts[3] != "committer@devcell.io" {
		t.Errorf("committer email: want %q, got %q", "committer@devcell.io", parts[3])
	}
	t.Logf("PASS: git identity = %s", logLine)
}

// TestGit_CommitUsesGitConfig verifies that when no GIT_* env vars are set,
// git reads identity from ~/.config/git/config inside the container.
func TestGit_CommitUsesGitConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	c := startContainer(t, map[string]string{
		"HOST_USER": hostUser,
		"APP_NAME":  "test",
	})

	// Write a git config file, init repo, commit, check identity
	out, code := asUser(t, c, `
		mkdir -p ~/.config/git &&
		cat > ~/.config/git/config <<'GITCFG'
[user]
	name = Config User
	email = config@devcell.io
GITCFG
		cd /tmp &&
		mkdir gitcfgtest && cd gitcfgtest &&
		git init &&
		touch file.txt &&
		git add file.txt &&
		git commit -m "config commit" &&
		git log -1 --format='%an|%ae'
	`)
	if code != 0 {
		t.Fatalf("FAIL: git commit with config failed (exit %d):\n%s", code, out)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	logLine := lines[len(lines)-1]
	parts := strings.Split(logLine, "|")
	if len(parts) != 2 {
		t.Fatalf("FAIL: unexpected git log format: %q", logLine)
	}

	if parts[0] != "Config User" {
		t.Errorf("author name: want %q, got %q", "Config User", parts[0])
	}
	if parts[1] != "config@devcell.io" {
		t.Errorf("author email: want %q, got %q", "config@devcell.io", parts[1])
	}
	t.Logf("PASS: git identity from config = %s", logLine)
}
