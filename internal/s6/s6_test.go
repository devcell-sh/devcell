package s6

import (
	"strings"
	"testing"
)

func TestConstants(t *testing.T) {
	if EnvDir != "/etc/s6/env" {
		t.Errorf("EnvDir = %q, want /etc/s6/env", EnvDir)
	}
	if SourceDir != "/etc/s6-rc/source" {
		t.Errorf("SourceDir = %q, want /etc/s6-rc/source", SourceDir)
	}
	if CompiledDir != "/etc/s6-rc/compiled" {
		t.Errorf("CompiledDir = %q, want /etc/s6-rc/compiled", CompiledDir)
	}
	if ScanDir != "/run/service" {
		t.Errorf("ScanDir = %q, want /run/service", ScanDir)
	}
	if LiveDir != "/run/s6-rc" {
		t.Errorf("LiveDir = %q, want /run/s6-rc", LiveDir)
	}
	if ReadyFile != "/run/devcell-session-ready" {
		t.Errorf("ReadyFile = %q, want /run/devcell-session-ready", ReadyFile)
	}
	if ServicesDir != SourceDir {
		t.Errorf("ServicesDir must alias SourceDir for backward compat")
	}
}

func TestWriteEnvScript(t *testing.T) {
	env := SessionEnv{
		HostUser:    "dmitry",
		SessionHome: "/Users/dmitry",
		DevcellHome: "/Users/devcell",
	}
	script := WriteEnvScript(env)

	if !strings.Contains(script, "sudo mkdir -p") {
		t.Error("must create env dir with sudo")
	}
	if !strings.Contains(script, EnvDir) {
		t.Errorf("must reference EnvDir (%s)", EnvDir)
	}
	for _, kv := range []struct{ key, val string }{
		{"HOST_USER", "dmitry"},
		{"SESSION_HOME", "/Users/dmitry"},
		{"DEVCELL_HOME", "/Users/devcell"},
	} {
		if !strings.Contains(script, kv.key) {
			t.Errorf("must write %s env var", kv.key)
		}
		if !strings.Contains(script, kv.val) {
			t.Errorf("must contain value %q for %s", kv.val, kv.key)
		}
	}
	if !strings.Contains(script, "sudo tee") {
		t.Error("env writes must use sudo tee")
	}
}

func TestWriteEnvScript_Idempotent(t *testing.T) {
	env := SessionEnv{
		HostUser:    "test",
		SessionHome: "/home/test",
		DevcellHome: "/opt/devcell",
	}
	s1 := WriteEnvScript(env)
	s2 := WriteEnvScript(env)
	if s1 != s2 {
		t.Error("WriteEnvScript must be deterministic")
	}
}

func TestDarwinSessionEnv(t *testing.T) {
	env := DarwinSessionEnv("dmitry", "devcell")
	if env.HostUser != "dmitry" {
		t.Errorf("HostUser = %q, want dmitry", env.HostUser)
	}
	if env.SessionHome != "/Users/dmitry" {
		t.Errorf("SessionHome = %q, want /Users/dmitry", env.SessionHome)
	}
	if env.DevcellHome != "/Users/devcell" {
		t.Errorf("DevcellHome = %q, want /Users/devcell", env.DevcellHome)
	}
}

func TestLinuxSessionEnv(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	if env.HostUser != "dmitry" {
		t.Errorf("HostUser = %q, want dmitry", env.HostUser)
	}
	if env.SessionHome != "/home/dmitry" {
		t.Errorf("SessionHome = %q, want /home/dmitry", env.SessionHome)
	}
	if env.DevcellHome != "/opt/devcell" {
		t.Errorf("DevcellHome = %q, want /opt/devcell", env.DevcellHome)
	}
}

func TestWSL1SessionEnv(t *testing.T) {
	env := WSL1SessionEnv("root")
	if env.HostUser != "root" {
		t.Errorf("HostUser = %q, want root", env.HostUser)
	}
	if env.SessionHome != "/root" {
		t.Errorf("SessionHome = %q, want /root", env.SessionHome)
	}
	if env.DevcellHome != "/opt/devcell" {
		t.Errorf("DevcellHome = %q, want /opt/devcell", env.DevcellHome)
	}

	env2 := WSL1SessionEnv("user")
	if env2.SessionHome != "/home/user" {
		t.Errorf("SessionHome = %q, want /home/user", env2.SessionHome)
	}
}

func TestWSL1ExecPATH(t *testing.T) {
	env := WSL1SessionEnv("dmitry")
	path := WSL1ExecPATH(env)

	for _, required := range []string{
		"/home/dmitry/go/bin",
		"/home/dmitry/.local/state/nix/profiles/profile/bin",
		"/home/dmitry/.local/share/mise/shims",
		"/home/dmitry/.local/bin",
		"/opt/devcell/.local/state/nix/profiles/profile/bin",
		"/nix/var/nix/profiles/default/bin",
	} {
		if !strings.Contains(path, required) {
			t.Errorf("WSL1ExecPATH must include %s, got: %q", required, path)
		}
	}

	goIdx := strings.Index(path, "/home/dmitry/go/bin")
	nixIdx := strings.Index(path, "/nix/var/nix/profiles/default/bin")
	if goIdx < 0 || nixIdx < 0 || goIdx > nixIdx {
		t.Error("session-user paths must precede system paths in WSL1ExecPATH")
	}
}

func TestWSL1ExecPATH_Root(t *testing.T) {
	env := WSL1SessionEnv("root")
	path := WSL1ExecPATH(env)
	if !strings.Contains(path, "/root/go/bin") {
		t.Errorf("WSL1ExecPATH for root must use /root, got: %q", path)
	}
}

func TestLinuxExecPATH(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	path := LinuxExecPATH(env)

	for _, required := range []string{
		"/home/dmitry/go/bin",
		"/home/dmitry/.local/state/nix/profiles/profile/bin",
		"/home/dmitry/.local/share/mise/shims",
		"/home/dmitry/.local/bin",
		"/run/wrappers/bin",
		"/nix/var/nix/profiles/devcell-tools/bin",
		"/opt/devcell/.local/state/nix/profiles/profile/bin",
		"/nix/var/nix/profiles/default/bin",
	} {
		if !strings.Contains(path, required) {
			t.Errorf("LinuxExecPATH must include %s, got: %q", required, path)
		}
	}

	goIdx := strings.Index(path, "/home/dmitry/go/bin")
	nixIdx := strings.Index(path, "/nix/var/nix/profiles/default/bin")
	if goIdx < 0 || nixIdx < 0 || goIdx > nixIdx {
		t.Error("session-user paths must precede system paths in LinuxExecPATH")
	}
}

func TestLinuxExecPATH_MatchesEntrypoint(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	path := LinuxExecPATH(env)
	snippet := EntrypointSnippet(env)
	if !strings.Contains(snippet, "export PATH=") {
		t.Fatal("entrypoint must export PATH")
	}
	for _, segment := range strings.Split(path, ":") {
		resolved := strings.ReplaceAll(segment, "/home/dmitry", "$SESSION_HOME")
		resolved = strings.ReplaceAll(resolved, "/opt/devcell", "$DEVCELL_HOME")
		if !strings.Contains(snippet, segment) && !strings.Contains(snippet, resolved) {
			t.Errorf("LinuxExecPATH segment %q not found in entrypoint PATH", segment)
		}
	}
}

func TestEntrypointSnippet_NotifyEmitsJSONL(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	script := EntrypointSnippet(env)

	if !strings.Contains(script, `"scope":"system"`) {
		t.Error("_notify must emit JSONL with scope=system")
	}
	if !strings.Contains(script, "/proc/1/fd/1") {
		t.Error("_notify must check PID 1 stdout for TTY guard")
	}
	if !strings.Contains(script, "/dev/pts/") {
		t.Error("_notify must have TTY guard for /dev/pts/*")
	}
}

// --- EntrypointSnippet tests ---

func TestEntrypointSnippet(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	script := EntrypointSnippet(env)

	// Must NOT invoke sudo (entrypoint runs as root).
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sudo ") {
			t.Errorf("entrypoint snippet must not invoke sudo (runs as root): %s", trimmed)
		}
	}

	// Must set explicit PATH before using any tools
	if !strings.Contains(script, "export PATH=") {
		t.Error("must export PATH explicitly")
	}
	pathPos := strings.Index(script, "export PATH=")
	for _, required := range []string{
		"/nix/var/nix/profiles/devcell-tools/bin",
		"/nix/var/nix/profiles/default/bin",
		"$DEVCELL_HOME/.local/state/nix/profiles/profile/bin",
		"$SESSION_HOME/go/bin",
		"$SESSION_HOME/.local/share/mise/shims",
		"$SESSION_HOME/.local/state/nix/profiles/profile/bin",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("PATH must include %s", required)
		}
	}
	mkdirPos := strings.Index(script, "mkdir")
	if pathPos < 0 || mkdirPos < 0 || pathPos > mkdirPos {
		t.Error("PATH export must come before first tool usage")
	}

	// Must write envdir vars
	if !strings.Contains(script, EnvDir) {
		t.Errorf("must reference EnvDir (%s)", EnvDir)
	}
	if !strings.Contains(script, "HOST_USER") {
		t.Error("must write HOST_USER")
	}

	// Must use s6-svscan for supervision
	if !strings.Contains(script, "s6-svscan") {
		t.Error("must start s6-svscan supervision tree")
	}
	if !strings.Contains(script, ScanDir) {
		t.Errorf("must reference ScanDir (%s)", ScanDir)
	}

	// Must use s6-rc-init with compiled database
	if !strings.Contains(script, "s6-rc-init") {
		t.Error("must run s6-rc-init to initialize live state")
	}
	if !strings.Contains(script, CompiledDir) {
		t.Errorf("must reference CompiledDir (%s)", CompiledDir)
	}
	if !strings.Contains(script, LiveDir) {
		t.Errorf("must reference LiveDir (%s)", LiveDir)
	}

	// Must use s6-rc change to bring up services
	if !strings.Contains(script, "s6-rc") {
		t.Error("must use s6-rc to manage service lifecycle")
	}
	if !strings.Contains(script, "change user") {
		t.Error("must bring up the 'user' bundle via s6-rc change")
	}

	// Must NOT use setsid (s6-svscan handles supervision)
	if strings.Contains(script, "setsid") {
		t.Error("must not use setsid: s6-svscan supervises longruns")
	}

	// Must NOT use custom topo sort
	if strings.Contains(script, "topo_order") {
		t.Error("must not use custom topo sort: s6-rc handles dependency resolution")
	}

	// Must NOT reference entrypoint.d fragments
	if strings.Contains(script, "/etc/devcell/entrypoint.d") {
		t.Error("must not reference entrypoint.d fragments (removed)")
	}

	// Must touch ready file
	if !strings.Contains(script, ReadyFile) {
		t.Errorf("must touch ReadyFile (%s)", ReadyFile)
	}

	// Must write DEVCELL_BOOT_DIR to envdir so s6 services can signal boot progress
	if !strings.Contains(script, "DEVCELL_BOOT_DIR") || !strings.Contains(script, "$S6_ENV/DEVCELL_BOOT_DIR") {
		t.Error("must write DEVCELL_BOOT_DIR to s6 envdir so services can signal boot progress")
	}

	// Must notify boot progress
	if !strings.Contains(script, "_notify") {
		t.Error("must use _notify for boot progress signaling")
	}
	if !strings.Contains(script, "boot.ready") {
		t.Error("must signal boot.ready on completion")
	}

	// Must fix file ownership after service activation
	if !strings.Contains(script, "chown") {
		t.Error("must fix ownership of root-created files")
	}

	// Must make /etc/passwd writable before useradd
	chmodPos := strings.Index(script, "chmod")
	if chmodPos < 0 || !strings.Contains(script, "/etc/passwd") {
		t.Error("must chmod /etc/passwd writable before useradd")
	}

	// Must create session user before gosu
	if !strings.Contains(script, "useradd") {
		t.Error("must create session user inline")
	}
	if chmodPos >= 0 {
		useraddPos := strings.Index(script, "useradd")
		if useraddPos >= 0 && chmodPos > useraddPos {
			t.Error("chmod /etc/passwd must precede useradd")
		}
	}

	// User creation must come before gosu exec
	useraddPos := strings.Index(script, "useradd")
	gosuPos := strings.Index(script, "exec gosu")
	if useraddPos < 0 || gosuPos < 0 || useraddPos > gosuPos {
		t.Error("user creation must precede gosu exec")
	}

	// Must exec into CMD via gosu
	if !strings.Contains(script, "exec gosu") {
		t.Error("must exec into CMD via gosu")
	}
	if !strings.Contains(script, `"$@"`) {
		t.Error("must pass through CMD args via $@")
	}
}

func TestEntrypointSnippet_VsActivateScript_NoDuplicateSudo(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	snippet := EntrypointSnippet(env)
	activate := ActivateScript(env)

	if !strings.Contains(activate, "sudo") {
		t.Error("ActivateScript should use sudo")
	}
	for _, line := range strings.Split(snippet, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sudo ") {
			t.Errorf("EntrypointSnippet must not invoke sudo: %s", trimmed)
		}
	}
}

// --- ActivateScript tests ---

func TestActivateScript(t *testing.T) {
	env := SessionEnv{
		HostUser:    "dmitry",
		SessionHome: "/Users/dmitry",
		DevcellHome: "/Users/devcell",
	}
	script := ActivateScript(env)

	// Must source nix-daemon profile
	if !strings.Contains(script, "nix-daemon.sh") {
		t.Error("must source nix-daemon profile")
	}

	// Must write env vars
	for _, v := range []string{"HOST_USER", "SESSION_HOME", "DEVCELL_HOME"} {
		if !strings.Contains(script, v) {
			t.Errorf("must set %s", v)
		}
	}

	// Must use s6-svscan for supervision
	if !strings.Contains(script, "s6-svscan") {
		t.Error("must start s6-svscan supervision tree")
	}

	// Must use s6-rc-init
	if !strings.Contains(script, "s6-rc-init") {
		t.Error("must run s6-rc-init")
	}

	// Must use s6-rc change user
	if !strings.Contains(script, "s6-rc") && !strings.Contains(script, "change user") {
		t.Error("must use s6-rc change user to bring up services")
	}

	// Must NOT use setsid
	if strings.Contains(script, "setsid") {
		t.Error("must not use setsid: s6-svscan handles supervision")
	}

	// Must NOT use custom topo sort
	if strings.Contains(script, "topo_order") {
		t.Error("must not use custom topo sort")
	}

	// Must run via sudo (non-root context)
	if !strings.Contains(script, "sudo") {
		t.Error("service activation must use sudo")
	}

	// Must touch ready file
	if !strings.Contains(script, ReadyFile) {
		t.Errorf("must touch ReadyFile (%s)", ReadyFile)
	}

	// Must be gracefully degraded
	if !strings.Contains(script, "s6-rc") || !strings.Contains(script, "skipping") {
		t.Error("must gracefully skip when s6-rc is not available")
	}
}

func TestActivateScript_ExportsEnvBeforeActivation(t *testing.T) {
	env := SessionEnv{
		HostUser:    "dmitry",
		SessionHome: "/Users/dmitry",
		DevcellHome: "/Users/devcell",
	}
	script := ActivateScript(env)

	envPos := strings.Index(script, "HOST_USER")
	svscanPos := strings.Index(script, "s6-svscan")
	if envPos < 0 || svscanPos < 0 {
		t.Fatal("script must have both env writes and s6-svscan")
	}
	if envPos > svscanPos {
		t.Error("env vars must be written before s6-svscan start")
	}
}

func TestTartDelegatesToDarwinSessionEnv(t *testing.T) {
	env := DarwinSessionEnv("admin", "devcell")
	script := ActivateScript(env)
	if !strings.Contains(script, "/Users/admin") {
		t.Error("Darwin script must use /Users/<cellUser> for SessionHome")
	}
	if !strings.Contains(script, "/Users/devcell") {
		t.Error("Darwin script must use /Users/<vmUser> for DevcellHome")
	}
}

func TestEntrypointSnippet_S6BootSequenceOrder(t *testing.T) {
	env := LinuxSessionEnv("dmitry")
	script := EntrypointSnippet(env)

	svscanPos := strings.Index(script, "s6-svscan")
	initPos := strings.Index(script, "s6-rc-init")
	changePos := strings.Index(script, "s6-rc -l")
	gosuPos := strings.Index(script, "exec gosu")

	if svscanPos < 0 || initPos < 0 || changePos < 0 || gosuPos < 0 {
		t.Fatal("script must contain s6-svscan, s6-rc-init, s6-rc change, exec gosu")
	}

	if svscanPos >= initPos {
		t.Error("s6-svscan must start before s6-rc-init")
	}
	if initPos >= changePos {
		t.Error("s6-rc-init must run before s6-rc change")
	}
	if changePos >= gosuPos {
		t.Error("s6-rc change must complete before exec gosu")
	}
}

func TestActivateScript_S6BootSequenceOrder(t *testing.T) {
	env := DarwinSessionEnv("dmitry", "devcell")
	script := ActivateScript(env)

	svscanPos := strings.Index(script, "s6-svscan")
	initPos := strings.Index(script, "s6-rc-init")
	changePos := strings.Index(script, "s6-rc -l")

	if svscanPos < 0 || initPos < 0 || changePos < 0 {
		t.Fatal("script must contain s6-svscan, s6-rc-init, s6-rc change")
	}

	if svscanPos >= initPos {
		t.Error("s6-svscan must start before s6-rc-init")
	}
	if initPos >= changePos {
		t.Error("s6-rc-init must run before s6-rc change")
	}
}
