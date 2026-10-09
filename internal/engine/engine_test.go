package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/engine"
)

// resolve calls engine.Resolve and requires it to succeed without a
// deprecation warning.
func resolve(t *testing.T, flagEngine, flagOS, tomlEngine, tomlOS string) engine.Name {
	t.Helper()
	got, deprecation, err := engine.Resolve(flagEngine, flagOS, tomlEngine, tomlOS)
	require.NoError(t, err)
	assert.Empty(t, deprecation)
	return got
}

// resolveErr calls engine.Resolve and requires it to fail.
func resolveErr(t *testing.T, flagEngine, flagOS, tomlEngine, tomlOS string) error {
	t.Helper()
	_, _, err := engine.Resolve(flagEngine, flagOS, tomlEngine, tomlOS)
	require.Error(t, err)
	return err
}

func TestResolveEngine_FlagWins(t *testing.T) {
	assert.Equal(t, engine.Tart, resolve(t, "tart", "", "docker", ""))
}

func TestResolveEngine_TOMLFallback(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "", "winkit", ""))
}

func TestResolveEngine_DefaultDocker(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "", "", "", ""))
}

func TestResolveEngine_ExplicitDocker(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "docker", "", "", ""))
}

// --- --os flag tests (CELL-491 2d) ---

func TestResolveEngine_OSLinux(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "", "linux", "", ""))
}

func TestResolveEngine_OSWindows(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "windows", "", ""))
}

func TestResolveEngine_OSWinPE(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "winpe", "", ""))
}

func TestResolveEngine_OSMacos(t *testing.T) {
	assert.Equal(t, engine.Tart, resolve(t, "", "macos", "", ""))
}

func TestResolveEngine_EngineFlagWithCompatibleOS(t *testing.T) {
	assert.Equal(t, engine.Tart, resolve(t, "tart", "macos", "", ""))
	assert.Equal(t, engine.Winkit, resolve(t, "winkit", "winpe", "", ""))
}

func TestResolveEngine_OSOverridesTomlEngine(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "windows", "docker", ""))
}

func TestResolveEngine_TomlOSFallback(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "", "", "windows"))
	assert.Equal(t, engine.Winkit, resolve(t, "", "", "", "winpe"))
	assert.Equal(t, engine.Tart, resolve(t, "", "", "", "macos"))
}

func TestResolveEngine_TomlOSOverridesTomlEngine(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "", "", "docker", "linux"), "[cell] os linux maps to docker, which matches [cell] engine anyway")
}

func TestResolveEngine_Precedence_EngineFlagFirst(t *testing.T) {
	assert.Equal(t, engine.Tart, resolve(t, "tart", "macos", "winkit", "linux"))
}

func TestResolveEngine_Precedence_OSFlagSecond(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "windows", "docker", "linux"))
}

func TestResolveEngine_Precedence_TomlEngineThird(t *testing.T) {
	assert.Equal(t, engine.Tart, resolve(t, "", "", "tart", ""))
}

func TestResolveEngine_Precedence_TomlOSFourth(t *testing.T) {
	assert.Equal(t, engine.Winkit, resolve(t, "", "", "", "windows"))
}

func TestResolveEngine_ImpossibleCombos(t *testing.T) {
	for _, c := range []struct{ engine, os string }{
		{"tart", "windows"},
		{"winkit", "linux"},
		{"winkit", "macos"},
		{"docker", "winpe"},
		{"tart", "linux"}, // planned, not implemented
	} {
		err := resolveErr(t, c.engine, c.os, "", "")
		assert.Contains(t, err.Error(), "incompatible", "--engine %s --os %s", c.engine, c.os)
	}
}

func TestResolveEngine_InvalidOS(t *testing.T) {
	err := resolveErr(t, "", "freebsd", "", "")
	assert.Contains(t, err.Error(), "unsupported")
	assert.Contains(t, err.Error(), "linux, macos, windows, winpe")
}

// The full Windows guest exists in the capability table but has no CLI
// spelling yet.
func TestResolveEngine_WindowsFullNotReachableFromCLI(t *testing.T) {
	err := resolveErr(t, "", string(engine.WindowsFull), "", "")
	assert.Contains(t, err.Error(), "unsupported --os value")
}

// --- deprecated engine alias: qemu is now winkit ---

const qemuDeprecation = `engine "qemu" is deprecated and will be removed in a future release: use engine = "winkit" instead`

func TestResolveEngine_QemuFlagIsDeprecatedAliasForWinkit(t *testing.T) {
	got, deprecation, err := engine.Resolve("qemu", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, engine.Winkit, got)
	assert.Equal(t, qemuDeprecation, deprecation)
}

func TestResolveEngine_QemuTOMLIsDeprecatedAliasForWinkit(t *testing.T) {
	got, deprecation, err := engine.Resolve("", "", "qemu", "")
	require.NoError(t, err)
	assert.Equal(t, engine.Winkit, got)
	assert.Equal(t, qemuDeprecation, deprecation)
}

func TestResolveEngine_QemuAliasWithOS(t *testing.T) {
	got, deprecation, err := engine.Resolve("qemu", "windows", "", "")
	require.NoError(t, err)
	assert.Equal(t, engine.Winkit, got)
	assert.Equal(t, qemuDeprecation, deprecation)

	err = resolveErr(t, "qemu", "linux", "", "")
	assert.Contains(t, err.Error(), "incompatible")
}

// A [cell] engine value that a flag overrides is never read, so it must
// not warn.
func TestResolveEngine_OverriddenQemuTOMLDoesNotWarn(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "docker", "", "qemu", ""))
	assert.Equal(t, engine.Tart, resolve(t, "", "macos", "qemu", ""))
}

// --- engine name validation ---
//
// An unknown engine must fail loudly: runAgent/runBuild/runInit have no
// branch for it, so accepting it silently runs the docker path.

func TestResolveEngine_RejectsUnknownFlagEngine(t *testing.T) {
	for _, name := range []string{"hyperv", "dokcer", "QEMU", "Winkit"} {
		err := resolveErr(t, name, "", "", "")
		assert.Contains(t, err.Error(), "--engine")
		assert.Contains(t, err.Error(), name)
		assert.Contains(t, err.Error(), "docker, tart, winkit")
	}
}

func TestResolveEngine_RejectsUnknownFlagEngineWithOS(t *testing.T) {
	err := resolveErr(t, "hyperv", "windows", "", "")
	assert.Contains(t, err.Error(), "hyperv")
}

func TestResolveEngine_RejectsUnknownTOMLEngine(t *testing.T) {
	err := resolveErr(t, "", "", "hyperv", "")
	assert.Contains(t, err.Error(), "[cell] engine")
	assert.Contains(t, err.Error(), "hyperv")
}

func TestResolveEngine_RejectsLibvirtFlag(t *testing.T) {
	err := resolveErr(t, "libvirt", "", "", "")
	assert.Contains(t, err.Error(), "libvirt engine was retired")
	assert.Contains(t, err.Error(), "cell --engine winkit")
	assert.Contains(t, err.Error(), "macOS host")
}

func TestResolveEngine_RejectsLibvirtFlagWithOS(t *testing.T) {
	err := resolveErr(t, "libvirt", "windows", "", "")
	assert.Contains(t, err.Error(), "cell --engine winkit")
}

func TestResolveEngine_RejectsLibvirtTOML(t *testing.T) {
	err := resolveErr(t, "", "", "libvirt", "")
	assert.Contains(t, err.Error(), "[cell] engine")
	assert.Contains(t, err.Error(), "cell --engine winkit")
	assert.Contains(t, err.Error(), "macOS host")
}

func TestResolveEngine_RejectsRetiredVagrant(t *testing.T) {
	const retired = "vagrant"
	for _, err := range []error{
		resolveErr(t, retired, "", "", ""),
		resolveErr(t, retired, "linux", "", ""),
		resolveErr(t, "", "", retired, ""),
	} {
		assert.Contains(t, err.Error(), retired+" engine was retired")
		assert.Contains(t, err.Error(), "--os macos")
		assert.Contains(t, err.Error(), `[cell] os = "macos"`)
	}
	assert.Contains(t, resolveErr(t, "", "", retired, "").Error(), "[cell] engine")
}

// An explicit --engine or --os overrides the TOML engine, so a stale TOML
// value must not block the override.
func TestResolveEngine_FlagOverridesInvalidTOMLEngine(t *testing.T) {
	assert.Equal(t, engine.Docker, resolve(t, "docker", "", "libvirt", ""))
	assert.Equal(t, engine.Winkit, resolve(t, "", "windows", "libvirt", ""))
}

func TestResolveEngine_AcceptsEveryDispatchedEngine(t *testing.T) {
	for _, name := range []string{"docker", "tart", "winkit"} {
		assert.Equal(t, engine.Name(name), resolve(t, name, "", "", ""), name)
		assert.Equal(t, engine.Name(name), resolve(t, "", "", name, ""), name)
	}
}

// --- capability table ---

func TestValid(t *testing.T) {
	for _, n := range []engine.Name{engine.Docker, engine.Tart, engine.Winkit} {
		assert.True(t, engine.Valid(n), n)
	}
	// qemu is only an alias that Resolve follows, not an engine.
	for _, n := range []engine.Name{"", "hyperv", "qemu", "QEMU"} {
		assert.False(t, engine.Valid(n), n)
	}
	for _, retired := range []engine.Name{"libvirt", "vagrant"} {
		assert.False(t, engine.Valid(retired), retired)
	}
}

func TestRetired(t *testing.T) {
	hint, ok := engine.Retired("libvirt")
	assert.True(t, ok)
	assert.Contains(t, hint, "cell --engine winkit")

	hint, ok = engine.Retired("vagrant")
	assert.True(t, ok)
	assert.Contains(t, hint, "--os macos")

	for _, n := range []engine.Name{engine.Docker, engine.Winkit, "qemu", "hyperv"} {
		_, ok := engine.Retired(n)
		assert.False(t, ok, n)
	}
}

func TestSupports(t *testing.T) {
	cases := []struct {
		engine engine.Name
		guest  engine.Guest
		want   bool
	}{
		{engine.Docker, engine.Linux, true},
		{engine.Docker, engine.MacOS, false},
		{engine.Docker, engine.WindowsPE, false},
		{engine.Docker, engine.WindowsFull, false},
		{engine.Tart, engine.Linux, false}, // planned, not implemented
		{engine.Tart, engine.MacOS, true},
		{engine.Tart, engine.WindowsPE, false},
		{engine.Tart, engine.WindowsFull, false},
		{engine.Winkit, engine.Linux, false},
		{engine.Winkit, engine.MacOS, false},
		{engine.Winkit, engine.WindowsPE, true},
		{engine.Winkit, engine.WindowsFull, true},
		{"qemu", engine.WindowsPE, false},
		{"hyperv", engine.WindowsPE, false},
		{engine.Docker, "freebsd", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, engine.Supports(c.engine, c.guest), "%s on %s", c.engine, c.guest)
	}
}

func TestDefaultFor(t *testing.T) {
	for guest, want := range map[engine.Guest]engine.Name{
		engine.Linux:       engine.Docker,
		engine.MacOS:       engine.Tart,
		engine.WindowsPE:   engine.Winkit,
		engine.WindowsFull: engine.Winkit,
	} {
		got, ok := engine.DefaultFor(guest)
		assert.True(t, ok, guest)
		assert.Equal(t, want, got, guest)
		assert.True(t, engine.Supports(got, guest), "default engine for %s must support it", guest)
	}
	_, ok := engine.DefaultFor("freebsd")
	assert.False(t, ok)
}

func TestGuestValues(t *testing.T) {
	assert.Equal(t, engine.Guest("linux"), engine.Linux)
	assert.Equal(t, engine.Guest("macos"), engine.MacOS)
	assert.Equal(t, engine.Guest("winpe"), engine.WindowsPE)
	assert.Equal(t, engine.Guest("windows-full"), engine.WindowsFull)
	assert.Equal(t, engine.Name("winkit"), engine.Winkit)
}

func TestGuestForOS(t *testing.T) {
	for os, want := range map[string]engine.Guest{
		"linux":   engine.Linux,
		"macos":   engine.MacOS,
		"windows": engine.WindowsFull,
		"winpe":   engine.WindowsPE,
	} {
		got, ok := engine.GuestForOS(os)
		assert.True(t, ok, os)
		assert.Equal(t, want, got, os)
	}
	for _, os := range []string{"", "windows-full", "freebsd", "Windows"} {
		_, ok := engine.GuestForOS(os)
		assert.False(t, ok, os)
	}
}

// --- registry ---

type fakeEngine struct{ engine.Engine }

func (fakeEngine) Run(context.Context, engine.RunOpts) error { return nil }

func TestFor_ReturnsRegisteredEngine(t *testing.T) {
	const name engine.Name = "test-registered"
	want := fakeEngine{}
	engine.Register(name, want)

	got, err := engine.For(name)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestFor_UnregisteredEngine(t *testing.T) {
	_, err := engine.For("test-unregistered")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-unregistered")
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	const name engine.Name = "test-duplicate"
	engine.Register(name, fakeEngine{})
	assert.Panics(t, func() { engine.Register(name, fakeEngine{}) })
}

func TestRegister_PanicsOnNil(t *testing.T) {
	assert.Panics(t, func() { engine.Register("test-nil", nil) })
}

func TestExitError(t *testing.T) {
	err := &engine.ExitError{Code: 3}
	assert.Contains(t, err.Error(), "3")
}

// A --stack override replaces the resolved stack; an absent one keeps it.
func TestCell_WithStack(t *testing.T) {
	c := engine.Cell{Name: "dev", Stack: "base"}
	assert.Equal(t, "go", c.WithStack("go").Stack)
	assert.Equal(t, "base", c.WithStack("").Stack)
	assert.Equal(t, "base", c.Stack, "WithStack modified the receiver")
}
