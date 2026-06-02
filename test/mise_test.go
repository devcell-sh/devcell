// mise_test.go — TDD tests for DIMM-214: bake mise installs + shims into image,
// two-level shim PATH for reliable runtime tooling.
//
// L1: file-content / wiring validation (no Docker, no nix runtime needed)
//     - Verifies the Dockerfile and nixhome modules contain the design hooks.
//     - Failing L1 = the wiring is missing; impl needs to land first.
//
// L2: container exec — declared mise tools are on PATH and runnable.
//     - Uses the existing testcontainers harness (image() + startContainer + exec).
//     - Run via `task test:integration -- -run TestMise`.

package container_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// L1 — Wiring checks (file content)
// ---------------------------------------------------------------------------

// TestMise_DockerfileBakesShims asserts the Dockerfile runs `mise reshim` at
// build time with MISE_DATA_DIR pointed at the image-level baked shim dir.
// Without this, the impure path never produces /opt/devcell/.local/share/mise/shims/.
func TestMise_DockerfileBakesShims(t *testing.T) {
	dockerfile := readDockerfile(t)

	want := "/opt/devcell/.local/share/mise"
	if !strings.Contains(dockerfile, want+" mise reshim") &&
		!strings.Contains(dockerfile, "MISE_DATA_DIR="+want) {
		t.Fatalf("Dockerfile missing baked-shim step: expected `MISE_DATA_DIR=%s mise reshim` (or env-exported equivalent) somewhere in the build chain", want)
	}
}

// TestMise_DockerfileBakesShimsForReshim asserts the reshim invocation comes
// AFTER mise install (so there are installs to reshim against). Order matters.
func TestMise_DockerfileBakesShimsForReshim(t *testing.T) {
	dockerfile := readDockerfile(t)

	installIdx := strings.Index(dockerfile, "mise install")
	reshimIdx := strings.Index(dockerfile, "mise reshim")
	if installIdx == -1 {
		t.Fatal("Dockerfile has no `mise install` step — baseline broken")
	}
	if reshimIdx == -1 {
		t.Fatal("Dockerfile has no `mise reshim` step (TestMise_DockerfileBakesShims should also fail)")
	}
	if reshimIdx < installIdx {
		t.Fatalf("`mise reshim` (idx %d) comes BEFORE `mise install` (idx %d) — order must be install→reshim", reshimIdx, installIdx)
	}
}

// TestMise_SessionPathHasBakedShimDir asserts nixhome/modules/mise.nix adds
// /opt/devcell/.local/share/mise/shims to home.sessionPath (level-2 dir).
func TestMise_SessionPathHasBakedShimDir(t *testing.T) {
	mise := readNixhomeFile(t, "modules/mise.nix")

	if !strings.Contains(mise, "/opt/devcell/.local/share/mise/shims") {
		t.Fatal("nixhome/modules/mise.nix missing `/opt/devcell/.local/share/mise/shims` — level-2 baked shims won't be on PATH")
	}
	if !regexp.MustCompile(`(?s)home\.sessionPath\s*=\s*\[[^\]]*"/opt/devcell/\.local/share/mise/shims"`).MatchString(mise) {
		t.Fatal("baked shim dir not inside a `home.sessionPath = [ ... ]` list")
	}
}

// TestMise_SessionPath_UserShimsBeforeBaked asserts precedence: user shims dir
// must come BEFORE the baked dir so user-installed tools override baked ones.
// home-manager prepends home.sessionPath entries in list order; first wins.
func TestMise_SessionPath_UserShimsBeforeBaked(t *testing.T) {
	mise := readNixhomeFile(t, "modules/mise.nix")

	bakedPath := "/opt/devcell/.local/share/mise/shims"
	bakedIdx := strings.Index(mise, bakedPath)
	if bakedIdx == -1 {
		t.Fatal("baked shim dir missing — TestMise_SessionPathHasBakedShimDir should also fail")
	}

	// Find a user-shim entry that is NOT the baked path string.
	// Accept either $HOME/.local/share/mise/shims or ${config.home.homeDirectory}/...
	userPattern := regexp.MustCompile(`(\$HOME|\$\{config\.home\.homeDirectory\}|~)/\.local/share/mise/shims`)
	userMatches := userPattern.FindAllStringIndex(mise, -1)
	if len(userMatches) == 0 {
		t.Fatal("user shims dir (`$HOME/.local/share/mise/shims` or `${config.home.homeDirectory}/.local/share/mise/shims`) missing from mise.nix")
	}
	userFirst := userMatches[0][0]

	if userFirst >= bakedIdx {
		t.Fatalf("user shims (idx %d) must come BEFORE baked shims (idx %d) in home.sessionPath; current order would let baked override user installs", userFirst, bakedIdx)
	}
}

// TestMise_ShellRcHasBakedShimDir asserts 05-shell-rc.sh's PATH export
// includes /opt/devcell/.local/share/mise/shims (level-2 baked dir) for the
// session user. mise.nix's home.sessionPath only affects the devcell user;
// session users get PATH from this shell-rc fragment, so it must mirror the
// two-level shim design or declared tools stay off the session-user PATH.
func TestMise_ShellRcHasBakedShimDir(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/05-shell-rc.sh")

	if !strings.Contains(frag, "/opt/devcell/.local/share/mise/shims") {
		t.Fatal("nixhome/modules/fragments/05-shell-rc.sh missing `/opt/devcell/.local/share/mise/shims` in PATH — session user won't have level-2 baked shims on PATH (user shim dir alone is fragile; baked dir is the safety net per DIMM-214)")
	}
}

// TestMise_ShellRc_UserShimsBeforeBaked asserts precedence inside
// 05-shell-rc.sh's PATH export: $HOME/.local/share/mise/shims (level 1)
// must come BEFORE /opt/devcell/.local/share/mise/shims (level 2) so users
// can override baked tools by running `mise install <tool>@<ver>` post-boot.
func TestMise_ShellRc_UserShimsBeforeBaked(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/05-shell-rc.sh")

	bakedPath := "/opt/devcell/.local/share/mise/shims"
	bakedIdx := strings.Index(frag, bakedPath)
	if bakedIdx == -1 {
		t.Fatal("baked shim dir missing — TestMise_ShellRcHasBakedShimDir should also fail")
	}

	userPattern := regexp.MustCompile(`\$HOME/\.local/share/mise/shims`)
	userMatches := userPattern.FindAllStringIndex(frag, -1)
	if len(userMatches) == 0 {
		t.Fatal("user shims dir (`$HOME/.local/share/mise/shims`) missing from 05-shell-rc.sh PATH export")
	}
	userFirst := userMatches[0][0]

	if userFirst >= bakedIdx {
		t.Fatalf("user shims (idx %d) must come BEFORE baked shims (idx %d) in 05-shell-rc.sh PATH; current order would let baked override user installs", userFirst, bakedIdx)
	}
}

// TestMise_EntrypointReshimNotSilenced asserts the runtime reshim in
// 10-mise.sh no longer silently swallows failures. The previous form
// `mise reshim 2>/dev/null || true` hid the very bug we're fixing.
func TestMise_EntrypointReshimNotSilenced(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	for _, line := range strings.Split(frag, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "mise") || !strings.Contains(trimmed, "reshim") {
			continue
		}
		if strings.Contains(trimmed, "2>/dev/null") && strings.Contains(trimmed, "|| true") {
			t.Fatalf("reshim line still silenced: %q — must log failures loudly", trimmed)
		}
	}
}

// TestMise_ReshimGatedByShaOnWarmBoot pins the steady-state-boot optimization.
// Pre-fix shape ran `mise reshim` on every cold boot, BEFORE the sha-gate
// that already proves whether `mise install -y` has work to do. On macOS
// bind mounts with a populated $HOME/.local/share/mise (~17k entries),
// reshim's walk costs ~30–40s per boot even when .tool-versions has not
// changed since the previous launch.
//
// Fix: the earliest `mise reshim` invocation in the fragment must sit
// INSIDE the global sha-gate (i.e. after `if [ -f "$HOME/.tool-versions" ]`
// opens), so it runs only on the install branch.
func TestMise_ReshimGatedByShaOnWarmBoot(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	gateOpen := strings.Index(frag, `if [ -f "$HOME/.tool-versions" ]`)
	if gateOpen == -1 {
		t.Fatal("global sha-gate marker `if [ -f \"$HOME/.tool-versions\" ]` not found — fragment shape changed")
	}

	// The fragment invokes reshim only as `"$mise_bin" reshim`. We match the
	// invocation form specifically — bare-prose "mise reshim" appears in
	// comments and would yield false positives if used as a fallback.
	reshimIdx := strings.Index(frag, `"$mise_bin" reshim`)
	if reshimIdx == -1 {
		t.Fatal("no `\"$mise_bin\" reshim` invocation found in 10-mise.sh — baseline broken or invocation form changed")
	}

	if reshimIdx < gateOpen {
		t.Fatalf("`mise reshim` (byte %d) runs BEFORE the sha-gate opens at byte %d.\n"+
			"Result: reshim executes on every boot regardless of whether .tool-versions changed,\n"+
			"walking ~17k files on the persistent bind mount (~30-40s on macOS hosts).\n"+
			"Move the reshim call inside the else branch of the sha-gate, alongside `mise install -y`.",
			reshimIdx, gateOpen)
	}
}

// TestMise_ChownsGatedByShaOnWarmBoot — same shape as the reshim test, but
// covers the two `chown -R` invocations (over $user_mise and over
// $HOME/.local/state/mise) that walk the same ~17k-file tree. They must
// also sit inside the global sha-gate so warm boots skip them.
func TestMise_ChownsGatedByShaOnWarmBoot(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	gateOpen := strings.Index(frag, `if [ -f "$HOME/.tool-versions" ]`)
	if gateOpen == -1 {
		t.Fatal("global sha-gate marker not found — fragment shape changed")
	}

	cases := []struct {
		name    string
		pattern string
	}{
		{"chown user_mise", `chown -R "$HOST_USER" "$user_mise"`},
		{"chown mise state", `chown -R "$HOST_USER" "$HOME/.local/state/mise"`},
	}
	for _, tc := range cases {
		idx := strings.Index(frag, tc.pattern)
		if idx == -1 {
			t.Errorf("%s: pattern %q not found in 10-mise.sh — baseline broken or fragment shape changed",
				tc.name, tc.pattern)
			continue
		}
		if idx < gateOpen {
			t.Errorf("%s: `%s` (byte %d) runs BEFORE the sha-gate opens at byte %d.\n"+
				"Result: a recursive chown walks the entire mise data dir on every warm boot.\n"+
				"Move this chown inside the else branch of the sha-gate.",
				tc.name, tc.pattern, idx, gateOpen)
		}
	}
}

// TestMise_EntrypointAlwaysCleansDanglingSymlinks asserts the entrypoint
// cleans dangling install symlinks unconditionally — NOT only when /opt/mise
// exists. Pure images don't have /opt/mise; if cleanup is gated on the cross-
// bind loop's outer iteration, leftover dangling symlinks from a prior image
// generation (impure → pure transition) persist and break `mise reshim`.
// Symptom: terraform / opentofu missing from PATH on every fresh pure cell.
func TestMise_EntrypointAlwaysCleansDanglingSymlinks(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	// The cleanup loop must exist OUTSIDE the outer `for tool_dir in "$baked/installs"/*/` block.
	// We assert by structural inspection: the dangling-symlink cleanup
	// (`[ -L "$link" ] && [ ! -e "$link" ]`) appears at the same indentation
	// as `mkdir -p "$user_mise"`, not nested inside the baked-installs loop.
	//
	// Simpler heuristic: the cleanup block must appear BEFORE the
	// `for tool_dir in "$baked/installs"/*/` opening (so it runs even when
	// $baked doesn't exist).
	bakedLoopIdx := strings.Index(frag, `for tool_dir in "$baked/installs"`)
	cleanupIdx := strings.Index(frag, `[ ! -e "$link" ]`)

	if bakedLoopIdx == -1 {
		t.Fatal("baked-installs loop pattern not found — 10-mise.sh shape changed unexpectedly")
	}
	if cleanupIdx == -1 {
		t.Fatal("dangling-symlink cleanup pattern not found in 10-mise.sh")
	}
	if cleanupIdx > bakedLoopIdx {
		t.Fatalf("dangling-symlink cleanup (idx %d) is INSIDE or AFTER the baked-installs loop (idx %d) — must be hoisted before the loop so pure images (no /opt/mise) still clean leftover symlinks", cleanupIdx, bakedLoopIdx)
	}
}

// TestMise_EntrypointBakedLoopGatedOnDirExists asserts the cross-bind loop
// over `/opt/mise/installs/` only runs when the directory exists. Without
// this gate, pure images (no /opt/mise) still iterate empty/dangling content
// and waste cycles or worse, create dangling links.
//
// Note: shell's `for x in glob/*/` is already empty when glob has no matches,
// so this is mostly a defense-in-depth check. The test pins the design intent.
func TestMise_EntrypointBakedLoopGatedOnDirExists(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	// Either an explicit `[ -d "$baked/installs" ]` guard wraps the loop,
	// or the loop body has a `[ -d "$tool_dir" ] || continue` first line.
	// Accept either form.
	hasOuterGuard := strings.Contains(frag, `[ -d "$baked/installs" ]`) || strings.Contains(frag, `[ -d "$baked" ]`)
	hasInnerContinue := regexp.MustCompile(`for tool_dir in "\$baked/installs"[^\n]*\n\s*\[ -d "\$tool_dir" \] \|\| continue`).MatchString(frag)

	if !hasOuterGuard && !hasInnerContinue {
		t.Fatal("baked-installs loop missing pure-image guard — need either `[ -d \"$baked/installs\" ]` wrapper or `[ -d \"$tool_dir\" ] || continue` inside the loop body")
	}
}

// TestMise_EntrypointInvalidatesShaOnStaleState pins the DIMM-215 fix.
// When the cleanup loop wipes dangling install symlinks (or detects an empty
// tool dir), the sha-gate further down would otherwise still match
// ~/.tool-versions's checksum and skip `mise install -y` — leaving declared
// tools missing from PATH. The fix: invalidate .tv-global.sha (and the
// workspace variant) immediately after cleanup so the install pass runs.
//
// Assert structurally:
//  1. The fragment removes `.tv-global.sha` (an `rm -f ... .tv-global.sha`).
//  2. The removal happens BEFORE the sha-gate read (`sha256sum ".tool-versions"`).
//  3. The removal is conditional on a cleanup/stale-state flag (not a blanket
//     unconditional wipe — that would defeat the optimization on steady-state).
func TestMise_EntrypointInvalidatesShaOnStaleState(t *testing.T) {
	frag := readNixhomeFile(t, "modules/fragments/10-mise.sh")

	// 1) An `rm -f` line touching .tv-global.sha must exist.
	rmPattern := regexp.MustCompile(`rm\s+-f[^\n]*\.tv-global\.sha`)
	rmMatches := rmPattern.FindAllStringIndex(frag, -1)
	if len(rmMatches) == 0 {
		t.Fatal("entrypoint never removes .tv-global.sha — sha-gate will skip `mise install -y` even after cleanup wipes installs (DIMM-215)")
	}

	// 2) The invalidation must come BEFORE the sha-gate read so the next
	//    install pass sees a missing/different sha and proceeds.
	rmIdx := rmMatches[0][0]
	gateIdx := strings.Index(frag, `sha256sum "$HOME/.tool-versions"`)
	if gateIdx == -1 {
		t.Fatal("sha-gate read (`sha256sum \"$HOME/.tool-versions\"`) not found — fragment shape changed")
	}
	if rmIdx > gateIdx {
		t.Fatalf("`.tv-global.sha` invalidation (idx %d) comes AFTER the sha-gate read (idx %d) — must precede so the gate sees the wiped sha", rmIdx, gateIdx)
	}

	// 3) Invalidation must be inside a conditional (not unconditional). We
	//    accept any `if` guard immediately above the rm line in the same
	//    block. Concretely: there must be an `if [ ... ]; then` between the
	//    cleanup loop and the rm. Looser check: the rm line is indented
	//    further than `setup_mise_home() {` opens its body, AND there's an
	//    `if ` token between the cleanup pattern and the rm.
	cleanupAnchor := strings.Index(frag, `[ ! -e "$link" ]`)
	if cleanupAnchor == -1 {
		t.Fatal("cleanup pattern not found — fragment shape changed (TestMise_EntrypointAlwaysCleansDanglingSymlinks should also fail)")
	}
	between := frag[cleanupAnchor:rmIdx]
	if !strings.Contains(between, "if ") {
		t.Fatal("`.tv-global.sha` invalidation is not guarded by an `if ...` — would defeat the sha-gate optimization on steady-state restarts")
	}
}

// TestMise_ImageNixStagesToolVersions asserts the pure-image build (image.nix)
// manually stages /etc/devcell/tool-versions because home.activation scripts
// (writeToolVersions in mise.nix) don't run at pure-image build time — only
// home.file content gets copied via homeConfig.activationPackage/home-files/.
//
// Without this stage, fresh pure cells have no ~/.tool-versions to install
// from at boot — `mise install -y` is gated on the file's existence in
// 10-mise.sh, so declared tools never get installed and reshim has nothing
// to generate shims for. User-visible: `command not found` for terraform,
// tofu, etc. on every pure cell launch.
func TestMise_ImageNixStagesToolVersions(t *testing.T) {
	imgNix := readNixhomeFile(t, "packages/image.nix")

	// Accept either an explicit `tool-versions` write step or a generic
	// "stage devcell.mise.tools content into /etc/devcell/tool-versions" pattern.
	// We check structural intent: the file path must appear AND the homeConfig's
	// mise.tools attribute must be referenced (so we know the content is sourced
	// from the live declarations, not a hardcoded list).
	if !strings.Contains(imgNix, "/etc/devcell/tool-versions") {
		t.Fatal("image.nix doesn't stage /etc/devcell/tool-versions — pure images won't have declared mise tools available on first boot")
	}
	if !strings.Contains(imgNix, "devcell.mise.tools") &&
		!strings.Contains(imgNix, "config.devcell.mise") {
		t.Fatal("image.nix references /etc/devcell/tool-versions but doesn't source content from `homeConfig.config.devcell.mise.tools` — content must be live, not hardcoded")
	}
}

// TestMise_AllDeclaredToolsCoveredByBakeStep — sanity check that the baseline
// `mise install` step exists; downstream reshim picks up whatever it produces.
func TestMise_AllDeclaredToolsCoveredByBakeStep(t *testing.T) {
	declared := declaredMiseTools(t)
	if len(declared) == 0 {
		t.Fatal("no devcell.mise.tools.* declarations found across nixhome modules — baseline broken")
	}

	dockerfile := readDockerfile(t)
	if !strings.Contains(dockerfile, "mise install") {
		t.Fatal("Dockerfile missing `mise install` step — declared tools won't be installed at build time")
	}

	t.Logf("declared mise tools (%d): %v", len(declared), declared)
}

// ---------------------------------------------------------------------------
// L2 — Container behavior (requires docker; skip otherwise)
// ---------------------------------------------------------------------------

// TestMise_DeclaredToolsOnPATH asserts every declared mise tool resolves on
// PATH inside a fresh container. This is the user-visible bug being fixed.
func TestMise_DeclaredToolsOnPATH(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)
	declared := declaredMiseTools(t)

	for _, tool := range declared {
		bin := miseBinaryName(tool)
		t.Run(tool, func(t *testing.T) {
			out, code := exec(t, c, []string{"bash", "-lc", "command -v " + bin})
			if code != 0 || strings.TrimSpace(out) == "" {
				t.Fatalf("declared mise tool %q (binary %q) not on PATH: out=%q exit=%d", tool, bin, out, code)
			}
			t.Logf("%s → %s", tool, strings.TrimSpace(out))
		})
	}
}

// TestMise_TwoLevelShims_BakedDirExists asserts the image-level baked shim
// dir exists with shims for every declared tool.
func TestMise_TwoLevelShims_BakedDirExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	out, code := exec(t, c, []string{"ls", "/opt/devcell/.local/share/mise/shims"})
	if code != 0 {
		t.Fatalf("baked shim dir missing from image: exit=%d out=%q", code, out)
	}
	files := strings.Fields(out)
	if len(files) == 0 {
		t.Fatal("baked shim dir is empty — mise reshim didn't generate any shims at build time")
	}

	declared := declaredMiseTools(t)
	for _, tool := range declared {
		bin := miseBinaryName(tool)
		found := false
		for _, f := range files {
			if f == bin {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("declared tool %q (binary %q) missing from baked shim dir; have: %v", tool, bin, files)
		}
	}
}

// TestMise_TerraformAndOpentofuOnPATH pins the specific bug (terraform and
// opentofu shims silently missing pre-fix). Named explicitly so future
// readers can grep for the regression.
func TestMise_TerraformAndOpentofuOnPATH(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	for _, bin := range []string{"terraform", "tofu"} {
		bin := bin
		t.Run(bin, func(t *testing.T) {
			out, code := exec(t, c, []string{"bash", "-lc", "command -v " + bin})
			if code != 0 || strings.TrimSpace(out) == "" {
				t.Fatalf("%s missing from PATH (this was the pre-DIMM-214 bug): out=%q exit=%d", bin, out, code)
			}
		})
	}
}

// TestMise_PATHOrder_UserBeforeBaked asserts that user shim dir precedes
// baked shim dir in the runtime PATH.
func TestMise_PATHOrder_UserBeforeBaked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	out, code := exec(t, c, []string{"bash", "-lc", "echo $PATH"})
	if code != 0 {
		t.Fatalf("echo $PATH failed: exit=%d", code)
	}
	path := out

	bakedIdx := strings.Index(path, "/opt/devcell/.local/share/mise/shims")
	if bakedIdx == -1 {
		t.Fatalf("baked shim dir not on PATH: %s", path)
	}
	// Find user shim dir occurrence that is NOT the baked path string.
	// Pattern: ".local/share/mise/shims" appearing without "/opt/devcell" prefix.
	userIdx := -1
	for searchStart := 0; ; {
		i := strings.Index(path[searchStart:], ".local/share/mise/shims")
		if i == -1 {
			break
		}
		abs := searchStart + i
		// Check the preceding 12 chars don't form "/opt/devcell"
		startCtx := abs - 12
		if startCtx < 0 {
			startCtx = 0
		}
		if !strings.Contains(path[startCtx:abs], "/opt/devcell") {
			userIdx = abs
			break
		}
		searchStart = abs + 1
	}
	if userIdx == -1 {
		t.Fatalf("user shim dir not on PATH (only baked found): %s", path)
	}
	if userIdx >= bakedIdx {
		t.Fatalf("user shim dir must come before baked in PATH; userIdx=%d bakedIdx=%d PATH=%s", userIdx, bakedIdx, path)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func readNixhomeFile(t *testing.T, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "nixhome", relPath))
	if err != nil {
		t.Fatalf("read nixhome/%s: %v", relPath, err)
	}
	return string(data)
}

// readImagesDockerfile returns the contents of images/Dockerfile (impure build path).
func readImagesDockerfile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "images", "Dockerfile"))
	if err != nil {
		t.Fatalf("read images/Dockerfile: %v", err)
	}
	return string(data)
}

// declaredMiseTools parses nixhome/modules/*.nix for `devcell.mise.tools.<name> = "..."`
// declarations and returns the list of tool names.
func declaredMiseTools(t *testing.T) []string {
	t.Helper()
	modulesDir := filepath.Join("..", "nixhome", "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		t.Fatalf("read nixhome/modules: %v", err)
	}
	// Match `devcell.mise.tools.<name> = "..."` at line start (optional whitespace)
	// — commented-out lines (e.g. `# devcell.mise.tools.python = "3.13.2";` in
	// python.nix) must NOT match, otherwise tests assert against tools that are
	// not in home.packages mise.tools and have no shim.
	re := regexp.MustCompile(`(?m)^\s*devcell\.mise\.tools\.([a-z0-9_-]+)\s*=\s*"`)
	seen := map[string]bool{}
	var tools []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nix") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(modulesDir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				tools = append(tools, name)
			}
		}
	}
	return tools
}

// TestMise_DeclaredMiseToolsParser_SkipsCommented exercises the parser regex
// in declaredMiseTools against synthetic input: line-anchored matches must
// skip commented declarations and accept whitespace-indented real ones.
// Locks in the fix for the original bug where the parser matched
// `# devcell.mise.tools.python = ...` and tests then demanded a python
// shim that didn't exist in the image's bake step.
func TestMise_DeclaredMiseToolsParser_SkipsCommented(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{"plain declaration", `devcell.mise.tools.go = "1.26.0";`, true},
		{"indented declaration", `  devcell.mise.tools.terraform = "1.14.3";`, true},
		{"tab-indented declaration", "\tdevcell.mise.tools.node = \"24.13.1\";", true},
		{"hash-commented", `# devcell.mise.tools.python = "3.13.2";`, false},
		{"hash-commented indented", `  # devcell.mise.tools.python = "3.13.2";`, false},
		{"hash with no space", `#devcell.mise.tools.python = "3.13.2";`, false},
		{"mid-line text not at start", `foo = devcell.mise.tools.bar = "1";`, false},
	}
	re := regexp.MustCompile(`(?m)^\s*devcell\.mise\.tools\.([a-z0-9_-]+)\s*=\s*"`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := re.MatchString(tc.input)
			if got != tc.wantMatch {
				t.Fatalf("regex match on %q: got %v, want %v", tc.input, got, tc.wantMatch)
			}
		})
	}
}

// miseBinaryName maps a mise tool key to the binary name when they differ.
func miseBinaryName(miseTool string) string {
	switch miseTool {
	case "opentofu":
		return "tofu"
	default:
		return miseTool
	}
}
