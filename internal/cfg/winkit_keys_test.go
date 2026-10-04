package cfg_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// clearWinkitEnv unsets every winkit env var, old and new spelling, so the
// developer's shell cannot leak into a Resolved* test.
func clearWinkitEnv(t *testing.T) {
	t.Helper()
	for _, suffix := range []string{"SSH_PORT", "WINDOWS_ISO", "CPUS", "MEMORY_GB", "CACHE_DIR", "ACCEL"} {
		t.Setenv("DEVCELL_WINKIT_"+suffix, "")
		t.Setenv("DEVCELL_QEMU_"+suffix, "")
	}
}

func TestLoadFile_WinkitKeys(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[cell]
winkit_ssh_port = 3333
winkit_windows_iso = "/isos/win.iso"
winkit_cpus = 8
winkit_memory_gb = 16
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cell.WinkitSSHPort != 3333 || c.Cell.WinkitWindowsISO != "/isos/win.iso" || c.Cell.WinkitCPUs != 8 || c.Cell.WinkitMemoryGB != 16 {
		t.Errorf("winkit keys = %+v", c.Cell)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("winkit_* keys must not warn, got %v", deprecatedNames(c))
	}
}

// The qemu_* spellings keep working: they merge into the winkit_* fields
// and warn once each with the new syntax.
func TestLoadFile_QemuKeysMergeIntoWinkitAndWarn(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[cell]
qemu_ssh_port = 3333
qemu_windows_iso = "/isos/win.iso"
qemu_cpus = 8
qemu_memory_gb = 16
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cell.WinkitSSHPort != 3333 || c.Cell.WinkitWindowsISO != "/isos/win.iso" || c.Cell.WinkitCPUs != 8 || c.Cell.WinkitMemoryGB != 16 {
		t.Errorf("qemu_* keys must merge into winkit_*, got %+v", c.Cell)
	}
	want := map[string]string{
		"[cell] qemu_ssh_port":    "use [cell] winkit_ssh_port = 2222 instead",
		"[cell] qemu_windows_iso": `use [cell] winkit_windows_iso = "~/Downloads/Win11_ARM64.iso" instead`,
		"[cell] qemu_cpus":        "use [cell] winkit_cpus = 4 instead",
		"[cell] qemu_memory_gb":   "use [cell] winkit_memory_gb = 8 instead",
	}
	got := map[string]string{}
	for _, u := range c.DeprecatedUses {
		got[u.Name] = u.Message
	}
	if len(got) != len(want) {
		t.Errorf("deprecated = %v, want %d entries", deprecatedNames(c), len(want))
	}
	for name, msg := range want {
		if got[name] != msg {
			t.Errorf("%s: message = %q, want %q", name, got[name], msg)
		}
	}
}

func TestLoadFile_WinkitKeyWinsOverQemuKey(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[cell]
winkit_cpus = 8
qemu_cpus = 2
winkit_windows_iso = "/new.iso"
qemu_windows_iso = "/old.iso"
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cell.WinkitCPUs != 8 || c.Cell.WinkitWindowsISO != "/new.iso" {
		t.Errorf("winkit_* must win, got cpus=%d iso=%q", c.Cell.WinkitCPUs, c.Cell.WinkitWindowsISO)
	}
}

// A qemu_* key in the project file still overrides a winkit_* key in the
// global file: the old spelling is merged per file, before layering.
func TestLoadLayered_ProjectQemuKeyOverridesGlobalWinkitKey(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[cell]\nwinkit_ssh_port = 2200\nwinkit_memory_gb = 4\n")
	project := writeTOML(t, dir, "project.toml", "[cell]\nqemu_ssh_port = 3333\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c.Cell.WinkitSSHPort != 3333 {
		t.Errorf("WinkitSSHPort = %d, want the project's 3333", c.Cell.WinkitSSHPort)
	}
	if c.Cell.WinkitMemoryGB != 4 {
		t.Errorf("WinkitMemoryGB = %d, want the global 4", c.Cell.WinkitMemoryGB)
	}
}

func TestResolvedWinkit_Defaults(t *testing.T) {
	clearWinkitEnv(t)
	var c cfg.CellSection
	if got := c.ResolvedWinkitSSHPort(); got != 2222 {
		t.Errorf("ResolvedWinkitSSHPort = %d, want 2222", got)
	}
	if got := c.ResolvedWinkitWindowsISO(); got != "" {
		t.Errorf("ResolvedWinkitWindowsISO = %q, want empty", got)
	}
	if got := c.ResolvedWinkitCPUs(); got != 4 {
		t.Errorf("ResolvedWinkitCPUs = %d, want 4", got)
	}
	if got := c.ResolvedWinkitMemoryGB(); got != 4 {
		t.Errorf("ResolvedWinkitMemoryGB = %d, want 4", got)
	}
}

func TestResolvedWinkit_TOML(t *testing.T) {
	clearWinkitEnv(t)
	c := cfg.CellSection{WinkitSSHPort: 3333, WinkitWindowsISO: "/w.iso", WinkitCPUs: 8, WinkitMemoryGB: 16}
	if c.ResolvedWinkitSSHPort() != 3333 || c.ResolvedWinkitWindowsISO() != "/w.iso" || c.ResolvedWinkitCPUs() != 8 || c.ResolvedWinkitMemoryGB() != 16 {
		t.Errorf("TOML values must resolve, got %d %q %d %d", c.ResolvedWinkitSSHPort(), c.ResolvedWinkitWindowsISO(), c.ResolvedWinkitCPUs(), c.ResolvedWinkitMemoryGB())
	}
}

func TestResolvedWinkit_WinkitEnvWins(t *testing.T) {
	clearWinkitEnv(t)
	t.Setenv("DEVCELL_WINKIT_SSH_PORT", "4444")
	t.Setenv("DEVCELL_QEMU_SSH_PORT", "5555")
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", "/env.iso")
	t.Setenv("DEVCELL_WINKIT_CPUS", "6")
	t.Setenv("DEVCELL_WINKIT_MEMORY_GB", "12")
	c := cfg.CellSection{WinkitSSHPort: 3333, WinkitWindowsISO: "/w.iso", WinkitCPUs: 8, WinkitMemoryGB: 16}
	if c.ResolvedWinkitSSHPort() != 4444 || c.ResolvedWinkitWindowsISO() != "/env.iso" || c.ResolvedWinkitCPUs() != 6 || c.ResolvedWinkitMemoryGB() != 12 {
		t.Errorf("DEVCELL_WINKIT_* must win, got %d %q %d %d", c.ResolvedWinkitSSHPort(), c.ResolvedWinkitWindowsISO(), c.ResolvedWinkitCPUs(), c.ResolvedWinkitMemoryGB())
	}
}

func TestResolvedWinkit_QemuEnvFallback(t *testing.T) {
	clearWinkitEnv(t)
	t.Setenv("DEVCELL_QEMU_SSH_PORT", "5555")
	t.Setenv("DEVCELL_QEMU_WINDOWS_ISO", "/old-env.iso")
	t.Setenv("DEVCELL_QEMU_CPUS", "3")
	t.Setenv("DEVCELL_QEMU_MEMORY_GB", "9")
	c := cfg.CellSection{WinkitSSHPort: 3333, WinkitWindowsISO: "/w.iso", WinkitCPUs: 8, WinkitMemoryGB: 16}
	if c.ResolvedWinkitSSHPort() != 5555 || c.ResolvedWinkitWindowsISO() != "/old-env.iso" || c.ResolvedWinkitCPUs() != 3 || c.ResolvedWinkitMemoryGB() != 9 {
		t.Errorf("DEVCELL_QEMU_* must still be read, got %d %q %d %d", c.ResolvedWinkitSSHPort(), c.ResolvedWinkitWindowsISO(), c.ResolvedWinkitCPUs(), c.ResolvedWinkitMemoryGB())
	}
}

func TestGetenv_RenamedWinkitVars(t *testing.T) {
	clearWinkitEnv(t)
	t.Setenv("DEVCELL_QEMU_CACHE_DIR", "/old-cache")
	if got := cfg.Getenv("DEVCELL_WINKIT_CACHE_DIR"); got != "/old-cache" {
		t.Errorf("Getenv falls back to DEVCELL_QEMU_CACHE_DIR, got %q", got)
	}
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "/new-cache")
	if got := cfg.Getenv("DEVCELL_WINKIT_CACHE_DIR"); got != "/new-cache" {
		t.Errorf("DEVCELL_WINKIT_CACHE_DIR must win, got %q", got)
	}
	t.Setenv("DEVCELL_QEMU_ACCEL", "tcg")
	if got := cfg.Getenv("DEVCELL_WINKIT_ACCEL"); got != "tcg" {
		t.Errorf("Getenv falls back to DEVCELL_QEMU_ACCEL, got %q", got)
	}
}

// ApplyEnv records one warning per DEVCELL_QEMU_* var that is set while its
// DEVCELL_WINKIT_* replacement is not; checkConfig prints them once.
func TestApplyEnv_QemuEnvVarsWarn(t *testing.T) {
	env := map[string]string{
		"DEVCELL_QEMU_CPUS":          "8",
		"DEVCELL_QEMU_CACHE_DIR":     "/cache",
		"DEVCELL_QEMU_WINDOWS_ISO":   "/old.iso",
		"DEVCELL_WINKIT_WINDOWS_ISO": "/new.iso",
	}
	c := cfg.CellConfig{}
	cfg.ApplyEnv(&c, func(k string) string { return env[k] })
	got := map[string]string{}
	for _, u := range c.DeprecatedUses {
		got[u.Name] = u.Warning()
	}
	if len(got) != 2 {
		t.Fatalf("want 2 warnings (CPUS, CACHE_DIR), got %v", got)
	}
	want := "DEVCELL_QEMU_CPUS is deprecated and will be removed in a future release: use DEVCELL_WINKIT_CPUS instead, e.g. export DEVCELL_WINKIT_CPUS=8"
	if got["DEVCELL_QEMU_CPUS"] != want {
		t.Errorf("warning =\n%q\nwant\n%q", got["DEVCELL_QEMU_CPUS"], want)
	}
	if !strings.Contains(got["DEVCELL_QEMU_CACHE_DIR"], "use DEVCELL_WINKIT_CACHE_DIR instead") {
		t.Errorf("CACHE_DIR warning = %q", got["DEVCELL_QEMU_CACHE_DIR"])
	}
}
