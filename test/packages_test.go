package container_test

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DimmKirr/devcell/internal/testutil"
)

// TestPackages_MiseInstallsToolsAtStart builds a real image with `cell build`
// and starts it with `cell shell`, proving [packages.python] and
// [packages.node] tools are installed by mise at container start and run.
//
// Uses the nixhome checkout from nixhomeDir() (DEVCELL_NIXHOME), which must
// contain modules/fragments/11-mise-packages.sh.
func TestPackages_MiseInstallsToolsAtStart(t *testing.T) {
	if testing.Short() {
		t.Skip("long: builds its own image with `cell build` (~10 min)")
	}
	nixhome, err := filepath.Abs(nixhomeDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(nixhome, "modules", "fragments", "11-mise-packages.sh")); err != nil {
		t.Skipf("nixhome at %s lacks 11-mise-packages.sh; set DEVCELL_NIXHOME to a checkout that has it", nixhome)
	}
	cellBin, err := ensureCellBinary()
	if err != nil {
		t.Fatal(err)
	}

	// Docker-accessible dirs (bind-mounted by the daemon host).
	base := testutil.TestResultsDir(t, hostBaseDirFn)
	projectDir := filepath.Join(base, "packages-project")
	configDir := filepath.Join(base, "config")
	home := filepath.Join(base, "home")
	for _, d := range []string{projectDir, filepath.Join(configDir, "devcell"),
		filepath.Join(home, ".claude", "commands"), filepath.Join(home, ".claude", "agents"), filepath.Join(home, ".claude", "skills")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		osexec.Command("docker", "run", "--rm", "-v", home+":"+home, "alpine", "rm", "-rf", home).Run()
	})
	toml := fmt.Sprintf(`[cell]
stack = "base"
modules = ["python", "node"]

[nix]
nixhome = %q

[packages.python]
"pyjokes" = "latest"

[packages.node]
"semver" = "7"
`, nixhome)
	if err := os.WriteFile(filepath.Join(projectDir, ".devcell.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEVCELL_CELL_NAME", "pkgtest")

	tag := fmt.Sprintf("devcell-user:pkgtest-%s-%d", shortSHA(), time.Now().Unix())
	build := osexec.Command(cellBin, "build", "--image", tag, "--debug")
	build.Dir = projectDir
	build.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+configDir)
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("cell build: %v", err)
	}
	t.Cleanup(func() { osexec.Command("docker", "rmi", "-f", tag).Run() })

	script := `for b in pyjokes semver; do printf '%s=' "$b"; command -v "$b" || echo MISSING; done
echo "semver-run=$(semver 1.2.3 2>&1)"
echo "pyjokes-run=$(pyjokes >/dev/null 2>&1 && echo ok || echo fail)"
echo "conf=$(cat /etc/mise/conf.d/devcell-packages.toml 2>&1 | tr '\n' ' ')"`
	out := runCellShell(t, cellBin, projectDir, configDir, home, tag, "--debug", "shell", "--", "bash", "-lc", script)
	t.Logf("cell shell output:\n%s", out)

	for _, want := range []string{
		`conf=[tools] "npm:semver" = "7" "pipx:pyjokes" = "latest"`,
		"semver-run=1.2.3",
		"pyjokes-run=ok",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
	for _, bin := range []string{"pyjokes", "semver"} {
		if strings.Contains(out, bin+"=MISSING") {
			t.Errorf("%s is not on PATH", bin)
		}
	}
}
