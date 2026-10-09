//go:build darwin || linux

package winkit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/devcell-sh/go-winkit/gosshd"
	"github.com/devcell-sh/go-winkit/unattend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
)

func TestGuestFor(t *testing.T) {
	for in, want := range map[engine.Guest]guest{
		"":                 guestFull,
		engine.WindowsPE:   guestPE,
		engine.WindowsFull: guestFull,
	} {
		got, err := guestFor(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

func TestGuestFor_UnknownGuestErrors(t *testing.T) {
	for _, g := range []engine.Guest{engine.Linux, engine.MacOS, "windows", "freebsd"} {
		_, err := guestFor(g)
		require.Error(t, err, g)
		assert.Contains(t, err.Error(), string(g))
	}
}

func TestEngine_IsRegisteredForWinkit(t *testing.T) {
	e, err := engine.For(engine.Winkit)
	require.NoError(t, err)
	assert.Equal(t, Engine{}, e)
}

// Every verb rejects a guest winkit cannot run before touching the host.
func TestEngine_UnknownGuestErrors(t *testing.T) {
	c := engine.Cell{Name: "main", HostHome: t.TempDir(), Stack: "base", Guest: engine.MacOS}
	ctx := context.Background()

	for verb, err := range map[string]error{
		"Init":  Engine{}.Init(ctx, engine.InitOpts{Cell: c}),
		"Build": Engine{}.Build(ctx, engine.BuildOpts{Cell: c, DryRun: true}),
		"Run":   Engine{}.Run(ctx, engine.RunOpts{Cell: c, Binary: "claude", DryRun: true}),
	} {
		require.Error(t, err, verb)
		assert.Contains(t, err.Error(), "macos", verb)
	}
	assert.NoDirExists(t, filepath.Join(c.HostHome, ".devcell"), "a rejected guest must not create anything")
}

// --- WindowsFull: winkit stage full-wsl ---

func TestBuildConfig_FullIsNotPE(t *testing.T) {
	c := buildConfig(peCell("/home/u"), guestFull, "/fake/w.iso", "/fake/v.iso", nil)

	require.NotNil(t, c.Opts)
	assert.False(t, c.Opts.PE, "the full guest is a full Windows install, not a PE boot volume")
	require.NotNil(t, c.Opts.WSL)
	assert.Equal(t, "nix", c.Opts.WSL.Image)
	assert.Equal(t, buildopts.StageFullWSL, c.Opts.Stage())
}

func TestBuildConfig_FullNixHomeFromEnv(t *testing.T) {
	t.Setenv("DEVCELL_NIXHOME", "/path/to/nixhome")
	c := buildConfig(peCell("/home/u"), guestFull, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Equal(t, "/path/to/nixhome", c.Opts.WSL.NixHome)
}

func TestArtifactPath_FullNamedAfterStage(t *testing.T) {
	assert.Equal(t, "winkit-full-wsl.qcow2", filepath.Base(artifactPath("/home/u", guestFull, "base", nil)))
}

// A PE and a Full template for the same stack and modules must never share
// a directory: winkit keeps the data disk, manifest, vars and run state
// next to the image.
func TestTemplateDir_DistinctPerGuest(t *testing.T) {
	for _, mods := range [][]string{nil, {"docker", "node"}} {
		pe := templateDir("/home/u", guestPE, "base", mods)
		full := templateDir("/home/u", guestFull, "base", mods)

		assert.NotEqual(t, pe, full, "modules %v", mods)
		assert.False(t, strings.HasPrefix(pe, full+string(filepath.Separator)), "PE template inside the Full one: %s", pe)
		assert.False(t, strings.HasPrefix(full, pe+string(filepath.Separator)), "Full template inside the PE one: %s", full)
	}
}

// Templates stay under ~/.devcell/windows/, which DiscoverRunningVMs knows
// is not a cell.
func TestTemplateDir_FullStaysUnderTemplateRoot(t *testing.T) {
	assert.Equal(t, "/home/u/.devcell/windows/full-wsl/base", templateDir("/home/u", guestFull, "base", nil))
}

// The legacy winkit-core.qcow2 name only ever held PE boot volumes.
func TestImagePath_FullNeverFallsBackToLegacyName(t *testing.T) {
	home := t.TempDir()
	dir := templateDir(home, guestFull, "base", nil)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, legacyPEArtifactName), []byte("qcow2"), 0o644))

	assert.Equal(t, filepath.Join(dir, "winkit-full-wsl.qcow2"), imagePath(home, guestFull, "base", nil))
}

func TestStartOpts_FullBootsFullImage(t *testing.T) {
	opts := startOpts(cellWith("main", "/home/u", cfg.CellSection{}), guestFull, 20022, 23389, 10150)

	assert.Equal(t, imagePath("/home/u", guestFull, "base", nil), opts.Image)
	assert.Equal(t, InstanceDir("/home/u", "main"), opts.StateDir)
}

func TestBuild_FullGuestBuildsFullWSLStage(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var got build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		got = c
		// Create a sparse file >4 GB so the disk sanity check passes.
		require.NoError(t, os.MkdirAll(filepath.Dir(c.Dest), 0o755))
		f, fErr := os.Create(c.Dest)
		require.NoError(t, fErr)
		require.NoError(t, f.Truncate(5*1024*1024*1024))
		require.NoError(t, f.Close())
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: engine.Cell{
		Name: "main", HostHome: home, Stack: "base", Guest: engine.WindowsFull,
	}})
	require.NoError(t, err)

	assert.False(t, got.Opts.PE)
	assert.Equal(t, buildopts.StageFullWSL, got.Opts.Stage())
	assert.Equal(t, filepath.Join(home, ".devcell", "windows", "full-wsl", "base", "winkit-full-wsl.qcow2"), got.Dest)
	assert.DirExists(t, filepath.Dir(got.Dest))
}

// --- the command channel ---

// Both guests import the distro under the same name, so the agent command
// is the same wsl invocation. Full Windows runs it under the Windows
// OpenSSH default shell, PowerShell, where --% hands the rest of the line
// to wsl.exe untouched, as cmd.exe does on PE.
func TestGuestCommand_Full(t *testing.T) {
	assert.Equal(t, "wsl -d winkit", guestCommand(guestFull, nil, nil))
	assert.Equal(t,
		`wsl --% -d winkit -- env 'GIT_AUTHOR_NAME=Ada Lovelace' claude 'fix it'\''s bug'`,
		guestCommand(guestFull, []string{"GIT_AUTHOR_NAME=Ada Lovelace"}, []string{"claude", "fix it's bug"}))
}

func TestChannel_PERunsOverGosshd(t *testing.T) {
	assert.Equal(t, sshChannel{port: 20022, user: gosshd.DefaultUser, password: gosshd.DefaultPassword}, guestPE.channel(20022))
}

// WSL registrations are per user: gosshd runs as SYSTEM and cannot see the
// distro the full install registered for its Windows user.
func TestChannel_FullRunsOverWindowsOpenSSHAsTheUser(t *testing.T) {
	u := unattend.DefaultConfig()

	assert.Equal(t, sshChannel{port: 20122, user: u.Username, password: u.Password}, guestFull.channel(20022))
}

func TestRun_DryRunFullGuest(t *testing.T) {
	home := t.TempDir()
	out := captureStdout(t, func() {
		require.NoError(t, Engine{}.Run(context.Background(), engine.RunOpts{
			Cell:   engine.Cell{Name: "main", HostHome: home, BaseDir: home, Stack: "base", Guest: engine.WindowsFull},
			Binary: "claude",
			DryRun: true,
		}))
	})

	assert.Contains(t, out, "Windows+WSL1 runner (dry-run)")
	assert.Contains(t, out, filepath.Join("full-wsl", "base", "winkit-full-wsl.qcow2"))
	assert.Contains(t, out, "wsl --% -d winkit -- ")
}
