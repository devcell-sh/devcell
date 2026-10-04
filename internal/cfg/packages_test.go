package cfg_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestLoadFile_PackagesNixStableIsDeprecated(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[packages.nix]\nstable = [\"htop\"]\nunstable = [\"uv\"]\n")
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(deprecatedNames(c), ","); got != "[packages.nix] stable" {
		t.Errorf("deprecated = %q, want [packages.nix] stable only", got)
	}
	if strings.Join(c.Packages.Nix.Stable, ",") != "htop" {
		t.Errorf("stable = %v, the old key must keep working", c.Packages.Nix.Stable)
	}
}

// Listing a package in a newer channel is a request for the newer version.
func TestResolveNixChannels_NewerChannelWins(t *testing.T) {
	got, overrides := cfg.ResolveNixChannels(cfg.NixPackages{
		Stable:   []string{"jq", "uv", "git"},
		Unstable: []string{"uv", "git"},
		Edge:     []string{"git"},
	})
	if s := strings.Join(got.Stable, ","); s != "jq" {
		t.Errorf("stable = %q, want jq", s)
	}
	if s := strings.Join(got.Unstable, ","); s != "uv" {
		t.Errorf("unstable = %q, want uv", s)
	}
	if s := strings.Join(got.Edge, ","); s != "git" {
		t.Errorf("edge = %q, want git", s)
	}
	want := "uv from unstable (overrides stable); git from edge (overrides stable, unstable)"
	if s := strings.Join(overrides, "; "); s != want {
		t.Errorf("overrides = %q, want %q", s, want)
	}
}

func TestValidateNixPackages_DuplicatesAcrossChannelsAreAllowed(t *testing.T) {
	if err := cfg.ValidateNixPackages(cfg.NixPackages{Stable: []string{"uv"}, Unstable: []string{"uv"}}); err != nil {
		t.Errorf("newer channel wins, duplicates must not error: %v", err)
	}
}

// A project upgrades a globally listed package without editing the global file.
func TestLoadLayered_ProjectChannelOverridesGlobalPackage(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[cell]\npackages = [\"uv\", \"jq\"]\n")
	project := writeTOML(t, dir, "project.toml", "[packages.nix]\nunstable = [\"uv\"]\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateNixPackages(c.Packages.Nix); err != nil {
		t.Fatal(err)
	}
	np, _ := cfg.ResolveNixChannels(c.Packages.Nix)
	if strings.Join(np.Stable, ",") != "jq" || strings.Join(np.Unstable, ",") != "uv" {
		t.Errorf("resolved = %+v, want uv only in unstable", np)
	}
}

// [cell] packages = [] in a project clears the global list, as
// [packages.nix] stable = [] did.
func TestLoadLayered_EmptyCellPackagesClearsGlobal(t *testing.T) {
	dir := t.TempDir()
	global := writeTOML(t, dir, "global.toml", "[cell]\npackages = [\"htop\"]\n")
	project := writeTOML(t, dir, "project.toml", "[cell]\npackages = []\n")
	c, err := cfg.LoadLayered(global, project, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Packages.Nix.Stable) != 0 {
		t.Errorf("stable = %v, an explicit empty list must clear the global one", c.Packages.Nix.Stable)
	}
}

// [packages.python] and [packages.npm] become a mise [tools] table that the
// container installs at start: pipx:<name> (one venv per tool) and npm:<name>.
func TestPackagesSection_MiseTools(t *testing.T) {
	p := cfg.PackagesSection{
		Python: map[string]string{"ruff": "*", "black": "24.1"},
		Node:   map[string]string{"prettier": "^3", "@scope/tool": ""},
	}
	want := `[tools]
"npm:@scope/tool" = "latest"
"npm:prettier" = "^3"
"pipx:black" = "24.1"
"pipx:ruff" = "latest"
`
	if got := p.MiseTools(); got != want {
		t.Errorf("MiseTools() =\n%s\nwant\n%s", got, want)
	}
	if got := (cfg.PackagesSection{}).MiseTools(); got != "" {
		t.Errorf("no packages must render nothing, got %q", got)
	}
}

// mise pins pipx tools with ==, so PEP 440 ranges fail at container start.
// Reject them at load with the syntax mise does accept.
func TestLoadFile_PythonVersionRangeIsError(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", "[packages.python]\n\"ruff\" = \">=0.6\"\n")
	_, err := cfg.LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), `"latest"`) {
		t.Errorf("err = %v, want a hint with mise version syntax", err)
	}
	for _, ok := range []string{"*", "latest", "0.6", "0.6.9"} {
		p := writeTOML(t, t.TempDir(), "test.toml", "[packages.python]\n\"ruff\" = \""+ok+"\"\n")
		if _, err := cfg.LoadFile(p); err != nil {
			t.Errorf("%q must be accepted: %v", ok, err)
		}
	}
}

// [packages.npm] was renamed to [packages.node], matching the node module.
func TestLoadFile_PackagesNpmIsDeprecatedAlias(t *testing.T) {
	p := writeTOML(t, t.TempDir(), "test.toml", `
[packages.node]
"prettier" = "^3"

[packages.npm]
"prettier" = "^2"
"eslint" = "9"
`)
	c, err := cfg.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(deprecatedNames(c), ","); got != "[packages.npm]" {
		t.Errorf("deprecated = %q, want [packages.npm]", got)
	}
	if c.Packages.Node["prettier"] != "^3" || c.Packages.Node["eslint"] != "9" {
		t.Errorf("node = %v, want [packages.node] to win and npm-only keys kept", c.Packages.Node)
	}
	want := "[tools]\n\"npm:eslint\" = \"9\"\n\"npm:prettier\" = \"^3\"\n"
	if got := c.Packages.MiseTools(); got != want {
		t.Errorf("MiseTools() = %q, want %q", got, want)
	}
}
