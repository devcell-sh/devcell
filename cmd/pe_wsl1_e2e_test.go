//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	winkit "github.com/devcell-sh/go-winkit"
	winkitbuild "github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/devcell-sh/go-winkit/gosshd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPEWSL1Nix_E2E is the local integration test for the full
// build -> start -> shell path that `cell shell --os=windows` exercises.
//
// Gate: DEVCELL_E2E=1 (requires real ISOs and hardware virtualisation).
//
// It builds a PE+WSL1+Nix image via winkit.Build, boots it via winkit.Start,
// connects via gosshd, and verifies that WSL1 is running with a nix rootfs.
func TestPEWSL1Nix_E2E(t *testing.T) {
	if os.Getenv("DEVCELL_E2E") != "1" {
		t.Skip("DEVCELL_E2E=1 required: this test builds and boots a real PE+WSL1 VM")
	}

	windowsISO := os.Getenv("DEVCELL_QEMU_WINDOWS_ISO")
	if windowsISO == "" {
		t.Skip("DEVCELL_QEMU_WINDOWS_ISO not set: cannot build without a Windows ISO")
	}

	virtioISO := os.Getenv("DEVCELL_QEMU_VIRTIO_ISO")
	if virtioISO == "" {
		t.Skip("DEVCELL_QEMU_VIRTIO_ISO not set: cannot build without VirtIO drivers")
	}

	nixHome := os.Getenv("DEVCELL_NIXHOME")

	// Use a temp dir for build artifacts so we don't pollute the user's home
	resultDir := t.TempDir()
	dest := filepath.Join(resultDir, "winkit-core.qcow2")
	cacheDir := filepath.Join(resultDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// --- Build ---
	t.Log("building PE+WSL1+Nix image...")
	buildCfg := winkitbuild.Config{
		Dest:       dest,
		CacheDir:   cacheDir,
		WindowsISO: windowsISO,
		VirtIOISO:  virtioISO,
		Opts: &buildopts.BuildOpts{
			PE: true,
			WSL: &buildopts.WSLConfig{
				Image:   "nix",
				NixHome: nixHome,
			},
		},
	}
	require.NoError(t, winkit.Build(ctx, buildCfg), "winkit.Build must succeed")

	// --- Verify artifact ---
	artifact, err := winkitbuild.LoadArtifact(dest)
	require.NoError(t, err)
	if artifact != nil {
		assert.Equal(t, winkitbuild.ArtifactKindPEWSL1, artifact.Kind)
		t.Logf("artifact: kind=%s boot=%s data=%s", artifact.Kind, artifact.BootVolume, artifact.DataDisk)
	}

	// --- Start ---
	const sshPort = 22199
	t.Log("starting PE+WSL1 VM...")
	startOpts := winkit.StartOpts{
		Image:    dest,
		Name:     "devcell-e2e-pe",
		StateDir: filepath.Join(resultDir, "state"),
		SSHPort:  sshPort,
		RDPPort:  0,
		Accel:    os.Getenv("DEVCELL_QEMU_ACCEL"),
	}

	machine, err := winkit.Start(ctx, startOpts)
	require.NoError(t, err, "winkit.Start must boot the PE image")
	defer func() {
		t.Log("stopping VM...")
		machine.Stop()
	}()

	// --- Wait for gosshd ---
	t.Log("waiting for gosshd...")
	client := waitForTestGosshd(t, ctx, sshPort)
	defer client.Close()

	run := func(command string) string {
		t.Helper()
		stdout, stderr, code, runErr := client.Run(ctx, command)
		require.NoError(t, runErr, "running %q", command)
		require.Zero(t, code, "%q exited %d\nstdout: %s\nstderr: %s", command, code, stdout, stderr)
		return strings.TrimSpace(string(stdout) + string(stderr))
	}

	// --- Verify WSL1 is running ---
	t.Log("verifying WSL1...")

	// The PE guest should have WSL1 with a "winkit" distro
	listOut := run("wsl --list --quiet")
	assert.Contains(t, listOut, "winkit", "WSL must have a 'winkit' distro")

	// Running uname inside WSL1 should return Linux
	unameOut := run("wsl -d winkit -- uname -s")
	assert.Equal(t, "Linux", unameOut, "WSL1 distro must report Linux")

	// Nix should be available inside WSL1
	nixOut := run("wsl -d winkit -- nix --version")
	assert.Contains(t, nixOut, "nix", "nix must be available in the WSL1 distro")

	t.Logf("PE+WSL1+Nix verified: uname=%s, nix=%s", unameOut, nixOut)

	// If DEVCELL_E2E_TEARDOWN=false, keep the VM up for manual inspection
	if os.Getenv("DEVCELL_E2E_TEARDOWN") == "false" {
		t.Log("DEVCELL_E2E_TEARDOWN=false: VM stays up. Press Ctrl+C to stop.")
		<-ctx.Done()
	}
}

func waitForTestGosshd(t *testing.T, ctx context.Context, port uint16) *gosshd.Client {
	t.Helper()
	deadline := time.Now().Add(15 * time.Minute)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var lastErr error
	for time.Now().Before(deadline) {
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		client, err := gosshd.Dial(dialCtx, addr)
		cancel()
		if err == nil {
			return client
		}
		lastErr = err
		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err(), "context expired waiting for gosshd")
		case <-time.After(10 * time.Second):
		}
	}
	require.NoError(t, lastErr, "gosshd at %s must come up", addr)
	return nil
}
