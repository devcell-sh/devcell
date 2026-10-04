package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// initFlowConfig is the TOML structure for reading back .devcell.toml.
type initFlowConfig struct {
	Cell struct {
		Stack   string   `toml:"stack"`
		Modules []string `toml:"modules"`
	} `toml:"cell"`
}

func readToml(t *testing.T, path string) initFlowConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var c initFlowConfig
	if _, err := toml.Decode(string(data), &c); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return c
}

// TestInitFlow_NonInteractive_DefaultsToBase verifies that -y mode
// produces a base stack with no modules and no interactive prompts.
func TestInitFlow_NonInteractive_DefaultsToBase(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("expected stack=base, got %q", result.Stack)
	}
	if len(result.Modules) != 0 {
		t.Errorf("expected no modules, got %v", result.Modules)
	}
	// .devcell.toml should exist with stack = "base"
	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("toml stack: expected base, got %q", c.Cell.Stack)
	}
}

// TestInitFlow_ExplicitStack verifies the --stack flag value is used as-is.
func TestInitFlow_ExplicitStack(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "go",
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "go" {
		t.Errorf("expected stack=go, got %q", result.Stack)
	}
}

// TestInitFlow_CreatesAllBuildArtifacts verifies scaffold output.
func TestInitFlow_CreatesAllBuildArtifacts(t *testing.T) {
	dir := t.TempDir()
	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "go",
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	for _, f := range []string{".devcell.toml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	for _, f := range []string{"flake.nix"} {
		if _, err := os.Stat(filepath.Join(dir, ".devcell", f)); err != nil {
			t.Errorf("missing .devcell/%s: %v", f, err)
		}
	}
	for _, f := range []string{"Dockerfile", "package.json", "pyproject.toml"} {
		if _, err := os.Stat(filepath.Join(dir, ".devcell", f)); err == nil {
			t.Errorf(".devcell/%s should NOT be created (dead build path removed)", f)
		}
	}
}

// TestInitFlow_LocalNixhome_CopiedToBuildDir verifies local nixhome is synced.
func TestInitFlow_LocalNixhome_CopiedToBuildDir(t *testing.T) {
	dir := t.TempDir()
	// Create a fake local nixhome with a marker file.
	nixhome := filepath.Join(t.TempDir(), "nixhome")
	os.MkdirAll(filepath.Join(nixhome, "stacks"), 0755)
	os.MkdirAll(filepath.Join(nixhome, "modules"), 0755)
	os.WriteFile(filepath.Join(nixhome, "marker.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(nixhome, "stacks", "base.nix"), []byte("{ imports = [ ../modules/base.nix ]; }"), 0644)
	os.WriteFile(filepath.Join(nixhome, "modules", "base.nix"), []byte("{}"), 0644)

	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:    dir,
		ConfigDir:  filepath.Join(dir, ".config", "devcell"),
		NixhomeSrc: nixhome,
		Stack:      "base",
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	// Marker file should be in .devcell/nixhome/
	marker := filepath.Join(dir, ".devcell", "nixhome", "marker.txt")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("nixhome not synced to build dir: %v", err)
	}
}

// TestInitFlow_FlakeUsesPathNixhome verifies flake.nix uses path:./nixhome
// when nixhome is present in build dir.
func TestInitFlow_FlakeUsesPathNixhome(t *testing.T) {
	dir := t.TempDir()
	// Create a fake local nixhome.
	nixhome := filepath.Join(t.TempDir(), "nixhome")
	os.MkdirAll(filepath.Join(nixhome, "stacks"), 0755)
	os.MkdirAll(filepath.Join(nixhome, "modules"), 0755)
	os.WriteFile(filepath.Join(nixhome, "stacks", "base.nix"), []byte("{ imports = [ ../modules/base.nix ]; }"), 0644)
	os.WriteFile(filepath.Join(nixhome, "modules", "base.nix"), []byte("{}"), 0644)

	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:    dir,
		ConfigDir:  filepath.Join(dir, ".config", "devcell"),
		NixhomeSrc: nixhome,
		Stack:      "base",
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	flake, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if !strings.Contains(string(flake), `"path:./nixhome"`) {
		t.Errorf("flake.nix should use path:./nixhome, got:\n%s", flake)
	}
}

// TestInitFlow_ReturnsBuildDir verifies the result includes the build dir path.
func TestInitFlow_ReturnsBuildDir(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	expected := filepath.Join(dir, ".devcell")
	if result.BuildDir != expected {
		t.Errorf("expected BuildDir=%s, got %s", expected, result.BuildDir)
	}
}

// TestInitFlow_ModulesWrittenToToml verifies that when ScaffoldWithModules
// is called with explicit modules, they appear in .devcell.toml.
func TestInitFlow_ModulesWrittenToToml(t *testing.T) {
	dir := t.TempDir()

	// Simulate what happens when user customizes: stack=base + modules.
	nixhome := filepath.Join(t.TempDir(), "nixhome")
	os.MkdirAll(filepath.Join(nixhome, "stacks"), 0755)
	os.MkdirAll(filepath.Join(nixhome, "modules"), 0755)
	os.WriteFile(filepath.Join(nixhome, "stacks", "base.nix"), []byte("{ imports = [ ../modules/base.nix ]; }"), 0644)
	os.WriteFile(filepath.Join(nixhome, "modules", "base.nix"), []byte("{}"), 0644)

	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:    dir,
		ConfigDir:  filepath.Join(dir, ".config", "devcell"),
		NixhomeSrc: nixhome,
		Stack:      "base",
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	// RunInitFlow with Yes=true and Stack="base" won't set modules.
	// We need to test ScaffoldWithModules directly with modules.
	_ = result

	// Now test scaffold with explicit modules.
	modules := []string{"go", "electronics"}
	err = ScaffoldWithModules(dir, "", nixhome, true, "base", modules)
	if err != nil {
		t.Fatalf("ScaffoldWithModules: %v", err)
	}
	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("expected stack=base, got %q", c.Cell.Stack)
	}
	if len(c.Cell.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %v", c.Cell.Modules)
	}
	if c.Cell.Modules[0] != "go" || c.Cell.Modules[1] != "electronics" {
		t.Errorf("expected [go electronics], got %v", c.Cell.Modules)
	}
}

// TestInitFlow_ExplicitModules verifies --modules flag writes to .devcell.toml.
func TestInitFlow_ExplicitModules(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Modules:   []string{"go", "infra", "electronics"},
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("expected stack=base, got %q", result.Stack)
	}
	if len(result.Modules) != 3 {
		t.Fatalf("expected 3 modules in result, got %v", result.Modules)
	}

	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("toml stack: expected base, got %q", c.Cell.Stack)
	}
	if len(c.Cell.Modules) != 3 {
		t.Fatalf("toml modules: expected 3, got %v", c.Cell.Modules)
	}
	for i, want := range []string{"go", "infra", "electronics"} {
		if c.Cell.Modules[i] != want {
			t.Errorf("toml modules[%d]: expected %q, got %q", i, want, c.Cell.Modules[i])
		}
	}
}

// TestInitFlow_ModulesImplyBaseStack verifies --modules without --stack defaults to base.
func TestInitFlow_ModulesImplyBaseStack(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Modules:   []string{"go", "node"},
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("expected stack=base when modules explicit, got %q", result.Stack)
	}
}

// TestInitFlow_ForceOverwrites verifies --force overwrites existing files.
func TestInitFlow_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	// First run
	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Yes:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Write sentinel to flake.nix
	sentinel := "# SENTINEL\n"
	os.WriteFile(filepath.Join(dir, ".devcell", "flake.nix"), []byte(sentinel), 0644)

	// Second run with force
	_, err = RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "go",
		Yes:       true,
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if string(data) == sentinel {
		t.Error("force should overwrite existing flake.nix")
	}
}

// --- Behavioral scenario tests ---
// These verify init flow scaffolding + nix image selection for 4 common scenarios.

// TestInitScenario_NewDirectory verifies init in a fresh directory defaults to
// base stack and FlakeNixImage is decoupled from binary version.
func TestInitScenario_NewDirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("new dir: expected stack=base, got %q", result.Stack)
	}
	if len(result.Modules) != 0 {
		t.Errorf("new dir: expected no modules, got %v", result.Modules)
	}
	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("new dir toml: expected stack=base, got %q", c.Cell.Stack)
	}

	flake, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if !strings.Contains(string(flake), "devcell.stacks.base") {
		t.Errorf("new dir flake.nix should reference devcell.stacks.base, got:\n%s", flake)
	}
}

// TestInitScenario_ExistingToml_Ultimate verifies force re-init with stack=ultimate.
func TestInitScenario_ExistingToml_Ultimate(t *testing.T) {
	dir := t.TempDir()
	// First init with base.
	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Re-init with ultimate (force).
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "ultimate",
		Yes:       true,
		Force:     true,
	})
	if err != nil {
		t.Fatalf("re-init ultimate: %v", err)
	}
	if result.Stack != "ultimate" {
		t.Errorf("re-init: expected stack=ultimate, got %q", result.Stack)
	}
	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "ultimate" {
		t.Errorf("re-init toml: expected stack=ultimate, got %q", c.Cell.Stack)
	}

	flake, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if !strings.Contains(string(flake), "devcell.stacks.ultimate") {
		t.Errorf("re-init flake.nix should reference devcell.stacks.ultimate, got:\n%s", flake)
	}
}

// TestInitScenario_ExistingToml_BaseWithDesktopModule verifies force re-init
// with base stack + desktop module.
func TestInitScenario_ExistingToml_BaseWithDesktopModule(t *testing.T) {
	dir := t.TempDir()
	// First init.
	_, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Re-init with base + desktop module.
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Modules:   []string{"desktop"},
		Yes:       true,
		Force:     true,
	})
	if err != nil {
		t.Fatalf("re-init base+desktop: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("expected stack=base, got %q", result.Stack)
	}
	if len(result.Modules) != 1 || result.Modules[0] != "desktop" {
		t.Errorf("expected modules=[desktop], got %v", result.Modules)
	}

	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("toml: expected stack=base, got %q", c.Cell.Stack)
	}
	if len(c.Cell.Modules) != 1 || c.Cell.Modules[0] != "desktop" {
		t.Errorf("toml: expected modules=[desktop], got %v", c.Cell.Modules)
	}

	flake, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	flakeStr := string(flake)
	if !strings.Contains(flakeStr, "devcell.stacks.base") {
		t.Errorf("flake.nix should reference devcell.stacks.base")
	}
	if !strings.Contains(flakeStr, "devcell.modules.desktop") {
		t.Errorf("flake.nix should reference devcell.modules.desktop, got:\n%s", flakeStr)
	}
}

// TestInitScenario_NewDirectory_ExplicitBase verifies cell init --stack=base in a new dir.
func TestInitScenario_NewDirectory_ExplicitBase(t *testing.T) {
	dir := t.TempDir()
	result, err := RunInitFlow(InitFlowOptions{
		BaseDir:   dir,
		ConfigDir: filepath.Join(dir, ".config", "devcell"),
		Stack:     "base",
		Yes:       true,
	})
	if err != nil {
		t.Fatalf("RunInitFlow: %v", err)
	}
	if result.Stack != "base" {
		t.Errorf("expected stack=base, got %q", result.Stack)
	}

	c := readToml(t, filepath.Join(dir, ".devcell.toml"))
	if c.Cell.Stack != "base" {
		t.Errorf("toml: expected stack=base, got %q", c.Cell.Stack)
	}

	flake, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if !strings.Contains(string(flake), "devcell.stacks.base") {
		t.Errorf("flake.nix should reference devcell.stacks.base")
	}
}
