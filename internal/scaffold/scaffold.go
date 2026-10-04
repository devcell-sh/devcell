package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/nixhome"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
)

//go:embed templates/devcell.toml.tmpl
var devcellTomlContent []byte

//go:embed templates/starship.toml.tmpl
var starshipTomlContent []byte

type scaffoldFile struct {
	name    string
	content []byte
}

func scaffoldFiles(modelsSnippet string, withNixhome bool, stack string, modules []string) []scaffoldFile {
	flake := []byte(GenerateFlakeNix(stack, modules, version.Version, withNixhome))

	// Detected models go right after the commented [llm] reference.
	models := ""
	if modelsSnippet != "" {
		models = "\n" + strings.TrimRight(modelsSnippet, "\n") + "\n\n"
	}
	tomlContent := bytes.ReplaceAll(devcellTomlContent, []byte("{{MODELS_SECTION}}\n"), []byte(models))

	if stack != "" {
		tomlContent = bytes.ReplaceAll(tomlContent,
			[]byte(`# stack = "base"`),
			[]byte(fmt.Sprintf(`stack = %q`, stack)))
	}
	if len(modules) > 0 {
		// Format modules as TOML array: modules = ["go", "infra"]
		quoted := make([]string, len(modules))
		for i, m := range modules {
			quoted[i] = fmt.Sprintf("%q", m)
		}
		modulesLine := fmt.Sprintf("modules = [%s]", strings.Join(quoted, ", "))
		// Replace the commented example line in the template.
		tomlContent = bytes.ReplaceAll(tomlContent,
			[]byte(`# modules = ["electronics", "desktop"]`),
			[]byte(modulesLine))
	}

	return []scaffoldFile{
		{"flake.nix", flake},
		{"devcell.toml", tomlContent},
	}
}

// GenerateFlakeNix produces a flake.nix string that imports the given stack
// and modules from the upstream devcell nixhome flake.
// stack is a stack name (e.g. "go"), modules is a list of module names,
// ver is the version tag, nixhomePath overrides the input URL to path:./nixhome.
// nixPkgs adds arbitrary nixpkgs packages, prioritized per channel (user override semantics).
// mcpEnabled lists MCP server names to enable (from [mcp] enabled in .devcell.toml);
// each emits devcell.managedMcp.servers."<name>".enabled = true; in the flake.
func GenerateFlakeNix(stack string, modules []string, ver string, withNixhome bool, nixPkgs ...cfg.NixPackages) string {
	return generateFlakeNixFull(stack, modules, ver, withNixhome, nil, nixPkgs...)
}

// GenerateFlakeNixWithMcp is like GenerateFlakeNix but also emits MCP server
// enablement lines from [mcp] enabled in .devcell.toml.
func GenerateFlakeNixWithMcp(stack string, modules []string, ver string, withNixhome bool, mcpEnabled []string, nixPkgs ...cfg.NixPackages) string {
	return generateFlakeNixFull(stack, modules, ver, withNixhome, mcpEnabled, nixPkgs...)
}

func generateFlakeNixFull(stack string, modules []string, ver string, withNixhome bool, mcpEnabled []string, nixPkgs ...cfg.NixPackages) string {
	if stack == "" {
		stack = "base"
	}
	inputURL := fmt.Sprintf(`"%s"`, nixhome.UpstreamFlakeRef(ver))
	if withNixhome {
		inputURL = `"path:./nixhome"`
	}

	// Build the module expression for nix. devcell.stacks.X is already a list,
	// so we concatenate with ++ rather than wrapping in [...].
	moduleExpr := fmt.Sprintf("devcell.stacks.%s", stack)
	for _, m := range modules {
		moduleExpr += fmt.Sprintf(" ++ devcell.modules.%s", m)
	}

	// Modules 2.0 (CELL-65): each module file declares
	// `options.devcell.modules.<name>.enable = mkEnableOption ...` and gates its
	// config on it. Importing the file alone is not enough — we must ALSO set
	// .enable = true. Append an inline configuration list-element for that.
	if len(modules) > 0 {
		var enableLines []string
		for _, m := range modules {
			enableLines = append(enableLines, fmt.Sprintf("devcell.modules.%s.enable = true;", m))
		}
		moduleExpr += fmt.Sprintf(" ++ [ { %s } ]", strings.Join(enableLines, " "))
	}

	// CELL-445: [cell] packages + [packages.nix] — arbitrary user packages that override modules.
	// Emitted as a NixOS-style module function ({ lib, pkgs, ... }: { ... })
	// so lib/pkgs are in scope when the module system evaluates.
	// Each channel gets its own priority (lower wins): stable matches
	// lib.hiPrio so user packages beat modules, and a newer channel beats an
	// older one when two packages ship the same file. Packages listed in two
	// channels are emitted only in the newest (cfg.ResolveNixChannels).
	var np cfg.NixPackages
	if len(nixPkgs) > 0 {
		np, _ = cfg.ResolveNixChannels(nixPkgs[0])
	}
	if len(np.Stable) > 0 || len(np.Unstable) > 0 || len(np.Edge) > 0 {
		var parts []string
		args := "lib, pkgs"
		if len(np.Stable) > 0 {
			parts = append(parts, fmt.Sprintf("(map (lib.setPrio (-10)) (with pkgs; [ %s ]))", strings.Join(np.Stable, " ")))
		}
		if len(np.Unstable) > 0 {
			args += ", pkgsUnstable"
			parts = append(parts, fmt.Sprintf("(map (lib.setPrio (-15)) (with pkgsUnstable; [ %s ]))", strings.Join(np.Unstable, " ")))
		}
		if len(np.Edge) > 0 {
			args += ", pkgsEdge"
			parts = append(parts, fmt.Sprintf("(map (lib.setPrio (-20)) (with pkgsEdge; [ %s ]))", strings.Join(np.Edge, " ")))
		}
		moduleExpr += fmt.Sprintf(" ++ [ ({ %s, ... }: { home.packages = %s; }) ]", args, strings.Join(parts, " ++ "))
	}

	// [mcp] enabled is now resolved at container start via DEVCELL_MCP_ENABLED
	// env var — no longer baked into the flake overlay. The mcpEnabled parameter
	// is kept for API compatibility but ignored.

	return fmt.Sprintf(`{
  description = "DevCell user stack — customise and run 'cell build'";

  # Follows main branch by default. To pin a specific release:
  #   inputs.devcell.url = "github:devcell-sh/home/v1.0.0";
  # To use your own nixhome fork:
  #   inputs.devcell.url = "github:yourusername/nixhome";
  inputs.devcell.url = %s;

  outputs = { self, devcell, ... }: {
    homeConfigurations = {
      "devcell-local" = devcell.lib.mkHome "x86_64-linux" (%s);
      "devcell-local-aarch64" = devcell.lib.mkHome "aarch64-linux" (%s);
    };
  };
}
`, inputURL, moduleExpr, moduleExpr)
}

var nixhomeRepo = "https://github.com/devcell-sh/home.git"

// IsGitURL returns true if source looks like a git URL or GitHub shorthand.
func IsGitURL(source string) bool {
	return strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "github:") ||
		strings.HasPrefix(source, "ssh://")
}

// ResolveNixhome pulls nixhome into buildDir/nixhome/ from the given source.
//   - Local path: copy directly (rsync-like)
//   - Git URL: shallow sparse clone, extract nixhome/ subdir
//   - Empty source: clone from upstream repo at the given version tag
//
// Always re-fetches; force is currently unused.
func ResolveNixhome(source, buildDir, ver string, force bool) error {
	dest := filepath.Join(buildDir, "nixhome")

	if source != "" && !IsGitURL(source) {
		// Local path — always sync (fast, user expects local changes picked up).
		return SyncNixhome(source, buildDir)
	}

	// Git source — always fetch latest.
	gs := parseGitSource(source)
	if gs.RepoURL == "" {
		// No source provided — use upstream default (flake at repo root).
		gs = gitSource{RepoURL: nixhomeRepo}
	}

	ref := gs.Ref
	if ref == "" {
		ref = ver
	}
	if ref == "" || ref == "v0.0.0" {
		ref = nixhome.DefaultNixhomeGitRef
	}
	subdir := gs.Subdir

	label := fmt.Sprintf("Fetching nixhome from %s", gs.RepoURL)
	if subdir != "" {
		label += "/" + subdir
	}
	sp := ux.NewProgressSpinner(label)

	tmpDir, err := os.MkdirTemp("", "devcell-nixhome-*")
	if err != nil {
		sp.Fail("Fetch nixhome failed")
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Shallow clone with sparse checkout.
	cloneArgs := []string{"clone", "--depth", "1", "--branch", ref, "--filter=blob:none"}
	if subdir != "" {
		cloneArgs = append(cloneArgs, "--sparse")
	}
	cloneArgs = append(cloneArgs, gs.RepoURL, tmpDir)

	cmds := [][]string{{"git"}}
	cmds[0] = append(cmds[0], cloneArgs...)
	if subdir != "" {
		cmds = append(cmds, []string{"git", "-C", tmpDir, "sparse-checkout", "set", subdir})
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if err := cmd.Run(); err != nil {
			sp.Fail("Fetch nixhome failed")
			return fmt.Errorf("%s: %w", strings.Join(args[:2], " "), err)
		}
	}

	// Copy from clone to dest.
	src := tmpDir
	if subdir != "" {
		src = filepath.Join(tmpDir, subdir)
	}
	if _, err := os.Stat(src); err != nil {
		sp.Fail("Fetch nixhome failed")
		return fmt.Errorf("nixhome not found in clone: %w", err)
	}
	os.RemoveAll(dest)
	os.Remove(filepath.Join(buildDir, "flake.lock"))
	if err := CopyDir(src, dest); err != nil {
		sp.Fail("Fetch nixhome failed")
		return err
	}

	// Record source origin for change detection.
	sourceLabel := gs.RepoURL
	if source != "" {
		sourceLabel = source
	}
	os.WriteFile(filepath.Join(dest, NixhomeSourceFile), []byte(sourceLabel+"\n"), 0644)

	sp.Success(fmt.Sprintf("Fetched nixhome (%s)", ref))
	return nil
}

// gitSource holds the parsed components of a git nixhome source.
type gitSource struct {
	RepoURL string // e.g. https://github.com/devcell-sh/home.git
	Ref     string // branch/tag override (empty = use version default)
	Subdir  string // subdirectory within repo (empty = repo root)
}

// parseGitSource parses various git URL formats into repo + ref + subdir.
// Supported formats:
//   - "github:user/repo"                                       → https://github.com/user/repo.git, subdir=""
//   - "github:user/repo/subdir"                                → https://github.com/user/repo.git, subdir="subdir"
//   - "https://github.com/user/repo/tree/branch/path/to/dir"  → repo.git, ref=branch, subdir="path/to/dir"
//   - "https://github.com/user/repo.git"                       → as-is
//   - "git@github.com:user/repo.git"                           → as-is
func parseGitSource(source string) gitSource {
	// GitHub shorthand: github:user/repo or github:user/repo/subdir
	if strings.HasPrefix(source, "github:") {
		parts := strings.SplitN(strings.TrimPrefix(source, "github:"), "/", 3)
		if len(parts) >= 2 {
			gs := gitSource{RepoURL: "https://github.com/" + parts[0] + "/" + parts[1] + ".git"}
			if len(parts) == 3 {
				gs.Subdir = parts[2]
			}
			return gs
		}
	}

	// GitHub tree URL: https://github.com/user/repo/tree/branch/path/to/dir
	if strings.Contains(source, "github.com/") && strings.Contains(source, "/tree/") {
		// Split on /tree/ to get repo and branch+path
		parts := strings.SplitN(source, "/tree/", 2)
		repoURL := strings.TrimSuffix(parts[0], "/") + ".git"
		if len(parts) == 2 {
			// branch/path/to/dir — first segment is branch, rest is subdir
			branchAndPath := strings.SplitN(parts[1], "/", 2)
			gs := gitSource{RepoURL: repoURL, Ref: branchAndPath[0]}
			if len(branchAndPath) == 2 {
				gs.Subdir = branchAndPath[1]
			}
			return gs
		}
		return gitSource{RepoURL: repoURL}
	}

	return gitSource{RepoURL: source}
}

// NixhomeSourceFile is the metadata file that tracks which source was used
// to populate .devcell/nixhome/. Used to detect when a different source
// would overwrite an existing nixhome.
const NixhomeSourceFile = ".devcell-source"

// SyncNixhome copies the nixhome directory from srcPath into configDir/nixhome/.
// It replaces any existing nixhome copy to ensure fresh content each build.
// Also removes the outer flake.lock so nix regenerates it from the inner
// nixhome's inputs — prevents stale lock from pinning different nixpkgs
// than the base image, which would cause a full re-download.
// Writes .devcell-source to track the origin.
//
// srcPath accepts either:
//   - a local filesystem path (copied as-is via CopyDir)
//   - a github flake ref like `github:owner/repo/ref?dir=subdir` — cloned via
//     git (host-side; no nix dependency), then the subdir is treated as the
//     source. This lets the thin builder mount a populated nixhome even when
//     the user has no local checkout (CELL-38 clean-machine path).
func SyncNixhome(srcPath, configDir string) error {
	if strings.HasPrefix(srcPath, "github:") {
		materialized, cleanup, err := materializeGithubFlakeRef(srcPath)
		if err != nil {
			return fmt.Errorf("materialize %s: %w", srcPath, err)
		}
		defer cleanup()
		// Recurse with the materialized local path; record the original ref
		// as origin in .devcell-source for change detection (see line below).
		if err := syncNixhomeFromLocal(materialized, configDir, srcPath); err != nil {
			return err
		}
		return nil
	}
	return syncNixhomeFromLocal(srcPath, configDir, srcPath)
}

// syncNixhomeFromLocal does the actual cp + .devcell-source + git-init work.
// origin is the value recorded in .devcell-source — for direct local syncs
// it equals srcPath, but for github materializations origin is the original
// `github:...` ref (so change detection sees the ref, not the temp dir).
func syncNixhomeFromLocal(srcPath, configDir, origin string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("nixhome source %s: %w", srcPath, err)
	}
	dest := filepath.Join(configDir, "nixhome")

	if nested, err := isPathNestedIn(dest, srcPath); err != nil {
		return fmt.Errorf("resolve nixhome paths: %w", err)
	} else if nested {
		return fmt.Errorf(
			"nixhome source %s contains its own build directory (%s) — "+
				"DEVCELL_NIXHOME/--nixhome must point at a separate nixhome checkout, "+
				"not the project's own directory (or an ancestor of it)",
			srcPath, dest,
		)
	}

	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("remove old nixhome: %w", err)
	}
	os.Remove(filepath.Join(configDir, "flake.lock"))
	// Skip the source's own .devcell/ build dir: a nixhome checkout opened
	// as a cell project has one, with a nested git repo that breaks git add.
	if err := copyDirSkipping(srcPath, dest, ".devcell"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, NixhomeSourceFile), []byte(origin+"\n"), 0644); err != nil {
		return err
	}
	ux.Debugf("SyncNixhome: copied %s → %s (origin: %s)", srcPath, dest, origin)

	if err := exec.Command("git", "init", "-q", dest).Run(); err != nil {
		return fmt.Errorf("git init synced nixhome: %w", err)
	}
	if err := exec.Command("git", "-C", dest, "add", ".").Run(); err != nil {
		return fmt.Errorf("git add synced nixhome: %w", err)
	}
	ux.Debugf("SyncNixhome: git init + add in %s (nix flake visibility fix)", dest)
	return nil
}

// materializeGithubFlakeRef runs `git clone --depth 1 --branch <ref>` to fetch
// the github repo into a temp dir, then returns the abs path to the subdir
// (e.g. nixhome/) referenced by `?dir=...`. Cleanup removes the temp clone.
//
// Host-side fetch — no nix dependency. Mirrors what nix would have done with
// `nix flake metadata` materialization but works on machines without nix
// (the thin-mode promise).
func materializeGithubFlakeRef(ref string) (string, func(), error) {
	parsed, err := ParseGithubFlakeRef(ref)
	if err != nil {
		return "", func() {}, err
	}
	tmpDir, err := os.MkdirTemp("", "devcell-nixhome-clone-")
	if err != nil {
		return "", func() {}, fmt.Errorf("mkdir tmp: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	ux.Debugf("Cloning %s @ %s → %s", parsed.CloneURL(), parsed.Ref, tmpDir)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", parsed.Ref, parsed.CloneURL(), tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("git clone %s @ %s: %w\n%s", parsed.CloneURL(), parsed.Ref, err, output)
	}

	src := tmpDir
	if parsed.Subdir != "" {
		src = filepath.Join(tmpDir, parsed.Subdir)
		if _, err := os.Stat(src); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("subdir %q not found in %s: %w", parsed.Subdir, parsed.CloneURL(), err)
		}
	}
	return src, cleanup, nil
}

// isPathNestedIn reports whether child is inside (or equal to) parent, after
// resolving symlinks so aliasing can't defeat the check. Tolerates paths
// that don't exist yet (e.g. dest before its first sync) by falling back to
// the unresolved absolute path.
func isPathNestedIn(child, parent string) (bool, error) {
	absChild, err := filepath.Abs(child)
	if err != nil {
		return false, err
	}
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(absChild); err == nil {
		absChild = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absParent); err == nil {
		absParent = resolved
	}
	rel, err := filepath.Rel(absParent, absChild)
	if err != nil {
		return false, nil
	}
	if rel == "." {
		return true, nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

// CopyDir recursively copies src directory to dst.
//
// Guards against dst being nested inside src (e.g. a caller accidentally
// pointing a nixhome sync at a project's own directory): without this, the
// walk would recurse into paths it is itself writing, corrupting the copy
// partway through. This is a defensive backstop — callers should also
// reject that configuration upfront (see isPathNestedIn in
// syncNixhomeFromLocal) so the failure is a clear error instead of a silent
// partial copy.
func CopyDir(src, dst string) error {
	return copyDirSkipping(src, dst)
}

// copyDirSkipping is CopyDir, minus the named top-level entries of src.
func copyDirSkipping(src, dst string, skip ...string) error {
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if absPath, aerr := filepath.Abs(path); aerr == nil {
			if absPath == absDst || strings.HasPrefix(absPath, absDst+string(filepath.Separator)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		rel, _ := filepath.Rel(src, path)
		if slices.Contains(skip, rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// Scaffold writes .devcell.toml to dir (project root) and build artifacts
// (flake.nix, starship.toml) to dir/.devcell/ (build context, gitignored).
// ScaffoldWithModules is like Scaffold but also writes the selected modules list.
func ScaffoldWithModules(dir string, modelsSnippet string, nixhomePath string, force bool, stack string, modules []string) error {
	return doScaffold(dir, modelsSnippet, nixhomePath, force, stack, modules)
}

func Scaffold(dir string, modelsSnippet string, nixhomePath string, force bool, stack ...string) error {
	stk := ""
	if len(stack) > 0 {
		stk = stack[0]
	}
	return doScaffold(dir, modelsSnippet, nixhomePath, force, stk, nil)
}

func doScaffold(dir string, modelsSnippet string, nixhomePath string, force bool, stk string, modules []string) error {

	buildDir := filepath.Join(dir, ".devcell")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", buildDir, err)
	}

	// Sync nixhome FIRST so the detect-on-disk check below sees it.
	if nixhomePath != "" {
		if err := SyncNixhome(nixhomePath, buildDir); err != nil {
			return fmt.Errorf("sync nixhome: %w", err)
		}
	}

	// Detect nixhome on disk — if .devcell/nixhome/ exists, use path:./nixhome.
	_, nixhomeStat := os.Stat(filepath.Join(buildDir, "nixhome"))
	withNixhome := nixhomeStat == nil

	// Write .devcell.toml to project root, build artifacts to .devcell/.
	for _, f := range scaffoldFiles(modelsSnippet, withNixhome, stk, modules) {
		var dest string
		if f.name == "devcell.toml" {
			// Config file → project root as .devcell.toml (dot-prefixed)
			dest = filepath.Join(dir, ".devcell.toml")
		} else {
			// Build artifacts → .devcell/ subdir
			dest = filepath.Join(buildDir, f.name)
		}
		if !force {
			if _, err := os.Stat(dest); err == nil {
				continue
			}
		}
		if err := os.WriteFile(dest, f.content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}

	// Scaffold homedir/.config/starship.toml for per-project prompt customization.
	starshipDir := filepath.Join(buildDir, "homedir", ".config")
	starshipDest := filepath.Join(starshipDir, "starship.toml")
	if force || os.IsNotExist(statErr(starshipDest)) {
		if err := os.MkdirAll(starshipDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", starshipDir, err)
		}
		if err := os.WriteFile(starshipDest, starshipTomlContent, 0644); err != nil {
			return fmt.Errorf("write homedir starship.toml: %w", err)
		}
	}

	return nil
}

// statErr returns the error from os.Stat (nil if file exists).
func statErr(path string) error {
	_, err := os.Stat(path)
	return err
}

// IsInitialized returns true when .devcell.toml exists in dir.
func IsInitialized(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".devcell.toml"))
	return err == nil
}
