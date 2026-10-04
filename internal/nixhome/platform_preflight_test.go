package nixhome_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/nixhome"
)

func TestPreflightPlatformCheck_NoNix_SkipsGracefully(t *testing.T) {
	err := nixhome.PreflightPlatformCheckWithLookPath(
		context.Background(),
		"path:/nonexistent",
		"aarch64-linux",
		func(string) (string, error) { return "", errors.New("not found") },
	)
	if err != nil {
		t.Errorf("should skip when nix not in PATH, got: %v", err)
	}
}

func TestPreflightPlatformCheck_NixFailure_ReturnsActionableError(t *testing.T) {
	// Use a nonexistent flake path so nix eval fails.
	requireNixStore(t)

	err := nixhome.PreflightPlatformCheck(
		context.Background(),
		"path:/nonexistent-flake-path-for-test",
		"aarch64-linux",
	)
	if err == nil {
		t.Fatal("should fail for nonexistent flake")
	}
	if !strings.Contains(err.Error(), "platform compatibility check failed") {
		t.Errorf("error should mention 'platform compatibility check failed', got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "aarch64-linux") {
		t.Errorf("error should mention target system, got: %s", err.Error())
	}
}

func TestPreflightPlatformCheck_MissingAttribute_SkipsGracefully(t *testing.T) {
	requireNixStore(t)

	dir := t.TempDir()
	flake := `{ outputs = _: { platformStrictCheck = { }; }; }`
	if err := os.WriteFile(filepath.Join(dir, "flake.nix"), []byte(flake), 0o644); err != nil {
		t.Fatal(err)
	}
	err := nixhome.PreflightPlatformCheck(
		context.Background(),
		"path:"+dir,
		"mips64el-linux",
	)
	if err != nil {
		t.Errorf("should skip when attribute is missing, got: %v", err)
	}
}

// requireNixStore skips tests that evaluate a flake when nix is missing or
// its daemon is unreachable.
func requireNixStore(t *testing.T) {
	t.Helper()
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		t.Skip("nix not in PATH")
	}
	if out, err := exec.Command(nixBin, "store", "info").CombinedOutput(); err != nil {
		t.Skipf("nix daemon unreachable (restart nix-daemon): %s", strings.TrimSpace(string(out)))
	}
}

// A nix store/daemon failure is an environment problem, not a platform
// incompatibility, and must not suggest adding platform guards.
func TestPreflightPlatformCheck_DaemonUnreachable_ReportsStore(t *testing.T) {
	dir := t.TempDir()
	fakeNix := filepath.Join(dir, "nix")
	script := "#!/bin/sh\necho \"error: cannot connect to socket at '/nix/var/nix/daemon-socket/socket': Connection refused\" >&2\nexit 1\n"
	if err := os.WriteFile(fakeNix, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	err := nixhome.PreflightPlatformCheckWithLookPath(context.Background(), "path:"+dir, "x86_64-linux",
		func(string) (string, error) { return fakeNix, nil })
	if err == nil {
		t.Fatal("want an error when the nix daemon is unreachable")
	}
	if strings.Contains(err.Error(), "platform guard") || !strings.Contains(err.Error(), "nix daemon") {
		t.Errorf("error should blame the nix daemon, not the platform: %v", err)
	}
}
