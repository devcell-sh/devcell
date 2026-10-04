package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// binaryPath returns the path where the test binary is built.
// Tests build it once via TestMain.
var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cell-smoke-*")
	if err != nil {
		panic(err)
	}

	binaryPath = tmp + "/cell"
	out, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput()
	if err != nil {
		os.RemoveAll(tmp)
		panic("go build failed: " + string(out))
	}

	code := m.Run()
	os.RemoveAll(tmp) // os.Exit skips defers
	os.Exit(code)
}

func TestHelp(t *testing.T) {
	out, err := exec.Command(binaryPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("expected 'Usage:' in --help output, got:\n%s", out)
	}
}

func TestVersion(t *testing.T) {
	out, err := exec.Command(binaryPath, "--version").Output()
	if err != nil {
		t.Fatalf("--version exited non-zero: %v", err)
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Error("--version produced empty output")
	}
}

func TestUnknownSubcommand(t *testing.T) {
	cmd := exec.Command(binaryPath, "definitely-not-a-command")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected non-zero exit for unknown subcommand")
	}
	if !strings.Contains(string(out), `unknown command "definitely-not-a-command"`) {
		t.Errorf("expected unknown command error, got:\n%s", out)
	}
}

func TestDebugFlagInHelp(t *testing.T) {
	out, err := exec.Command(binaryPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "--debug") {
		t.Errorf("--debug flag not found in --help output:\n%s", out)
	}
}

func TestPlainTextFlagInHelp(t *testing.T) {
	out, err := exec.Command(binaryPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "--plain-text") {
		t.Errorf("--plain-text flag not found in --help output:\n%s", out)
	}
}

// scaffoldedHome creates a temp HOME with global devcell.toml and a project-level
// .devcell.toml so the CLI skips the first-run interactive prompt.
// Returns home path. The home dir doubles as a project root (has .devcell.toml)
// — callers that run agent subcommands must set cmd.Dir = home.
func scaffoldedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	cfgDir := home + "/.config/devcell"
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Global config (loaded by cfg.LoadFromOS as globalPath)
	if err := os.WriteFile(cfgDir+"/devcell.toml", []byte("[cell]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Project-level config (checked by scaffold.IsInitialized via cwd)
	if err := os.WriteFile(home+"/.devcell.toml", []byte("[cell]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return home
}

// hermeticDryRun runs `cell <args...> --dry-run` in a fresh project whose
// .devcell.toml is projectTOML and whose only git config is gitconfig. The
// developer's own GIT_*, locale, timezone, cell name and XDG config dir are
// stripped from the environment so they cannot leak into the guest env.
func hermeticDryRun(t *testing.T, projectTOML, gitconfig string, extraEnv []string, args ...string) string {
	t.Helper()
	home := scaffoldedHome(t)
	if err := os.WriteFile(home+"/.devcell.toml", []byte(projectTOML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home+"/.gitconfig", []byte(gitconfig), 0644); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(k, "GIT_"), k == "XDG_CONFIG_HOME", k == "HOME",
			k == "DEVCELL_CELL_NAME", k == "TMUX_SESSION_NAME", k == "TZ", k == "LANG", k == "LC_ALL":
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+home, "DEVCELL_BUNK=1",
		"GIT_CONFIG_GLOBAL="+home+"/.gitconfig", "GIT_CONFIG_NOSYSTEM=1")
	cmd := exec.Command(binaryPath, append(args, "--dry-run")...)
	cmd.Dir = home
	cmd.Env = append(env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cell %v --dry-run: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

// guestEnvTOML exercises [env] (with host expansion) and [mise].
const guestEnvTOML = `[cell]
[env]
FROM_TOML_ENV = "plain"
EXPANDED = "${DEVCELL_TEST_HOST_VALUE}"
[mise]
trusted_config_paths = "/"
`

const guestEnvGitconfig = "[user]\n\tname = Hopper\n\temail = hopper@example.com\n"

// assertGuestEnvDryRun checks that a VM engine's dry-run hands the guest the
// unified cell env (internal/cell.GuestEnv): [env], [mise], cell name,
// IS_SANDBOX, LANG/LC_ALL and the git identity precedence
// host env > [git] > host git config > DevCell default.
func assertGuestEnvDryRun(t *testing.T, engineArgs ...string) {
	t.Helper()
	run := func(t *testing.T, toml string, extraEnv ...string) string {
		t.Helper()
		env := append([]string{"DEVCELL_TEST_HOST_VALUE=expanded-ok"}, extraEnv...)
		args := append(append([]string{}, engineArgs...), "--cell-name=flagcell", "shell")
		return hermeticDryRun(t, toml, guestEnvGitconfig, env, args...)
	}
	mustContain := func(t *testing.T, out string, wants ...string) {
		t.Helper()
		for _, w := range wants {
			if !strings.Contains(out, w) {
				t.Errorf("expected %q in dry-run output, got:\n%s", w, out)
			}
		}
	}

	t.Run("env mise locale cell name", func(t *testing.T) {
		out := run(t, guestEnvTOML)
		mustContain(t, out,
			"FROM_TOML_ENV=plain",
			"EXPANDED=expanded-ok",
			"MISE_TRUSTED_CONFIG_PATHS=/",
			"DEVCELL_CELL_NAME=flagcell",
			"IS_SANDBOX=1",
			"LANG=en_US.UTF-8",
			"LC_ALL=en_US.UTF-8",
		)
	})
	t.Run("git config when no env and no [git]", func(t *testing.T) {
		out := run(t, guestEnvTOML)
		mustContain(t, out, "GIT_AUTHOR_NAME=Hopper", "GIT_COMMITTER_EMAIL=hopper@example.com")
	})
	t.Run("[git] beats git config", func(t *testing.T) {
		out := run(t, guestEnvTOML+"[git]\nauthor_name = \"TomlName\"\nauthor_email = \"toml@example.com\"\n")
		mustContain(t, out, "GIT_AUTHOR_NAME=TomlName", "GIT_COMMITTER_EMAIL=toml@example.com")
		if strings.Contains(out, "Hopper") {
			t.Errorf("git config identity leaked past [git]:\n%s", out)
		}
	})
	t.Run("host env beats [git]", func(t *testing.T) {
		out := run(t, guestEnvTOML+"[git]\nauthor_name = \"TomlName\"\n", "GIT_AUTHOR_NAME=EnvName")
		mustContain(t, out, "GIT_AUTHOR_NAME=EnvName", "GIT_AUTHOR_EMAIL=devcell@devcell.io")
		if strings.Contains(out, "TomlName") || strings.Contains(out, "Hopper") {
			t.Errorf("lower-tier identity leaked past host env:\n%s", out)
		}
	})
}

// shellWords splits line the way sh would, via printf.
func shellWords(t *testing.T, line string) []string {
	t.Helper()
	out, err := exec.Command("sh", "-c", `printf '%s\0' `+line).Output()
	if err != nil {
		t.Fatalf("sh cannot parse %q: %v", line, err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

// TestPlainTextNoSpinnerChars verifies that --plain-text suppresses spinner
// Unicode sequences. We run with --dry-run to avoid docker exec but still
// exercise the pre-exec ux output path.
func TestPlainTextNoSpinnerChars(t *testing.T) {
	spinnerChars := []string{"⡀", "⢀", "⠄", "⠠", "⠐", "⠂", "⠁", "⠈"}

	home := scaffoldedHome(t)
	cmd := exec.Command(binaryPath, "--plain-text", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(),
		"DEVCELL_BUNK=1",
		"HOME="+home,
	)
	out, _ := cmd.CombinedOutput()
	s := string(out)
	for _, ch := range spinnerChars {
		if strings.Contains(s, ch) {
			t.Errorf("spinner char %q found in --plain-text output:\n%s", ch, s)
		}
	}
}

// TestDebugNoSpinnerChars verifies --debug also suppresses spinners.
func TestDebugNoSpinnerChars(t *testing.T) {
	spinnerChars := []string{"⡀", "⢀", "⠄", "⠠", "⠐", "⠂", "⠁", "⠈"}

	home := scaffoldedHome(t)
	cmd := exec.Command(binaryPath, "--debug", "shell", "--dry-run")
	cmd.Dir = home
	cmd.Env = append(os.Environ(),
		"DEVCELL_BUNK=1",
		"HOME="+home,
	)
	out, _ := cmd.CombinedOutput()
	s := string(out)
	for _, ch := range spinnerChars {
		if strings.Contains(s, ch) {
			t.Errorf("spinner char %q found in --debug output:\n%s", ch, s)
		}
	}
}
