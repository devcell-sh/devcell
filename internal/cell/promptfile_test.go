package cell

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
)

// promptFileConfig mirrors sampleConfig() but points BaseDir at a temp dir so
// the writer has somewhere real to land, and carries a CellName so the
// per-cell namespacing is exercised.
func promptFileConfig(t *testing.T, cellName string) config.Config {
	t.Helper()
	wsHome := t.TempDir()
	return config.Config{
		CellName:      "devcell-85",
		BaseDir:       t.TempDir(),
		WorkspaceHome: wsHome,
		WorkspaceName: cellName,
		HostUser:      "warden",
		HostHome:      "/Users/warden",
	}
}

func TestWritePromptFile_WritesContentToWorkspaceHome(t *testing.T) {
	c := promptFileConfig(t, "main")

	if _, err := WritePromptFile(c, "additional-systemprompt.md", "hello prompt\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hostPath := filepath.Join(c.WorkspaceHome, "prompts", "main", "additional-systemprompt.md")
	got, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("expected file at %s: %v", hostPath, err)
	}
	if string(got) != "hello prompt\n" {
		t.Errorf("content mismatch:\n got %q\nwant %q", got, "hello prompt\n")
	}
}

func TestWritePromptFile_ReturnsContainerHomePath(t *testing.T) {
	c := promptFileConfig(t, "main")

	got, err := WritePromptFile(c, "additional-systemprompt.md", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/home/warden/prompts/main/additional-systemprompt.md"
	if got != want {
		t.Errorf("container path:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(got, c.BaseDir) {
		t.Errorf("returned path must not contain host BaseDir %q", c.BaseDir)
	}
	if strings.Contains(got, c.WorkspaceHome) {
		t.Errorf("returned path must not contain host WorkspaceHome %q", c.WorkspaceHome)
	}
}

// Two cells open on the same project must not clobber each other's file:
// ContainerContext embeds the cell name, so a shared path would let one cell
// boot with another's container context.
func TestWritePromptFile_NamespacedPerCell(t *testing.T) {
	wsHome := t.TempDir()
	mk := func(cell string) config.Config {
		return config.Config{CellName: "devcell-85", BaseDir: t.TempDir(), WorkspaceHome: wsHome, WorkspaceName: cell, HostUser: "warden"}
	}

	pathA, err := WritePromptFile(mk("alpha"), "additional-systemprompt.md", "content-alpha")
	if err != nil {
		t.Fatalf("alpha: %v", err)
	}
	pathB, err := WritePromptFile(mk("beta"), "additional-systemprompt.md", "content-beta")
	if err != nil {
		t.Fatalf("beta: %v", err)
	}

	if pathA == pathB {
		t.Fatalf("both cells resolved to the same container path %q", pathA)
	}

	for _, tc := range []struct{ cell, want string }{
		{"alpha", "content-alpha"},
		{"beta", "content-beta"},
	} {
		got, err := os.ReadFile(filepath.Join(wsHome, "prompts", tc.cell, "additional-systemprompt.md"))
		if err != nil {
			t.Fatalf("%s: %v", tc.cell, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s clobbered: got %q want %q", tc.cell, got, tc.want)
		}
	}
}

// Re-running a cell must overwrite, not append or fail.
func TestWritePromptFile_OverwritesExisting(t *testing.T) {
	c := promptFileConfig(t, "main")

	if _, err := WritePromptFile(c, "additional-systemprompt.md", "first run"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := WritePromptFile(c, "additional-systemprompt.md", "second"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(c.WorkspaceHome, "prompts", "main", "additional-systemprompt.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("expected overwrite, got %q", got)
	}
}

// An unset cell name must still produce a usable path rather than a directory
// with an empty segment.
func TestWritePromptFile_EmptyCellNameFallsBack(t *testing.T) {
	c := promptFileConfig(t, "")

	got, err := WritePromptFile(c, "additional-systemprompt.md", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/warden/prompts/main/additional-systemprompt.md"
	if got != want {
		t.Errorf("empty cell name should fall back to %q, got %q", want, got)
	}
}

// WriteOverlayPrompt is the seam both surfaces use: assemble container
// context + resolved prompt, materialize it, hand back the container path.
func TestWriteOverlayPrompt_WritesAssembledContent(t *testing.T) {
	c := promptFileConfig(t, "main")

	got, err := WriteOverlayPrompt(c, cfg.CellConfig{}, ResolveOpts{AppendFlagInline: "be terse"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home/warden/prompts/main/additional-systemprompt.md" {
		t.Errorf("container path = %q", got)
	}

	body, err := os.ReadFile(filepath.Join(c.WorkspaceHome, "prompts", "main", "additional-systemprompt.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "Docker container") {
		t.Error("overlay file missing container context")
	}
	if !strings.Contains(content, "be terse") {
		t.Error("overlay file missing resolved prompt")
	}
	if strings.Index(content, "be terse") <= strings.Index(content, "Docker container") {
		t.Error("container context must come before the resolved prompt")
	}
}

// With nothing configured the overlay still exists — container context is
// always present — so the flag is always emitted.
func TestWriteOverlayPrompt_ContextOnlyWhenNothingConfigured(t *testing.T) {
	c := promptFileConfig(t, "main")

	if _, err := WriteOverlayPrompt(c, cfg.CellConfig{}, ResolveOpts{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(c.WorkspaceHome, "prompts", "main", "additional-systemprompt.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "Docker container") {
		t.Error("overlay file missing container context")
	}
}

func TestWriteOverlayPrompt_ResolverErrorPropagates(t *testing.T) {
	c := promptFileConfig(t, "main")

	_, err := WriteOverlayPrompt(c, cfg.CellConfig{}, ResolveOpts{
		AppendFlagInline: "a",
		AppendFlagFile:   "/nonexistent/b.md",
	})
	if err == nil {
		t.Fatal("expected mutually-exclusive resolver error to propagate")
	}
}

// The base file is only written when a base is actually configured —
// otherwise the stock prompt must stay in effect.
func TestWriteBasePrompt_EmptyWhenUnconfigured(t *testing.T) {
	c := promptFileConfig(t, "main")

	path, err := WriteBasePrompt(c, ResolveOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected no base path when unconfigured, got %q", path)
	}
	if _, statErr := os.Stat(filepath.Join(c.WorkspaceHome, "prompts", "main", "system-prompt.md")); statErr == nil {
		t.Error("base prompt file must not be written when unconfigured")
	}
}

func TestWriteBasePrompt_WritesVerbatimWithoutContainerContext(t *testing.T) {
	c := promptFileConfig(t, "main")

	path, err := WriteBasePrompt(c, ResolveOpts{FlagInline: "you are a release bot"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/home/warden/prompts/main/system-prompt.md" {
		t.Errorf("base container path = %q", path)
	}

	body, err := os.ReadFile(filepath.Join(c.WorkspaceHome, "prompts", "main", "system-prompt.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(body)
	if got != "you are a release bot" {
		t.Errorf("base file = %q, want the resolved prompt verbatim", got)
	}
	// Container context belongs on the overlay: it is regenerated per run and
	// must not be something a user's base prompt can displace.
	if strings.Contains(got, "Docker container") {
		t.Error("container context leaked into the base prompt")
	}
}

// A restrictive umask (0077) strips world-read/execute bits from mkdir(2) and
// open(2). Without the post-creation chmod, the container user (different UID
// from the host user that wrote the file) gets EACCES.
func TestWritePromptFile_WorldReadableUnderRestrictiveUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	c := promptFileConfig(t, "main")

	if _, err := WritePromptFile(c, "additional-systemprompt.md", "test"); err != nil {
		t.Fatalf("WritePromptFile: %v", err)
	}

	promptsDir := filepath.Join(c.WorkspaceHome, "prompts")
	for _, dir := range []string{
		promptsDir,
		filepath.Join(promptsDir, "main"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if info.Mode()&0o005 != 0o005 {
			t.Errorf("dir %s must be world-traversable (other r+x), got %o", dir, info.Mode()&os.ModePerm)
		}
	}

	filePath := filepath.Join(promptsDir, "main", "additional-systemprompt.md")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Mode()&0o004 == 0 {
		t.Errorf("file must be world-readable (other r), got %o", info.Mode()&os.ModePerm)
	}
}

// Pre-existing directories with restrictive permissions (e.g. created by a
// previous build process running as a different user) must also get fixed.
func TestWritePromptFile_FixesRestrictiveExistingDirs(t *testing.T) {
	c := promptFileConfig(t, "main")

	promptsDir := filepath.Join(c.WorkspaceHome, "prompts")
	promptDir := filepath.Join(promptsDir, "main")
	if err := os.MkdirAll(promptDir, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Chmod(promptsDir, 0o700); err != nil {
		t.Fatalf("chmod prompts: %v", err)
	}

	if _, err := WritePromptFile(c, "additional-systemprompt.md", "test"); err != nil {
		t.Fatalf("WritePromptFile: %v", err)
	}

	for _, dir := range []string{
		promptsDir,
		promptDir,
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if info.Mode()&0o005 != 0o005 {
			t.Errorf("dir %s must be world-traversable, got %o", dir, info.Mode()&os.ModePerm)
		}
	}
}

// The overlay must carry container context plus append sources only.
func TestWriteOverlayPrompt_UsesAppendSourcesNotBase(t *testing.T) {
	c := promptFileConfig(t, "main")

	if _, err := WriteOverlayPrompt(c, cfg.CellConfig{}, ResolveOpts{
		FlagInline:      "BASE-TEXT",
		AppendEnvInline: "OVERLAY-TEXT",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(c.WorkspaceHome, "prompts", "main", "additional-systemprompt.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "Docker container") {
		t.Error("overlay missing container context")
	}
	if !strings.Contains(got, "OVERLAY-TEXT") {
		t.Error("overlay missing append-sourced text")
	}
	if strings.Contains(got, "BASE-TEXT") {
		t.Error("base prompt leaked into the overlay")
	}
}
