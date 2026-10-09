package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// runWinkit runs cell in a fresh project with projectTOML and extra env, and
// returns the combined output. It fails the test on a non-zero exit.
func runWinkit(t *testing.T, projectTOML string, extraEnv []string, args ...string) string {
	t.Helper()
	home := qemuTestHomeWithTOML(t, projectTOML)
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home, "XDG_CONFIG_HOME=",
		"DEVCELL_WINKIT_WINDOWS_ISO=", "DEVCELL_QEMU_WINDOWS_ISO=",
		"DEVCELL_WINKIT_SSH_PORT=", "DEVCELL_QEMU_SSH_PORT=")
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cell %v: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

func TestWinkitBuild_WinkitWindowsISOFlag(t *testing.T) {
	out := runWinkit(t, "[cell]\n", nil, "build", "--engine", "winkit", "--dry-run", "--winkit-windows-iso", "x")
	if !strings.Contains(out, "windowsISO: x") {
		t.Errorf("want windowsISO: x in dry-run output, got:\n%s", out)
	}
	if strings.Contains(out, "deprecated") {
		t.Errorf("--winkit-windows-iso must not warn, got:\n%s", out)
	}
}

func TestWinkitBuild_QemuWindowsISOFlagStillWorksAndWarns(t *testing.T) {
	out := runWinkit(t, "[cell]\n", nil, "build", "--engine", "winkit", "--dry-run", "--qemu-windows-iso", "x")
	if !strings.Contains(out, "windowsISO: x") {
		t.Errorf("want windowsISO: x in dry-run output, got:\n%s", out)
	}
	const warning = "--qemu-windows-iso is deprecated (use --winkit-windows-iso instead"
	if n := strings.Count(out, warning); n != 1 {
		t.Errorf("want the deprecation warning once, got %d times:\n%s", n, out)
	}
}

// Agent commands scan the flags from argv; the old spelling still sets
// the port and warns once.
func TestWinkitShell_QemuSSHPortFlagStillWorksAndWarns(t *testing.T) {
	out := runWinkit(t, "[cell]\n", nil, "--engine=winkit", "--os=winpe", "shell", "--dry-run", "--qemu-ssh-port", "3333")
	if !strings.Contains(out, "127.0.0.1:3333") {
		t.Errorf("want SSH port 3333 in dry-run output, got:\n%s", out)
	}
	const warning = "--qemu-ssh-port is deprecated (use --winkit-ssh-port instead"
	if n := strings.Count(out, warning); n != 1 {
		t.Errorf("want the deprecation warning once, got %d times:\n%s", n, out)
	}
}

func TestWinkitShell_WinkitSSHPortFlag(t *testing.T) {
	out := runWinkit(t, "[cell]\n", nil, "--engine=winkit", "--os=winpe", "shell", "--dry-run", "--winkit-ssh-port", "3333")
	if !strings.Contains(out, "127.0.0.1:3333") {
		t.Errorf("want SSH port 3333 in dry-run output, got:\n%s", out)
	}
	if strings.Contains(out, "deprecated") {
		t.Errorf("--winkit-ssh-port must not warn, got:\n%s", out)
	}
}

func TestWinkitShell_QemuSSHPortKeyStillWorksAndWarns(t *testing.T) {
	out := runWinkit(t, "[cell]\nqemu_ssh_port = 3333\n", nil, "--engine=winkit", "--os=winpe", "shell", "--dry-run")
	if !strings.Contains(out, "127.0.0.1:3333") {
		t.Errorf("want SSH port 3333 in dry-run output, got:\n%s", out)
	}
	const warning = "[cell] qemu_ssh_port is deprecated (use [cell] winkit_ssh_port = 2222 instead)"
	if n := strings.Count(out, warning); n != 1 {
		t.Errorf("want the deprecation warning once, got %d times:\n%s", n, out)
	}
}

func TestWinkitShell_QemuSSHPortEnvStillWorksAndWarns(t *testing.T) {
	out := runWinkit(t, "[cell]\n", []string{"DEVCELL_QEMU_SSH_PORT=3333"}, "--engine=winkit", "--os=winpe", "shell", "--dry-run")
	if !strings.Contains(out, "127.0.0.1:3333") {
		t.Errorf("want SSH port 3333 in dry-run output, got:\n%s", out)
	}
	const warning = "DEVCELL_QEMU_SSH_PORT is deprecated (use DEVCELL_WINKIT_SSH_PORT instead"
	if n := strings.Count(out, warning); n != 1 {
		t.Errorf("want the deprecation warning once, got %d times:\n%s", n, out)
	}
}
