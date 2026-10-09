package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// qemuTestHome sets up a temp HOME with config dir and .devcell.toml for qemu tests.
func qemuTestHome(t *testing.T) string {
	t.Helper()
	return qemuTestHomeWithTOML(t, "[cell]\n")
}

// qemuTestHomeWithTOML sets up a temp HOME with custom project TOML content.
func qemuTestHomeWithTOML(t *testing.T, projectTOML string) string {
	t.Helper()
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "devcell")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "devcell.toml"), []byte("[cell]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".devcell.toml"), []byte(projectTOML), 0644); err != nil {
		t.Fatal(err)
	}
	return home
}

// --- Cross-platform smoke tests (dry-run, mock, help) ---

func TestEngineQemu_DryRunShell(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home, "TERM=xterm-256color")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)

	for _, tc := range []struct{ label, want string }{
		{"ssh command", "ssh"},
		{"WSL guest (wsl)", "wsl"},
		{"WSL guest (-d winkit)", "-d winkit"},
		{"bunk SSH port (10122+100)", "127.0.0.1:10222"},
		{"TERM forwarded", "TERM=xterm-256color"},
	} {
		if !strings.Contains(s, tc.want) {
			t.Errorf("%s: want %q in output, got:\n%s", tc.label, tc.want, s)
		}
	}
	if strings.Contains(s, "docker run") {
		t.Errorf("winkit engine must not print docker run argv, got:\n%s", s)
	}
}

func TestEngineQemu_DryRunClaude(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "claude", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)

	if !strings.Contains(s, " claude --dangerously-skip-permissions") {
		t.Errorf("want claude binary and default flags in guest command, got:\n%s", s)
	}
	if strings.Contains(s, "docker") {
		t.Errorf("winkit engine must not involve docker, got:\n%s", s)
	}
}

func TestEngineQemu_DryRunCustomSSHPort(t *testing.T) {
	home := qemuTestHomeWithTOML(t, "[cell]\nwinkit_ssh_port = 3333\n")
	cmd := exec.Command(binaryPath, "--engine=winkit", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "127.0.0.1:3433") {
		t.Errorf("expected custom SSH port 3433 (3333+100) in dry-run output, got:\n%s", out)
	}
}

func TestEngineWinkit_EngineHelpIncludesWinkit(t *testing.T) {
	out, err := exec.Command(binaryPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "winkit") {
		t.Errorf("expected 'winkit' in --help output for --engine flag, got:\n%s", out)
	}
}

// --engine=qemu is the deprecated name of winkit: it still runs the winkit
// path and warns exactly once per invocation.
func TestEngineQemu_DeprecatedAliasForWinkit(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=qemu", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "wsl") || !strings.Contains(s, "-d winkit") {
		t.Errorf("expected the WSL guest command in dry-run output, got:\n%s", s)
	}
	const warning = `engine "qemu" is deprecated (use engine = "winkit" instead)`
	if n := strings.Count(s, warning); n != 1 {
		t.Errorf("want the deprecation warning exactly once, got %d times:\n%s", n, s)
	}
}

func TestBackgroundFlag_StrippedFromArgsQemu(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "--background", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)
	if strings.Contains(s, "--background") {
		t.Errorf("--background should be stripped from forwarded args, got:\n%s", s)
	}
}

// --- Non-darwin tests ---

func TestEngineQemu_NoDebugOnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this test validates the non-darwin error path")
	}
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "shell")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error on non-darwin without --debug, got exit 0:\n%s", out)
	}
	s := string(out)
	if !strings.Contains(s, "qemu engine requires macOS") {
		t.Errorf("expected 'qemu engine requires macOS' in error, got:\n%s", s)
	}
	if !strings.Contains(s, "--debug to simulate") {
		t.Errorf("expected '--debug to simulate' hint in error, got:\n%s", s)
	}
}

func TestEngineQemu_DebugMockOutput(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("mock output only on non-darwin")
	}
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "--debug", "shell")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 with --debug mock, got: %v\noutput: %s", err, out)
	}
	s := string(out)
	for _, want := range []string{
		"[MOCK",
		"mock mode",
		"would exec",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in debug mock output, got:\n%s", want, s)
		}
	}
}

func TestEngineQemu_DebugMockNoDocker(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("mock output only on non-darwin")
	}
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "--debug", "shell")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	if strings.Contains(string(out), "docker") {
		t.Errorf("mock output should not mention docker, got:\n%s", out)
	}
}

// TestEngineQemu_BuildDryRun verifies that --dry-run prints what would be
// built, including the stage-named artifact, without starting QEMU.
func TestEngineQemu_BuildDryRun(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "build", "--dry-run", "--stack=base")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "Windows+WSL1 build (dry-run)") {
		t.Errorf("expected 'Windows+WSL1 build (dry-run)' in --dry-run output, got:\n%s", s)
	}
	if !strings.Contains(s, filepath.Join(".devcell", "windows", "full-wsl", "base", "winkit-full-wsl.qcow2")) {
		t.Errorf("expected the stage-named artifact under the base template dir in --dry-run output, got:\n%s", s)
	}
}

// TestEngineQemu_DryRunGuestEnv checks the unified cell env reaches the
// PE+WSL1 guest. Before cell.GuestEnv, qemu dropped [env] and [mise] and did
// not set LC_ALL or IS_SANDBOX.
func TestEngineQemu_DryRunGuestEnv(t *testing.T) {
	assertGuestEnvDryRun(t, "--engine=winkit")
}

// --stack=<name> and --force in an agent's args are cell's, not the
// agent's: the stack picks the template, and neither reaches the guest.
func TestEngineWinkit_DryRunTakesStackAndForceFromArgs(t *testing.T) {
	home := qemuTestHome(t)
	cmd := exec.Command(binaryPath, "--engine=winkit", "shell", "--stack=go", "--force", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v\noutput: %s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, filepath.Join(".devcell", "windows", "full-wsl", "go", "winkit-full-wsl.qcow2")) {
		t.Errorf("expected the go stack's template in dry-run output, got:\n%s", s)
	}
	for _, arg := range []string{"--stack", "--force"} {
		if strings.Contains(s, arg) {
			t.Errorf("%s must not reach the guest command, got:\n%s", arg, s)
		}
	}
}
