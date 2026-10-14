package container_test

// cold_run_test.go — e2e "day-one" test: scaffold a project, build on a
// fresh nix volume, start a container, and verify the result. Exercises
// nix cache hydration, thin-image build, and basic cell functionality.

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/testutil"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestColdRun_BaseStack simulates a first-time user:
//
//  1. Scaffold devcell.toml (base stack) in a temp project dir
//  2. `devcell build --debug` against a fresh nix volume (triggers cache hydration)
//  3. Start a container from the built image
//  4. Assert: nix cache was pulled, sentinel exists, shell works, nix profile populated
//
// Long test: requires Docker + network. Skipped under -short.
func TestColdRun_BaseStack(t *testing.T) {
	if testing.Short() {
		t.Skip("long: build+run against a fresh volume (~minutes); drop -short to enable")
	}

	outDir := testutil.TestResultsDir(t, hostBaseDirFn)

	// ── Fresh volume ────────────────────────────────────────────────────────
	volName := "devcell-test-coldrun-" + shortSHA()
	_ = osexec.Command("docker", "volume", "rm", "-f", volName).Run()
	t.Cleanup(func() { _ = osexec.Command("docker", "volume", "rm", "-f", volName).Run() })

	// ── Isolated home (no global config bleed) ──────────────────────────────
	fakeHome := filepath.Join(outDir, "home")
	globalDir := filepath.Join(fakeHome, ".config", "devcell")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}

	// ── Project dir with devcell.toml ───────────────────────────────────────
	localNixhome, err := filepath.Abs(nixhomeDir())
	if err != nil {
		t.Fatalf("abs nixhome: %v", err)
	}

	projectDir := filepath.Join(outDir, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "devcell.toml"), []byte(`[cell]
stack = "base"
nixhome = "`+localNixhome+`"
`), 0o644); err != nil {
		t.Fatalf("write devcell.toml: %v", err)
	}

	cellBin, err := ensureCellBinary()
	if err != nil {
		t.Fatalf("ensure cell binary: %v", err)
	}

	baseEnv := append(os.Environ(),
		"HOME="+fakeHome,
		"DEVCELL_NIX_VOLUME="+volName,
	)

	// ── Build ───────────────────────────────────────────────────────────────
	imageTag := "devcell-user:coldrun-base-" + shortSHA()
	t.Cleanup(func() { _ = osexec.Command("docker", "rmi", imageTag).Run() })

	var buildLog string
	t.Run("build", func(t *testing.T) {
		cmd := osexec.Command(cellBin, "build", "--image", imageTag, "--debug")
		cmd.Dir = projectDir
		cmd.Env = baseEnv
		out, err := cmd.CombinedOutput()
		buildLog = string(out)
		logPath := filepath.Join(outDir, "build.log")
		os.WriteFile(logPath, out, 0o644)
		if err != nil {
			t.Fatalf("devcell build failed: %v (see %s)", err, logPath)
		}
		t.Logf("build log: %s", logPath)
	})

	// ── Assert: nix cache hydration happened ────────────────────────────────
	t.Run("nix_cache_hydrated", func(t *testing.T) {
		if !strings.Contains(buildLog, "Hydrating nix cache") {
			t.Error("build log does not mention nix cache hydration — expected pull on fresh volume")
		}
		if strings.Contains(buildLog, "building from scratch") {
			t.Error("nix cache hydration failed (fell back to building from scratch)")
		}
	})

	// ── Start container ─────────────────────────────────────────────────────
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:      imageTag,
		User:       "0",
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"pgrep", "tail"}).WithStartupTimeout(120 * 1e9),
		Mounts: testcontainers.Mounts(
			testcontainers.VolumeMount(volName, "/nix"),
		),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	// ── Assert: sentinel exists ─────────────────────────────────────────────
	t.Run("sentinel", func(t *testing.T) {
		out, code := containerExec(t, c, []string{"test", "-e", "/nix/var/nix/profiles/devcell-tools/bin/tini"})
		if code != 0 {
			t.Errorf("sentinel tini not found: exit=%d out=%q", code, out)
		}
	})

	// ── Assert: nix DB has registered paths ─────────────────────────────────
	t.Run("nix_db", func(t *testing.T) {
		out, code := containerExec(t, c, []string{"bash", "-lc", "nix path-info --all 2>/dev/null | wc -l"})
		if code != 0 {
			t.Fatalf("nix path-info failed: exit=%d out=%q", code, out)
		}
		t.Logf("nix paths registered: %s", strings.TrimSpace(out))
	})

	// ── Assert: shell works ─────────────────────────────────────────────────
	t.Run("shell", func(t *testing.T) {
		out, code := asUser(t, c, "echo hello-from-cold-run")
		if code != 0 || !strings.Contains(out, "hello-from-cold-run") {
			t.Errorf("shell echo failed: exit=%d out=%q", code, out)
		}
	})

	// ── Assert: nix profile populated ───────────────────────────────────────
	t.Run("nix_profile", func(t *testing.T) {
		out, code := containerExec(t, c, []string{
			"test", "-d", "/opt/devcell/.local/state/nix/profiles/profile/bin",
		})
		if code != 0 {
			t.Errorf("nix profile bin dir missing: exit=%d out=%q", code, out)
		}
	})

	t.Logf("artifacts: %s", outDir)
}
