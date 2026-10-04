package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
)

func TestWarnConfigDeprecations(t *testing.T) {
	c := cfg.CellConfig{DeprecatedUses: []cfg.DeprecatedUse{
		{Deprecation: cfg.Deprecation{Name: "[ports] forward", Replacement: "[cell] ports", Message: "use [cell] ports instead"}, File: "/g/devcell.toml"},
		{Deprecation: cfg.Deprecation{Name: "[mcp] enabled", Replacement: "[cell] mcps", Message: "use [cell] mcps instead"}, File: "/p/.devcell.toml"},
	}}
	var buf bytes.Buffer
	warnConfigDeprecations(&buf, c)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 warning lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasSuffix(lines[0], "[ports] forward is deprecated (use [cell] ports instead) (devcell.toml)") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "use [cell] mcps instead") {
		t.Errorf("line 1 = %q", lines[1])
	}
}

func TestWarnConfigDeprecations_NoneIsSilent(t *testing.T) {
	var buf bytes.Buffer
	warnConfigDeprecations(&buf, cfg.CellConfig{})
	if buf.Len() != 0 {
		t.Errorf("want no output, got %q", buf.String())
	}
}

// withArgs sets the argv scanFlag and scanStringFlag read for one test.
func withArgs(t *testing.T, args ...string) {
	t.Helper()
	old := osArgs
	t.Cleanup(func() { osArgs = old })
	osArgs = append([]string{"cell"}, args...)
}

// warnDeprecatedFlagsOutput returns what warnDeprecatedFlags prints for args.
func warnDeprecatedFlagsOutput(t *testing.T, args ...string) string {
	t.Helper()
	withArgs(t, args...)
	var buf bytes.Buffer
	warnDeprecatedFlags(&buf)
	return buf.String()
}

// --local only skipped the retired qemu-to-libvirt auto-default. Agent
// commands scan it from argv (DisableFlagParsing), so cobra's MarkDeprecated
// never fires there and runAgent must warn itself.
func TestWarnDeprecatedFlags_Local(t *testing.T) {
	got := warnDeprecatedFlagsOutput(t, "--engine=winkit", "--local", "claude")
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("want exactly one warning line, got %q", got)
	}
	if !strings.HasPrefix(got, " ⚠      --local is deprecated (remove it;") {
		t.Errorf("warning = %q", got)
	}
	if !strings.Contains(got, "libvirt engine was retired") {
		t.Errorf("warning must say why --local does nothing, got %q", got)
	}
}

func TestWarnDeprecatedFlags_AbsentIsSilent(t *testing.T) {
	got := warnDeprecatedFlagsOutput(t, "--engine=winkit", "--os", "macos", "claude", "--local-provider", "ollama", "--macos-version")
	if got != "" {
		t.Errorf("want no output, got %q", got)
	}
}

func TestWarnDeprecatedFlags_Macos(t *testing.T) {
	got := warnDeprecatedFlagsOutput(t, "--macos", "claude")
	want := " ⚠      --macos is deprecated (use --os macos instead)\n"
	if got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}
}

// The vagrant engine was retired: --vagrant-provider and --vagrant-box are
// still accepted, warn once each, and change nothing.
func TestWarnDeprecatedFlags_VagrantFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--vagrant-provider", "utm", "claude"},
		{"--vagrant-provider=libvirt", "claude"},
		{"--vagrant-box", "utm/bookworm", "claude"},
		{"--vagrant-box=mybox", "claude"},
	} {
		got := warnDeprecatedFlagsOutput(t, args...)
		flag, _, _ := strings.Cut(args[0], "=")
		want := " ⚠      " + flag + " is deprecated (remove it; the vagrant engine was retired, use --os macos for a Tart VM)\n"
		if got != want {
			t.Errorf("%v: warning = %q, want %q", args, got, want)
		}
	}
}

// The engine was renamed qemu to winkit; its knobs follow. The old
// spellings keep working and warn once with the new flag.
func TestWarnDeprecatedFlags_QemuFlagsRenamedToWinkit(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--qemu-ssh-port", "3333", "shell"}, " ⚠      --qemu-ssh-port is deprecated (use --winkit-ssh-port instead, e.g. --winkit-ssh-port 2222)\n"},
		{[]string{"--qemu-windows-iso=/w.iso", "shell"}, " ⚠      --qemu-windows-iso is deprecated (use --winkit-windows-iso instead, e.g. --winkit-windows-iso ~/Downloads/Win11_ARM64.iso)\n"},
	} {
		if got := warnDeprecatedFlagsOutput(t, tc.args...); got != tc.want {
			t.Errorf("%v: warning = %q, want %q", tc.args, got, tc.want)
		}
	}
	if got := warnDeprecatedFlagsOutput(t, "--winkit-ssh-port", "3333", "--winkit-windows-iso=/w.iso", "shell"); got != "" {
		t.Errorf("winkit flags must not warn, got %q", got)
	}
}

// Flags that never did anything: accepted, warn once, ignored.
func TestWarnDeprecatedFlags_DeadEngineFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--qemu-ssh-host", "10.0.0.5", "shell"}, "remove it; it has no effect (winkit forwards SSH on 127.0.0.1)"},
		{[]string{"--qemu-display=cocoa", "shell"}, "remove it; it has no effect (use `cell vnc` or `cell rdp` to see the VM)"},
		{[]string{"--tart-ssh-port", "22", "shell"}, "remove it; it has no effect (tart runs commands with tart exec)"},
		{[]string{"--tart-ssh-host=localhost", "shell"}, "remove it; it has no effect (tart runs commands with tart exec)"},
	} {
		flag, _, _ := strings.Cut(tc.args[0], "=")
		want := " ⚠      " + flag + " is deprecated (" + tc.want + ")\n"
		if got := warnDeprecatedFlagsOutput(t, tc.args...); got != want {
			t.Errorf("%v: warning = %q, want %q", tc.args, got, want)
		}
	}
}

func TestWinkitFlags_StrippedFromAgentArgs(t *testing.T) {
	got := stripCellFlags([]string{
		"--winkit-ssh-port", "3333", "--winkit-windows-iso=/w.iso",
		"--qemu-ssh-port=1", "--qemu-windows-iso", "/o.iso",
		"--qemu-ssh-host", "h", "--qemu-display=none",
		"--tart-ssh-port", "22", "--tart-ssh-host=h",
		"-p", "hi",
	})
	if strings.Join(got, " ") != "-p hi" {
		t.Errorf("stripCellFlags = %q, want [-p hi]", got)
	}
}

func TestWinkitFlags_InHelp(t *testing.T) {
	for _, flag := range []string{"winkit-ssh-port", "winkit-windows-iso"} {
		f := rootCmd.PersistentFlags().Lookup(flag)
		if f == nil {
			t.Errorf("--%s is not registered", flag)
			continue
		}
		if f.Deprecated != "" || f.Hidden {
			t.Errorf("--%s must be visible in help", flag)
		}
	}
}

func TestVagrantFlags_IgnoredByEngineResolution(t *testing.T) {
	withArgs(t, "--vagrant-provider", "utm", "--vagrant-box=mybox", "claude")
	var buf bytes.Buffer
	got, err := resolveEngine(&buf, cfg.CellConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got != engine.Docker {
		t.Errorf("engine = %q, want docker", got)
	}
}

func TestDeprecatedFlags_StillStrippedFromAgentArgs(t *testing.T) {
	got := stripCellFlags([]string{
		"--local", "--macos",
		"--vagrant-provider", "utm", "--vagrant-box=mybox",
		"-p", "hi",
	})
	if strings.Join(got, " ") != "-p hi" {
		t.Errorf("stripCellFlags = %q, want [-p hi]", got)
	}
}

// Cobra-parsed commands (build, init) accept the deprecated flags too:
// each stays registered but hidden from help, and the notice comes from
// warnDeprecatedFlags (root PersistentPreRun) rather than cobra's own
// "Flag --x has been deprecated" line, so it renders through ux.Deprecated.
func TestDeprecatedFlags_HiddenAndScannedForCobraCommands(t *testing.T) {
	for _, d := range scannedFlagDeprecations {
		name := strings.TrimPrefix(d.flag, "--")
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("%s must stay accepted so existing invocations keep working", d.flag)
			continue
		}
		if !f.Hidden {
			t.Errorf("%s must be hidden from help", d.flag)
		}
		if f.Deprecated != "" {
			t.Errorf("%s must not use cobra's MarkDeprecated (notice would bypass ux.Deprecated)", d.flag)
		}
	}
}

// init used to define its own --macos (the vagrant macOS box). A local flag
// would shadow the deprecated persistent one and advertise it in help.
func TestInitMacosFlag_IsTheDeprecatedPersistentFlag(t *testing.T) {
	if f := initCmd.Flags().Lookup("macos"); f != nil && !f.Hidden {
		t.Errorf("init defines its own visible --macos: %q", f.Usage)
	}
}

// --- engine resolution in cmd ---

func TestResolveEngine_MacosFlagIsDeprecatedAliasForOSMacos(t *testing.T) {
	withArgs(t, "--macos", "claude")
	var buf bytes.Buffer
	got, err := resolveEngine(&buf, cfg.CellConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got != engine.Tart {
		t.Errorf("engine = %q, want tart", got)
	}
	if buf.Len() != 0 {
		t.Errorf("resolveEngine must leave the --macos warning to warnDeprecatedFlags or cobra, got %q", buf.String())
	}
}

func TestResolveEngine_MacosFlagDefersToOS(t *testing.T) {
	withArgs(t, "--macos", "--os", "linux", "claude")
	var buf bytes.Buffer
	got, err := resolveEngine(&buf, cfg.CellConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got != engine.Docker {
		t.Errorf("engine = %q, want docker", got)
	}
}

const qemuEngineWarning = ` ⚠      engine "qemu" is deprecated (use engine = "winkit" instead)` + "\n"

func TestResolveEngine_QemuFlagWarnsOnce(t *testing.T) {
	withArgs(t, "--engine=qemu", "claude")
	var buf bytes.Buffer
	got, err := resolveEngine(&buf, cfg.CellConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got != engine.Winkit {
		t.Errorf("engine = %q, want winkit", got)
	}
	if buf.String() != qemuEngineWarning {
		t.Errorf("warning = %q, want %q", buf.String(), qemuEngineWarning)
	}
}

func TestResolveEngine_QemuTOMLWarnsOnce(t *testing.T) {
	withArgs(t, "claude")
	var buf bytes.Buffer
	got, err := resolveEngine(&buf, cfg.CellConfig{Cell: cfg.CellSection{Engine: "qemu"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != engine.Winkit {
		t.Errorf("engine = %q, want winkit", got)
	}
	if buf.String() != qemuEngineWarning {
		t.Errorf("warning = %q, want %q", buf.String(), qemuEngineWarning)
	}
}

func TestResolveEngine_RetiredVagrantEngine(t *testing.T) {
	withArgs(t, "--engine", "vagrant", "claude")
	var buf bytes.Buffer
	if _, err := resolveEngine(&buf, cfg.CellConfig{}); err == nil || !strings.Contains(err.Error(), "vagrant engine was retired") {
		t.Errorf("--engine vagrant: err = %v, want the retired-engine hint", err)
	}

	withArgs(t, "claude")
	_, err := resolveEngine(&buf, cfg.CellConfig{Cell: cfg.CellSection{Engine: "vagrant"}}) // retired
	if err == nil || !strings.Contains(err.Error(), `[cell] engine "vagrant": the vagrant engine was retired`) {
		t.Errorf("[cell] engine = vagrant: err = %v, want the retired-engine hint", err)
	}
}

// --- help text ---

func TestEngineAndOSFlagHelp(t *testing.T) {
	engineHelp := rootCmd.PersistentFlags().Lookup("engine").Usage
	for _, want := range []string{"docker", "tart", "winkit"} {
		if !strings.Contains(engineHelp, want) {
			t.Errorf("--engine help %q must mention %s", engineHelp, want)
		}
	}
	osHelp := rootCmd.PersistentFlags().Lookup("os").Usage
	for _, want := range []string{"linux", "macos", "windows", "winpe"} {
		if !strings.Contains(osHelp, want) {
			t.Errorf("--os help %q must mention %s", osHelp, want)
		}
	}
	for _, help := range []string{engineHelp, osHelp} {
		if strings.Contains(help, "qemu") || strings.Contains(help, "vagrant") { // deprecated and retired names
			t.Errorf("help %q must not advertise deprecated or retired engines", help)
		}
	}
}
