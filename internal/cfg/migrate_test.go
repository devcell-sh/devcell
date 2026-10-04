package cfg_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func migrate(t *testing.T, src string) cfg.MigrationResult {
	t.Helper()
	res, err := cfg.MigrateTOML(src)
	if err != nil {
		t.Fatalf("MigrateTOML: %v\n--- input ---\n%s", err, src)
	}
	return res
}

func TestMigrateTOML_NoDeprecations_IsNoop(t *testing.T) {
	src := "# mine\n[cell]\nstack = \"go\" # keep\n\n[gui]\nenabled = false\n"
	res := migrate(t, src)
	if res.After != src || len(res.Changes) != 0 {
		t.Fatalf("want untouched output, got %d changes:\n%s", len(res.Changes), res.After)
	}
}

func TestMigrateTOML_RemovesRetiredKeys_KeepsNeighbours(t *testing.T) {
	src := "[cell]\n# build mode\nthin = true\nstack = \"go\"\nlibvirt_uri = \"qemu:///system\"\nqemu_display = \"cocoa\"\n"
	res := migrate(t, src)
	want := "[cell]\n# build mode\nstack = \"go\"\n"
	if res.After != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.After, want)
	}
	if len(res.Changes) != 3 {
		t.Errorf("want 3 changes, got %+v", res.Changes)
	}
}

func TestMigrateTOML_RenamesKeyInPlace(t *testing.T) {
	src := "[cell]\nqemu_ssh_port = 3333 # forwarded\nqemu_windows_iso = \"~/w.iso\"\nstack = \"go\"\n"
	res := migrate(t, src)
	want := "[cell]\nwinkit_ssh_port = 3333 # forwarded\nwinkit_windows_iso = \"~/w.iso\"\nstack = \"go\"\n"
	if res.After != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.After, want)
	}
}

func TestMigrateTOML_MovesKeyToNewTable(t *testing.T) {
	src := "[cell]\ngui = false\nnixhome = \"~/dev/home\"\n"
	res := migrate(t, src)
	after := res.After
	for _, want := range []string{"[gui]\nenabled = false\n", "[nix]\nnixhome = \"~/dev/home\"\n"} {
		if !strings.Contains(after, want) {
			t.Errorf("want %q in:\n%s", want, after)
		}
	}
	if strings.Contains(after, "gui = false") || strings.Contains(after, "\nnixhome = \"~/dev/home\"\n[") {
		t.Errorf("old keys must be gone from [cell]:\n%s", after)
	}
}

func TestMigrateTOML_MovesIntoExistingTable_AndDropsEmptyOldTable(t *testing.T) {
	src := "[cell]\nstack = \"go\"\n\n[ports]\nforward = [\"3000\"]\n\n[mcp]\nenabled = [\"playwright\"]\n"
	res := migrate(t, src)
	after := res.After
	if !strings.Contains(after, "[cell]\nstack = \"go\"\nports = [\"3000\"]\nmcps = [\"playwright\"]\n") {
		t.Errorf("moved keys must land under the existing [cell]:\n%s", after)
	}
	if strings.Contains(after, "[ports]") || strings.Contains(after, "[mcp]") {
		t.Errorf("emptied tables must be dropped:\n%s", after)
	}
}

func TestMigrateTOML_KeepsTableThatStillHasKeys(t *testing.T) {
	src := "[ports]\nforward = [\"3000\"]\npublish_ip = \"127.0.0.1\"\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "[ports]\npublish_ip = \"127.0.0.1\"\n") {
		t.Errorf("publish_ip must stay under [ports]:\n%s", res.After)
	}
}

func TestMigrateTOML_MergesArraysWhenTargetExists(t *testing.T) {
	src := "[cell]\npackages = [\"htop\"]\n\n[packages.nix]\nstable = [\n  \"jq\",\n  \"ripgrep\",\n]\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "packages = [\"htop\", \"jq\", \"ripgrep\"]") {
		t.Errorf("want merged array:\n%s", res.After)
	}
	if strings.Contains(res.After, "[packages.nix]") {
		t.Errorf("emptied [packages.nix] must be dropped:\n%s", res.After)
	}
}

func TestMigrateTOML_MultiLineValueMovesIntact(t *testing.T) {
	src := "[mcp]\nenabled = [\n  \"playwright\", # browser\n  \"aws-api\",\n]\n"
	res := migrate(t, src)
	want := "[cell]\nmcps = [\n  \"playwright\", # browser\n  \"aws-api\",\n]\n"
	if !strings.Contains(res.After, want) {
		t.Errorf("got:\n%s\nwant to contain:\n%s", res.After, want)
	}
}

func TestMigrateTOML_VolumesTablesBecomeCellVolumes(t *testing.T) {
	src := "[cell]\nstack = \"go\"\n\n[[volumes]]\nmount = \"~/work/secrets:/run/secrets:ro\"\n\n[[volumes]]\nmount = \"/data\"\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "volumes = [\"~/work/secrets:/run/secrets:ro\", \"/data\"]") {
		t.Errorf("want [cell] volumes array:\n%s", res.After)
	}
	if strings.Contains(res.After, "[[volumes]]") {
		t.Errorf("array tables must be gone:\n%s", res.After)
	}
}

func TestMigrateTOML_RenamesTablesAndSubtables(t *testing.T) {
	src := "[op]\ndocuments = [\"prod\"]\n\n[packages.npm]\n\"prettier\" = \"^3\"\n\n[llm.models.providers.ollama]\nbase_url = \"http://x\"\n"
	res := migrate(t, src)
	for _, want := range []string{"[secrets.onepassword]\ndocuments = [\"prod\"]\n", "[packages.node]\n\"prettier\" = \"^3\"\n", "[llm.providers.ollama]\nbase_url = \"http://x\"\n"} {
		if !strings.Contains(res.After, want) {
			t.Errorf("want %q in:\n%s", want, res.After)
		}
	}
}

func TestMigrateTOML_LLMFlagsBecomeProviderAndModel(t *testing.T) {
	src := "[llm]\nuse_ollama = true\nmodel = \"ollama/qwen3:8b\"\n\n[llm.models]\ndefault = \"qwen3:8b\"\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "provider = \"ollama\"") || !strings.Contains(res.After, "model = \"qwen3:8b\"") {
		t.Errorf("want provider + bare model:\n%s", res.After)
	}
	if strings.Contains(res.After, "use_ollama") || strings.Contains(res.After, "[llm.models]") || strings.Contains(res.After, "default =") {
		t.Errorf("legacy llm keys must be gone:\n%s", res.After)
	}
}

func TestMigrateTOML_ModelsDefaultBecomesModel(t *testing.T) {
	src := "[llm]\nuse_openrouter = true\n\n[llm.models]\ndefault = \"deepseek/deepseek-v4-pro\"\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "[llm]\nprovider = \"openrouter\"\nmodel = \"deepseek/deepseek-v4-pro\"\n") {
		t.Errorf("got:\n%s", res.After)
	}
}

// The rewritten text must load with no deprecations left and the same
// effective values the old spelling produced.
func TestMigrateTOML_KitchenSink_LoadsClean(t *testing.T) {
	src := `# global config
[cell]
stack = "ultimate"   # keep
thin = true
qemu_ssh_port = 3333  # ssh
gui = false
nixhome = "~/dev/home"
libvirt_uri = "qemu:///system"

[[volumes]]
mount = "~/work/secrets:/run/secrets:ro"

[[volumes]]
mount = "/data"

[ports]
forward = ["3000", "8080:3000"]
publish_ip = "127.0.0.1"

[mcp]
enabled = ["playwright"]

[op]
documents = ["prod"]

[packages.npm]
"prettier" = "^3"

[packages.nix]
stable = [
  "htop",
  "jq",
]

[llm]
use_ollama = true
model = "ollama/qwen3:8b"

[llm.models.providers.ollama]
base_url = "http://x"
models = ["qwen3:8b"]
`
	res := migrate(t, src)
	path := filepath.Join(t.TempDir(), "devcell.toml")
	if err := os.WriteFile(path, []byte(res.After), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := cfg.LoadFile(path)
	if err != nil {
		t.Fatalf("migrated config does not load: %v\n%s", err, res.After)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("deprecations remain: %+v\n%s", c.DeprecatedUses, res.After)
	}
	if c.Cell.Stack != "ultimate" || c.Cell.WinkitSSHPort != 3333 || c.Nix.NixhomePath != "~/dev/home" {
		t.Errorf("cell values: %+v nix=%+v", c.Cell, c.Nix)
	}
	if c.GUI.Enabled == nil || *c.GUI.Enabled {
		t.Errorf("gui must be disabled: %+v", c.GUI)
	}
	if len(c.Volumes) != 2 || len(c.Cell.Ports) != 2 || len(c.Cell.Mcps) != 1 {
		t.Errorf("volumes=%d ports=%d mcps=%d", len(c.Volumes), len(c.Cell.Ports), len(c.Cell.Mcps))
	}
	if c.Ports.PublishIP != "127.0.0.1" {
		t.Errorf("publish_ip lost: %q", c.Ports.PublishIP)
	}
	if len(c.Secrets.OnePassword.Documents) != 1 || c.Packages.Node["prettier"] != "^3" {
		t.Errorf("secrets=%+v node=%+v", c.Secrets.OnePassword, c.Packages.Node)
	}
	if strings.Join(c.Cell.Packages, ",") != "htop,jq" {
		t.Errorf("packages: %v", c.Cell.Packages)
	}
	if c.LLM.Provider != "ollama" || c.LLM.Model != "qwen3:8b" || c.LLM.Providers["ollama"].BaseURL != "http://x" {
		t.Errorf("llm: %+v", c.LLM)
	}
	if !strings.Contains(res.After, "# global config") || !strings.Contains(res.After, "# keep") || !strings.Contains(res.After, "# ssh") {
		t.Errorf("comments must survive:\n%s", res.After)
	}
}

func TestMigrateTOML_RefusesWhenResultStillInvalid(t *testing.T) {
	// use_ollama and use_openrouter together is a load error before and
	// after migration; the migrator must surface it, not write a broken file.
	if _, err := cfg.MigrateTOML("[llm]\nuse_ollama = true\nuse_openrouter = true\n"); err == nil {
		t.Fatal("want an error for a config that cannot be migrated cleanly")
	}
}

func TestMigrateFile_WritesBackupAndResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devcell.toml")
	if err := os.WriteFile(path, []byte("[cell]\nqemu_cpus = 4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, backup, err := cfg.MigrateFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if backup != "" || len(res.Changes) != 1 {
		t.Errorf("dry run must not back up: backup=%q changes=%+v", backup, res.Changes)
	}
	if b, _ := os.ReadFile(path); string(b) != "[cell]\nqemu_cpus = 4\n" {
		t.Errorf("dry run must not write: %q", b)
	}

	res, backup, err = cfg.MigrateFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "[cell]\nwinkit_cpus = 4\n" {
		t.Errorf("file not rewritten: %q", b)
	}
	if b, err := os.ReadFile(backup); err != nil || string(b) != "[cell]\nqemu_cpus = 4\n" {
		t.Errorf("backup %q: %v %q", backup, err, b)
	}
	if !strings.HasPrefix(filepath.Base(backup), "devcell.toml.bak-") {
		t.Errorf("backup name: %q", backup)
	}

	res, backup, err = cfg.MigrateFile(path, true)
	if err != nil || backup != "" || len(res.Changes) != 0 {
		t.Errorf("second run must be a no-op: backup=%q changes=%+v err=%v", backup, res.Changes, err)
	}
}

func TestMigrateFile_ReadOnlyTargetLeavesNoBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devcell.toml")
	if err := os.WriteFile(path, []byte("[cell]\nthin = true\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	_, backup, err := cfg.MigrateFile(path, true)
	if err == nil || !errors.Is(err, cfg.ErrReadOnly) {
		t.Fatalf("want ErrReadOnly, got %v", err)
	}
	if backup != "" {
		t.Errorf("no backup path expected, got %q", backup)
	}
	if matches, _ := filepath.Glob(path + ".bak-*"); len(matches) != 0 {
		t.Errorf("no backup file expected, got %v", matches)
	}
}

// When the legacy keys do not say which provider was meant (no use_* flag
// and no ollama/ or openrouter/ prefix), the upgrade writes the provider
// out explicitly as "default" rather than leaving it implicit.
func TestMigrateTOML_UnclearProviderBecomesDefault(t *testing.T) {
	src := "[llm.models]\ndefault = \"deepseek/deepseek-v4-pro\"\n"
	res := migrate(t, src)
	if !strings.Contains(res.After, "[llm]\nprovider = \"default\"\nmodel = \"deepseek/deepseek-v4-pro\"\n") {
		t.Errorf("got:\n%s", res.After)
	}
	found := false
	for _, ch := range res.Changes {
		if strings.Contains(ch.Action, `provider = "default"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("want a change noting the explicit default provider, got %+v", res.Changes)
	}
}

// An existing provider, or one derived from a flag or prefix, is never
// overwritten by the default.
func TestMigrateTOML_ClearProviderIsKept(t *testing.T) {
	res := migrate(t, "[llm]\nprovider = \"openrouter\"\n\n[llm.models]\ndefault = \"deepseek/x\"\n")
	if strings.Contains(res.After, `"default"`) || !strings.Contains(res.After, `provider = "openrouter"`) {
		t.Errorf("got:\n%s", res.After)
	}
	res = migrate(t, "[llm.models]\ndefault = \"ollama/qwen3:8b\"\n")
	if !strings.Contains(res.After, `provider = "default"`) {
		t.Errorf("prefix must not activate provider (want default):\n%s", res.After)
	}
	if !strings.Contains(res.After, `model = "qwen3:8b"`) {
		t.Errorf("prefix must be stripped from model:\n%s", res.After)
	}
}

// A config with no deprecated llm keys is not touched, even without provider.
func TestMigrateTOML_NoLLMChange_NoProviderAdded(t *testing.T) {
	src := "[llm]\nmodel = \"deepseek/x\"\n"
	res := migrate(t, src)
	if len(res.Changes) != 0 || res.After != src {
		t.Errorf("got %+v:\n%s", res.Changes, res.After)
	}
}

// A config that is a symlink into a read-only location (home-manager links
// ~/.config/devcell/devcell.toml into /nix/store) cannot be rewritten in
// place anywhere, not even on the host. The error names the link target so
// the caller can point at the source, and the result still carries the
// changes so they can be shown.
func TestMigrateFile_ReadOnlySymlinkReportsTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "store", "abc-home-manager-files", "devcell.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("[cell]\nthin = true\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "devcell.toml")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	res, backup, err := cfg.MigrateFile(path, true)
	var ro *cfg.ReadOnlySymlinkError
	if !errors.As(err, &ro) {
		t.Fatalf("want ReadOnlySymlinkError, got %v", err)
	}
	if ro.Target != target {
		t.Errorf("target: want %q, got %q", target, ro.Target)
	}
	if !errors.Is(err, cfg.ErrReadOnly) {
		t.Errorf("must still be an ErrReadOnly: %v", err)
	}
	if len(res.Changes) == 0 {
		t.Errorf("want the changes reported even though nothing was written")
	}
	if backup != "" {
		t.Errorf("no backup path expected, got %q", backup)
	}
	if matches, _ := filepath.Glob(path + ".bak-*"); len(matches) != 0 {
		t.Errorf("no backup file expected, got %v", matches)
	}
}

// A plain read-only file also reports its changes, so a skip row can show
// what would have been rewritten.
func TestMigrateFile_ReadOnlyTargetReportsChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devcell.toml")
	if err := os.WriteFile(path, []byte("[cell]\nthin = true\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	res, _, err := cfg.MigrateFile(path, true)
	if !errors.Is(err, cfg.ErrReadOnly) {
		t.Fatalf("want ErrReadOnly, got %v", err)
	}
	if len(res.Changes) == 0 {
		t.Errorf("want the changes reported even though nothing was written")
	}
}

// Commented-out examples written in the old syntax are rewritten too, so
// uncommenting one later does not bring a deprecated key back. Prose
// comments are left alone.
func TestMigrateTOML_RefreshesCommentedExamples(t *testing.T) {
	src := `[cell]
stack = "go"
# Disable GUI (Xvfb + VNC + browser). GUI is enabled by default.
# gui = false
# qemu_ssh_port = 2222
# libvirt_uri = "qemu:///system"

# [llm]
# use_ollama = false

# [llm.models]
# default = "deepseek/deepseek-v4-pro"

# [llm.models.providers.ollama]
# models = ["qwen3:8b"]

# Extra volume mounts appended to docker run.
# [[volumes]]
# mount = "~/work/secrets:/run/secrets:ro"
#
# [[volumes]]
# mount = "~/.ssh:/home/user/.ssh:ro"

# [packages.npm]
# "some-tool" = "^1.0.0"
`
	res := migrate(t, src)
	for _, want := range []string{
		"# Disable GUI (Xvfb + VNC + browser). GUI is enabled by default.\n",
		"# [gui]\n# enabled = false\n",
		"# winkit_ssh_port = 2222\n",
		"# [llm]\n# provider = \"ollama\"\n",
		"# [llm]\n# model = \"deepseek/deepseek-v4-pro\"\n",
		"# [llm.providers.ollama]\n# models = [\"qwen3:8b\"]\n",
		"# Extra volume mounts appended to docker run.\n# [cell]\n# volumes = [\"~/work/secrets:/run/secrets:ro\", \"~/.ssh:/home/user/.ssh:ro\"]\n",
		"# [packages.node]\n# \"some-tool\" = \"^1.0.0\"\n",
	} {
		if !strings.Contains(res.After, want) {
			t.Errorf("want %q in:\n%s", want, res.After)
		}
	}
	for _, gone := range []string{"gui = false\n# qemu", "qemu_ssh_port", "libvirt_uri", "use_ollama", "[llm.models]", "default =", "[[volumes]]", "mount =", "packages.npm"} {
		if strings.Contains(res.After, gone) {
			t.Errorf("stale example %q must be gone:\n%s", gone, res.After)
		}
	}
	found := false
	for _, ch := range res.Changes {
		if ch.Key == "comments" && strings.Contains(ch.Action, "9 ") {
			found = true
		}
	}
	if !found {
		t.Errorf("want one 'comments' change counting the 9 rewritten examples, got %+v", res.Changes)
	}
}

// Stale comments alone are reason enough to rewrite the file.
func TestMigrateTOML_StaleCommentsAloneAreAChange(t *testing.T) {
	src := "[cell]\nstack = \"go\"\n# thin = true\n"
	res := migrate(t, src)
	if len(res.Changes) != 1 || res.Changes[0].Key != "comments" {
		t.Fatalf("want exactly the comments change, got %+v", res.Changes)
	}
	if res.After != "[cell]\nstack = \"go\"\n" {
		t.Errorf("retired example must be dropped:\n%s", res.After)
	}
}

// A commented example that is already current, or a prose line that merely
// mentions a key, is not a change.
func TestMigrateTOML_CurrentCommentsAreNoop(t *testing.T) {
	src := "[cell]\nstack = \"go\"\n# winkit_ssh_port = 2222\n# volumes = [\"~/data:/data:ro\"]\n# Set gui = false in [gui] to turn the desktop off.\n\n# [gui]\n# enabled = true\n"
	res := migrate(t, src)
	if len(res.Changes) != 0 || res.After != src {
		t.Errorf("want a noop, got %+v:\n%s", res.Changes, res.After)
	}
}

// home-manager links ~/.config/devcell/devcell.toml in two hops: the first
// into <hash>-home-manager-files/, the second from there to the per-file
// store path. Only the first hop names home-manager, so the detection must
// not rely on the fully resolved target.
func TestMigrateFile_ReadOnlySymlinkChain_DetectsHomeManager(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "nix", "store", "7rn1-devcell.toml")
	hm := filepath.Join(dir, "nix", "store", "abc-home-manager-files", ".config", "devcell", "devcell.toml")
	for _, d := range []string{filepath.Dir(final), filepath.Dir(hm)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(final, []byte("[cell]\nthin = true\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(final, hm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "devcell.toml")
	if err := os.Symlink(hm, path); err != nil {
		t.Fatal(err)
	}
	_, _, err := cfg.MigrateFile(path, true)
	var ro *cfg.ReadOnlySymlinkError
	if !errors.As(err, &ro) {
		t.Fatalf("want ReadOnlySymlinkError, got %v", err)
	}
	if !ro.HomeManaged() {
		t.Errorf("two-hop home-manager link must be detected: link=%q target=%q", ro.Link, ro.Target)
	}
	if ro.Target != final {
		t.Errorf("target: want %q, got %q", final, ro.Target)
	}
}
