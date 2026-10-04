package scaffold_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/nixhome"
	"github.com/DimmKirr/devcell/internal/scaffold"
)

func TestScaffold_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}
	// .devcell.toml in project root
	if _, err := os.Stat(filepath.Join(dir, ".devcell.toml")); err != nil {
		t.Errorf("missing .devcell.toml in project root: %v", err)
	}
	// Build artifacts in .devcell/ subdir
	for _, name := range []string{"flake.nix"} {
		if _, err := os.Stat(filepath.Join(dir, ".devcell", name)); err != nil {
			t.Errorf("missing %s in .devcell/: %v", name, err)
		}
	}
}

func TestScaffold_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	// Overwrite flake.nix with sentinel content
	sentinel := "# SENTINEL CONTENT\n"
	if err := os.WriteFile(filepath.Join(dir, ".devcell", "flake.nix"), []byte(sentinel), 0644); err != nil {
		t.Fatal(err)
	}
	// Scaffold again — must not overwrite
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != sentinel {
		t.Error("Scaffold overwrote existing flake.nix — should be idempotent")
	}
}

// TestScaffold_FlakeNixUsesGitHubURL — user flake must fetch nixhome from
// GitHub (not path:/opt/nixhome), so users can point to any nixhome source.
func TestScaffold_FlakeNixUsesGitHubURL(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	s := string(data)
	if !strings.Contains(s, "github:") {
		t.Errorf("flake.nix must use github: URL, got:\n%s", s)
	}
	if strings.Contains(s, "path:/opt/nixhome") {
		t.Errorf("flake.nix must NOT use path:/opt/nixhome (couples to base image internals), got:\n%s", s)
	}
}

func TestScaffold_DevcellTomlIsValidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	var v interface{}
	if _, err := toml.Decode(string(data), &v); err != nil {
		t.Errorf(".devcell.toml is not valid TOML: %v\ncontent:\n%s", err, string(data))
	}
}

func TestScaffold_FlakeNixContainsUpstreamURL(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	if !strings.Contains(string(data), nixhome.UpstreamOwner+"/"+nixhome.UpstreamRepo) {
		t.Errorf("flake.nix should reference %s/%s, got:\n%s", nixhome.UpstreamOwner, nixhome.UpstreamRepo, string(data))
	}
}

func TestScaffold_FlakeNixVersionSubstituted(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	s := string(data)
	if strings.Contains(s, "{{VERSION}}") {
		t.Errorf("unreplaced {{VERSION}} placeholder in flake.nix:\n%s", s)
	}
	// v0.0.0 (dev build) coerces to DefaultNixhomeGitRef via nixhome.UpstreamFlakeRef
	// — literal v0.0.0 would 404 against github (no such tag).
	want := nixhome.UpstreamOwner + "/" + nixhome.UpstreamRepo + "/" + nixhome.DefaultNixhomeGitRef
	if !strings.Contains(s, want) {
		t.Errorf("flake.nix should contain coerced upstream URL %q, got:\n%s", want, s)
	}
}

// --- Scaffold with models snippet ---

func TestScaffold_WithModelsSnippet_InjectsIntoToml(t *testing.T) {
	dir := t.TempDir()
	snippet := "# [llm]\n# provider = \"ollama\"\n# model = \"deepseek-r1:70b\"\n#\n# [llm.providers.ollama]\n# models = [\"deepseek-r1:70b\", \"qwen3:32b\"]\n"
	if err := scaffold.Scaffold(dir, snippet, "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	s := string(data)
	if !strings.Contains(s, "deepseek-r1:70b") {
		t.Errorf("expected detected models in .devcell.toml, got:\n%s", s)
	}
	if !strings.Contains(s, "qwen3:32b") {
		t.Errorf("expected qwen3:32b in devcell.toml, got:\n%s", s)
	}
}

func TestScaffold_EmptySnippet_ShowsCommentedLLMReference(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	s := string(data)
	for _, want := range []string{"# [llm]", "# [llm.providers.ollama]"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in .devcell.toml, got:\n%s", want, s)
		}
	}
	if strings.Contains(s, "{{MODELS_SECTION}}") {
		t.Error("placeholder must be removed when no models are detected")
	}
}

func TestScaffold_WithSnippet_StillValidTOML(t *testing.T) {
	dir := t.TempDir()
	snippet := "# [llm]\n# provider = \"ollama\"\n# model = \"deepseek-r1:70b\"\n#\n# [llm.providers.ollama]\n# models = [\"deepseek-r1:70b\", \"qwen3:32b\"]\n"
	if err := scaffold.Scaffold(dir, snippet, "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	var v interface{}
	if _, err := toml.Decode(string(data), &v); err != nil {
		t.Errorf(".devcell.toml is not valid TOML: %v\ncontent:\n%s", err, string(data))
	}
}

// --- DEVCELL_NIXHOME_PATH support ---

// TestScaffold_WithNixhomePath_FlakeUsesPathInput — when nixhomePath is set,
// flake.nix must use path:./nixhome instead of GitHub URL.
func TestScaffold_WithNixhomePath_FlakeUsesPathInput(t *testing.T) {
	dir := t.TempDir()
	// Create a fake nixhome source so SyncNixhome succeeds.
	fakeNixhome := t.TempDir()
	os.WriteFile(filepath.Join(fakeNixhome, "flake.nix"), []byte("# fake"), 0644)
	if err := scaffold.Scaffold(dir, "", fakeNixhome, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	s := string(data)
	if !strings.Contains(s, `inputs.devcell.url = "path:./nixhome"`) {
		t.Errorf("flake.nix must have inputs.devcell.url = path:./nixhome when nixhomePath is set, got:\n%s", s)
	}
	// The active (non-comment) URL line must not be a github: URL.
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "inputs.devcell.url") && strings.Contains(trimmed, "github:") {
			t.Errorf("active inputs.devcell.url must not use github: when nixhomePath is set, got line: %s", trimmed)
		}
	}
}

// TestSyncNixhome_CopiesDirectory — SyncNixhome copies nixhome dir into configDir/nixhome/.
func TestSyncNixhome_CopiesDirectory(t *testing.T) {
	// Create a fake nixhome source with a marker file
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "flake.nix"), []byte("# nixhome flake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "modules", "base.nix"), []byte("# base"), 0644); err != nil {
		t.Fatal(err)
	}

	configDir := t.TempDir()
	if err := scaffold.SyncNixhome(srcDir, configDir); err != nil {
		t.Fatalf("SyncNixhome failed: %v", err)
	}

	// Verify files were copied
	dest := filepath.Join(configDir, "nixhome", "flake.nix")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", dest, err)
	}
	if string(data) != "# nixhome flake" {
		t.Errorf("expected copied flake.nix content, got: %s", string(data))
	}

	// Verify subdirectory was copied
	subDest := filepath.Join(configDir, "nixhome", "modules", "base.nix")
	if _, err := os.Stat(subDest); err != nil {
		t.Errorf("expected %s to exist: %v", subDest, err)
	}
}

// A nixhome checkout that is also opened as a cell project has its own
// .devcell/ build dir, holding a nested git repo with no commits. Copying it
// broke the `git add` that makes the flake visible to nix.
func TestSyncNixhome_SkipsSourceBuildDir(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "flake.nix"), []byte("# nixhome flake"), 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(srcDir, ".devcell", "nixhome")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", "-q", nested).Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}

	configDir := t.TempDir()
	if err := scaffold.SyncNixhome(srcDir, configDir); err != nil {
		t.Fatalf("SyncNixhome failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "nixhome", ".devcell")); !os.IsNotExist(err) {
		t.Errorf("the source's .devcell/ must not be copied (stat err: %v)", err)
	}
}

// TestSyncNixhome_ErrorOnMissingPath — SyncNixhome returns error for non-existent source.
func TestSyncNixhome_ErrorOnMissingPath(t *testing.T) {
	configDir := t.TempDir()
	err := scaffold.SyncNixhome("/nonexistent/nixhome", configDir)
	if err == nil {
		t.Error("expected error for non-existent nixhome path, got nil")
	}
}

// TestSyncNixhome_RejectsSelfReferentialSource — SyncNixhome errors clearly
// (instead of corrupting configDir/nixhome via a partial recursive copy) when
// srcPath is the project directory itself, i.e. configDir/nixhome would be
// nested inside srcPath. Regression test for a real incident: pointing
// DEVCELL_NIXHOME at a project's own root caused CopyDir to walk into the
// destination it was writing, die on a dangling entrypoint.sh symlink
// mid-copy, and leave the project's .devcell/ permanently corrupted for
// every subsequent build referencing that source.
func TestSyncNixhome_RejectsSelfReferentialSource(t *testing.T) {
	projectDir := t.TempDir()
	configDir := filepath.Join(projectDir, ".devcell")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Give the project dir some pre-existing build artifacts, mirroring the
	// real .devcell/ layout (entrypoint.sh symlinked into nixhome/).
	if err := os.WriteFile(filepath.Join(configDir, "cell.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	err := scaffold.SyncNixhome(projectDir, configDir)
	if err == nil {
		t.Fatal("expected SyncNixhome to reject a self-referential source, got nil error")
	}
	if !strings.Contains(err.Error(), "own build directory") {
		t.Errorf("expected a self-reference error, got: %v", err)
	}

	// Nothing should have been touched — no partial/corrupted nixhome dir.
	if _, statErr := os.Stat(filepath.Join(configDir, "nixhome")); !os.IsNotExist(statErr) {
		t.Errorf("expected no nixhome dir to be created on rejection, stat err: %v", statErr)
	}
	// The pre-existing artifact must survive untouched.
	if _, statErr := os.Stat(filepath.Join(configDir, "cell.json")); statErr != nil {
		t.Errorf("expected pre-existing cell.json to survive, got: %v", statErr)
	}
}

// TestCopyDir_SkipsDestinationNestedInSource — even if a caller bypasses the
// SyncNixhome-level guard and calls CopyDir directly with dst nested inside
// src, CopyDir must not recurse into (or corrupt) its own output.
func TestCopyDir_SkipsDestinationNestedInSource(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "flake.nix"), []byte("# nixhome flake"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(src, "build", "nixhome")
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing file inside the nested dst tree — CopyDir must not touch it.
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dst, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("untouched"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// The real top-level file must have been copied.
	if data, err := os.ReadFile(filepath.Join(dst, "flake.nix")); err != nil || string(data) != "# nixhome flake" {
		t.Errorf("expected flake.nix copied into dst, err=%v data=%q", err, data)
	}
	// The nested dst tree must not have been walked into and rewritten.
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "untouched" {
		t.Errorf("expected sentinel inside nested dst untouched, err=%v data=%q", err, data)
	}
}

// --- Scaffold with stack ---

func TestScaffold_WithStack_FlakeUsesChosenStack(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false, "go"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	s := string(data)
	if !strings.Contains(s, "devcell.stacks.go") {
		t.Errorf("flake.nix should contain devcell.stacks.go, got:\n%s", s)
	}
	if strings.Contains(s, "devcell.stacks.ultimate") {
		t.Errorf("flake.nix should NOT contain devcell.stacks.ultimate when stack=go:\n%s", s)
	}
}

func TestScaffold_WithStack_TomlHasStack(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false, "go"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	s := string(data)
	if !strings.Contains(s, `stack = "go"`) {
		t.Errorf(".devcell.toml should contain stack = \"go\", got:\n%s", s)
	}
}

func TestScaffold_WithStack_TomlIsValidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false, "python"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell.toml"))
	var v interface{}
	if _, err := toml.Decode(string(data), &v); err != nil {
		t.Errorf(".devcell.toml is not valid TOML: %v\ncontent:\n%s", err, string(data))
	}
}

func TestScaffold_EmptyStack_UsesTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
	s := string(data)
	// Template-based flake should use github: URL (not GenerateFlakeNix output)
	if !strings.Contains(s, "github:") {
		t.Errorf("empty stack should use template with github: URL:\n%s", s)
	}
}

func TestScaffold_WithStack_AllStacks(t *testing.T) {
	stacks := []string{"base", "go", "node", "python", "fullstack", "electronics", "ultimate"}
	for _, stack := range stacks {
		t.Run(stack, func(t *testing.T) {
			dir := t.TempDir()
			if err := scaffold.Scaffold(dir, "", "", false, stack); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(dir, ".devcell", "flake.nix"))
			want := "devcell.stacks." + stack
			if !strings.Contains(string(data), want) {
				t.Errorf("flake.nix should contain %s", want)
			}
		})
	}
}

// --- GenerateFlakeNix ---

// TestGenerateFlakeNix_DefaultStack — ultimate stack with no modules produces devcell.stacks.ultimate.
func TestGenerateFlakeNix_DefaultStack(t *testing.T) {
	content := scaffold.GenerateFlakeNix("ultimate", nil, "v1.0.0", false)
	if !strings.Contains(content, "devcell.stacks.ultimate") {
		t.Errorf("expected devcell.stacks.ultimate in flake.nix:\n%s", content)
	}
	if strings.Contains(content, "devcell.modules.") {
		t.Errorf("no modules expected in default flake.nix:\n%s", content)
	}
}

// TestGenerateFlakeNix_CustomStackWithModules — go stack + electronics module.
func TestGenerateFlakeNix_CustomStackWithModules(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", []string{"electronics"}, "v1.0.0", false)
	if !strings.Contains(content, "devcell.stacks.go") {
		t.Errorf("expected devcell.stacks.go:\n%s", content)
	}
	if !strings.Contains(content, "devcell.modules.electronics") {
		t.Errorf("expected devcell.modules.electronics:\n%s", content)
	}
}

// TestGenerateFlakeNix_MultipleModules — base stack + go + electronics + desktop.
func TestGenerateFlakeNix_MultipleModules(t *testing.T) {
	content := scaffold.GenerateFlakeNix("base", []string{"go", "electronics", "desktop"}, "v1.0.0", false)
	if !strings.Contains(content, "devcell.stacks.base") {
		t.Errorf("expected devcell.stacks.base:\n%s", content)
	}
	for _, mod := range []string{"go", "electronics", "desktop"} {
		if !strings.Contains(content, "devcell.modules."+mod) {
			t.Errorf("expected devcell.modules.%s:\n%s", mod, content)
		}
	}
}

// TestGenerateFlakeNix_ModulesAreEnabled — Modules 2.0 (CELL-65): every name
// in `modules` must end up with `devcell.modules.<name>.enable = true` in the
// generated flake. Importing the file alone is insufficient under the new
// mkEnableOption pattern — without the enable line, modules sit inert.
func TestGenerateFlakeNix_ModulesAreEnabled(t *testing.T) {
	content := scaffold.GenerateFlakeNix("dev", []string{"electronics", "plex"}, "v1.0.0", false)
	for _, mod := range []string{"electronics", "plex"} {
		want := `devcell.modules.` + mod + `.enable = true`
		if !strings.Contains(content, want) {
			t.Errorf("missing enable line for %q (expected %q):\n%s", mod, want, content)
		}
	}
}

// TestGenerateFlakeNix_NoModulesNoEnableBlock — when modules list is empty,
// the generated flake should NOT contain an enable block (avoid empty no-op).
func TestGenerateFlakeNix_NoModulesNoEnableBlock(t *testing.T) {
	content := scaffold.GenerateFlakeNix("dev", nil, "v1.0.0", false)
	if strings.Contains(content, ".enable = true") {
		t.Errorf("empty modules list should not emit any .enable=true lines:\n%s", content)
	}
}

// TestGenerateFlakeNix_BothArchitectures — must have devcell-local and devcell-local-aarch64.
func TestGenerateFlakeNix_BothArchitectures(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", nil, "v1.0.0", false)
	if !strings.Contains(content, `"devcell-local"`) {
		t.Errorf("expected devcell-local config:\n%s", content)
	}
	if !strings.Contains(content, `"devcell-local-aarch64"`) {
		t.Errorf("expected devcell-local-aarch64 config:\n%s", content)
	}
}

// TestGenerateFlakeNix_VersionSubstituted — version placeholder must be replaced.
func TestGenerateFlakeNix_VersionSubstituted(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", nil, "v2.3.4", false)
	if strings.Contains(content, "{{VERSION}}") {
		t.Errorf("unreplaced {{VERSION}} placeholder:\n%s", content)
	}
	want := nixhome.UpstreamOwner + "/" + nixhome.UpstreamRepo + "/v2.3.4"
	if !strings.Contains(content, want) {
		t.Errorf("expected versioned URL containing %q:\n%s", want, content)
	}
}

// TestGenerateFlakeNix_NixhomePath — when nixhomePath set, uses path:./nixhome.
func TestGenerateFlakeNix_NixhomePath(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", nil, "v1.0.0", true)
	if !strings.Contains(content, `"path:./nixhome"`) {
		t.Errorf("expected path:./nixhome when nixhomePath set:\n%s", content)
	}
	// Should not have active github: URL
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "inputs.devcell.url") && strings.Contains(trimmed, "github:") {
			t.Errorf("active URL should not be github: when nixhomePath set: %s", trimmed)
		}
	}
}

// TestGenerateFlakeNix_NoPlaceholders — no {{ }} left in output.
func TestGenerateFlakeNix_NoPlaceholders(t *testing.T) {
	content := scaffold.GenerateFlakeNix("base", []string{"go", "python"}, "v1.0.0", false)
	if strings.Contains(content, "{{") {
		t.Errorf("unreplaced placeholder in flake.nix:\n%s", content)
	}
}

// TestGenerateFlakeNix_X86Architecture — devcell-local uses x86_64-linux.
func TestGenerateFlakeNix_X86Architecture(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", nil, "v1.0.0", false)
	if !strings.Contains(content, `"x86_64-linux"`) {
		t.Errorf("expected x86_64-linux in devcell-local config:\n%s", content)
	}
	if !strings.Contains(content, `"aarch64-linux"`) {
		t.Errorf("expected aarch64-linux in devcell-local-aarch64 config:\n%s", content)
	}
}

// TestGenerateFlakeNix_StackOnlyNoModules — should have stack but no modules lines.
func TestGenerateFlakeNix_StackOnlyNoModules(t *testing.T) {
	content := scaffold.GenerateFlakeNix("python", nil, "v1.0.0", false)
	if !strings.Contains(content, "devcell.stacks.python") {
		t.Errorf("expected devcell.stacks.python:\n%s", content)
	}
	if strings.Contains(content, "devcell.modules.") {
		t.Errorf("expected no module references:\n%s", content)
	}
}

// TestGenerateFlakeNix_AllStacks — each known stack produces correct stacks reference.
func TestGenerateFlakeNix_AllStacks(t *testing.T) {
	stacks := []string{"base", "go", "node", "python", "fullstack", "electronics", "ultimate"}
	for _, stack := range stacks {
		t.Run(stack, func(t *testing.T) {
			content := scaffold.GenerateFlakeNix(stack, nil, "v1.0.0", false)
			want := "devcell.stacks." + stack
			if !strings.Contains(content, want) {
				t.Errorf("expected %s in output:\n%s", want, content)
			}
		})
	}
}

// ── CELL-445: NixPackages in GenerateFlakeNix ───────────────────────────────

func TestGenerateFlakeNix_NixPackagesStable(t *testing.T) {
	pkgs := cfg.NixPackages{Stable: []string{"tmux", "htop"}}
	content := scaffold.GenerateFlakeNix("go", nil, "v1.0.0", false, pkgs)
	if !strings.Contains(content, "({ lib, pkgs, ... }: { home.packages = (map (lib.setPrio (-10)) (with pkgs; [ tmux htop ])); })") {
		t.Errorf("expected module-function wrapper with hiPri stable packages:\n%s", content)
	}
}

func TestGenerateFlakeNix_NixPackagesAllTiers(t *testing.T) {
	pkgs := cfg.NixPackages{
		Stable:   []string{"tmux"},
		Unstable: []string{"tool-a"},
		Edge:     []string{"edge-pkg"},
	}
	content := scaffold.GenerateFlakeNix("base", nil, "v1.0.0", false, pkgs)
	// Lower number wins: a newer channel beats an older one on file collisions.
	if !strings.Contains(content, "map (lib.setPrio (-10)) (with pkgs; [ tmux ])") {
		t.Errorf("expected stable at -10:\n%s", content)
	}
	if !strings.Contains(content, "map (lib.setPrio (-15)) (with pkgsUnstable; [ tool-a ])") {
		t.Errorf("expected unstable at -15:\n%s", content)
	}
	if !strings.Contains(content, "map (lib.setPrio (-20)) (with pkgsEdge; [ edge-pkg ])") {
		t.Errorf("expected edge at -20:\n%s", content)
	}
}

func TestGenerateFlakeNix_NixPackagesEmpty(t *testing.T) {
	content := scaffold.GenerateFlakeNix("go", nil, "v1.0.0", false, cfg.NixPackages{})
	if strings.Contains(content, "lib.setPrio") {
		t.Errorf("no package module expected when all tiers empty:\n%s", content)
	}
}

func TestGenerateFlakeNix_NixPackagesWithModules(t *testing.T) {
	pkgs := cfg.NixPackages{Stable: []string{"cowsay"}}
	content := scaffold.GenerateFlakeNix("go", []string{"electronics"}, "v1.0.0", false, pkgs)
	if !strings.Contains(content, "devcell.modules.electronics") {
		t.Errorf("expected modules still present:\n%s", content)
	}
	if !strings.Contains(content, "map (lib.setPrio (-10)) (with pkgs; [ cowsay ])") {
		t.Errorf("expected hiPri stable packages alongside modules:\n%s", content)
	}
}

// ── MCP enabled in GenerateFlakeNixWithMcp ──────────────────────────────────

func TestGenerateFlakeNixWithMcp_EmptyNoMcpBlock(t *testing.T) {
	content := scaffold.GenerateFlakeNixWithMcp("go", nil, "v1.0.0", false, nil)
	if strings.Contains(content, "managedMcp") {
		t.Errorf("no MCP block expected when enabled list is nil:\n%s", content)
	}
}

// TestSyncNixhome_OverwritesExisting — SyncNixhome replaces previous nixhome copy (fresh each build).
func TestSyncNixhome_OverwritesExisting(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "flake.nix"), []byte("# v2"), 0644); err != nil {
		t.Fatal(err)
	}

	configDir := t.TempDir()
	// Pre-populate with stale content
	staleDir := filepath.Join(configDir, "nixhome")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "flake.nix"), []byte("# v1"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := scaffold.SyncNixhome(srcDir, configDir); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(configDir, "nixhome", "flake.nix"))
	if string(data) != "# v2" {
		t.Errorf("SyncNixhome should overwrite stale content, got: %s", string(data))
	}
}

// --- Scaffold local-first ---

func TestScaffold_WritesDotDevcellToml(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".devcell.toml")); err != nil {
		t.Error(".devcell.toml should exist in project root")
	}
}

func TestScaffold_BuildArtifactsInDotDevcellDir(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"flake.nix"} {
		path := filepath.Join(dir, ".devcell", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s in .devcell/ subdir: %v", name, err)
		}
	}
	for _, name := range []string{"Dockerfile", "package.json", "pyproject.toml"} {
		path := filepath.Join(dir, ".devcell", name)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s should NOT be generated by scaffold anymore", name)
		}
	}
}

func TestScaffold_NoBuildArtifactsInProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	// Dockerfile and flake.nix should NOT be in project root
	for _, name := range []string{"Dockerfile", "flake.nix"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s should NOT be in project root, only in .devcell/", name)
		}
	}
}

func TestScaffold_NoOldStyleDevcellToml(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	// Old-style devcell.toml (without dot) should NOT be created
	if _, err := os.Stat(filepath.Join(dir, "devcell.toml")); err == nil {
		t.Error("old-style devcell.toml should NOT be created")
	}
}

func TestScaffold_IsInitializedAfterScaffold(t *testing.T) {
	dir := t.TempDir()
	if err := scaffold.Scaffold(dir, "", "", false); err != nil {
		t.Fatal(err)
	}
	if !scaffold.IsInitialized(dir) {
		t.Error("IsInitialized should return true after Scaffold")
	}
}

// --- IsInitialized ---

func TestIsInitialized_TrueWhenDotDevcellTomlExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".devcell.toml"), []byte("[cell]\n"), 0644)
	if !scaffold.IsInitialized(dir) {
		t.Error("IsInitialized should return true when .devcell.toml exists")
	}
}

func TestIsInitialized_FalseWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	if scaffold.IsInitialized(dir) {
		t.Error("IsInitialized should return false in empty dir")
	}
}

func TestIsInitialized_FalseWhenOnlyGlobalTomlExists(t *testing.T) {
	dir := t.TempDir()
	// Old-style devcell.toml (without dot) should NOT count as initialized
	os.WriteFile(filepath.Join(dir, "devcell.toml"), []byte("[cell]\n"), 0644)
	if scaffold.IsInitialized(dir) {
		t.Error("IsInitialized should return false for old-style devcell.toml (without dot)")
	}
}

// A package listed in two channels is emitted only in the newest one.
func TestGenerateFlakeNix_NewerChannelWins(t *testing.T) {
	content := scaffold.GenerateFlakeNix("base", nil, "v1.0.0", false,
		cfg.NixPackages{Stable: []string{"jq", "uv"}, Unstable: []string{"uv"}})
	if !strings.Contains(content, "(with pkgs; [ jq ])") || !strings.Contains(content, "(with pkgsUnstable; [ uv ])") {
		t.Errorf("uv must come only from unstable:\n%s", content)
	}
}
