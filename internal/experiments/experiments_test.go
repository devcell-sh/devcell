package experiments_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/experiments"
)

// Every experiment is named "<group>-<slug>": the group is the area it
// touches (ui, engine) and is derived from the name, never stored twice.
func TestList_NamesCarryGroupPrefix(t *testing.T) {
	seen := map[string]bool{}
	for _, x := range experiments.List() {
		if seen[x.Name] {
			t.Errorf("%s: registered twice", x.Name)
		}
		seen[x.Name] = true
		group, _, ok := strings.Cut(x.Name, "-")
		if !ok || group == "" || x.Group() != group {
			t.Errorf("%s: want <group>-<slug>, Group() = %q", x.Name, x.Group())
		}
		if x.Summary == "" {
			t.Errorf("%s: needs a one-line Summary for docs and the listing", x.Name)
		}
	}
	for _, x := range []experiments.Experiment{experiments.Cage, experiments.BootPopup, experiments.Windows} {
		if !seen[x.Name] {
			t.Errorf("%s is not in List()", x.Name)
		}
	}
	if experiments.Cage.Name != "ui-cage" || experiments.BootPopup.Name != "ui-boot-popup" || experiments.Windows.Name != "engine-windows" {
		t.Errorf("names: %s %s %s", experiments.Cage.Name, experiments.BootPopup.Name, experiments.Windows.Name)
	}
}

func TestResolve_DefaultNothingOn(t *testing.T) {
	set, err := experiments.Resolve("", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range experiments.List() {
		if set.Enabled(x) {
			t.Errorf("%s on by default", x.Name)
		}
	}
}

// The flag and the configured names (TOML plus env, resolved by cfg) are
// unioned. The environment itself is not read here.
func TestResolve_UnionOfFlagAndConfigured(t *testing.T) {
	t.Setenv(experiments.Env, "engine-windows") // must be ignored: cfg reads it, not this package
	set, err := experiments.Resolve("ui-boot-popup", []string{"ui-cage"})
	if err != nil {
		t.Fatal(err)
	}
	if !set.Enabled(experiments.Cage) || !set.Enabled(experiments.BootPopup) || set.Enabled(experiments.Windows) {
		t.Errorf("flag + configured should be on, env must not be read here, got %v", set)
	}
	set, err = experiments.Resolve("ui-cage, ui-boot-popup", nil)
	if err != nil || !set.Enabled(experiments.Cage) || !set.Enabled(experiments.BootPopup) {
		t.Errorf("csv with spaces should enable both, got %v (%v)", set, err)
	}
}

// A typo cannot silently leave a feature off: unknown names are an error
// that names the source and lists what exists.
func TestResolve_UnknownNameIsAnError(t *testing.T) {
	cases := map[string]func() (experiments.Set, error){
		"flag":       func() (experiments.Set, error) { return experiments.Resolve("ui-cage,cage", nil) },
		"configured": func() (experiments.Set, error) { return experiments.Resolve("", []string{"popup"}) },
	}
	for src, run := range cases {
		_, err := run()
		var unknown *experiments.UnknownError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: want *UnknownError, got %v", src, err)
			continue
		}
		if !strings.Contains(err.Error(), "ui-boot-popup") || !strings.Contains(err.Error(), "engine-windows") {
			t.Errorf("%s: error should list the known experiments, got %v", src, err)
		}
	}
	if _, err := experiments.Resolve("", []string{"nope"}); err == nil || !strings.Contains(err.Error(), experiments.Env) {
		t.Errorf("a configured typo should name %s as a possible source, got %v", experiments.Env, err)
	}
}

// Require is the one-line gate for code paths that need an experiment:
// a clear error with every way to turn it on.
func TestRequire(t *testing.T) {
	set, _ := experiments.Resolve("", nil)
	err := experiments.Require(set, experiments.Windows, "the windows guest")
	var need *experiments.RequiredError
	if !errors.As(err, &need) {
		t.Fatalf("want *RequiredError, got %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"the windows guest", "experimental", "--experimental=engine-windows", "DEVCELL_EXPERIMENTAL=engine-windows", `experimental = ["engine-windows"]`} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
	set, _ = experiments.Resolve("engine-windows", nil)
	if err := experiments.Require(set, experiments.Windows, "the windows guest"); err != nil {
		t.Errorf("enabled: want nil, got %v", err)
	}
}

// Markdown is the single source for the README and website sections: one
// row per experiment with name, summary, how to enable, doc link.
func TestMarkdown_OneRowPerExperiment(t *testing.T) {
	md := experiments.Markdown("")
	for _, x := range experiments.List() {
		if !strings.Contains(md, "| `"+x.Name+"` |") {
			t.Errorf("markdown lacks a row for %s:\n%s", x.Name, md)
		}
		if x.Doc != "" && !strings.Contains(md, "]("+x.Doc+")") {
			t.Errorf("markdown lacks the %s doc link %s", x.Name, x.Doc)
		}
	}
	site := experiments.Markdown("https://github.com/devcell-sh/devcell/blob/main/")
	if !strings.Contains(site, "](https://github.com/devcell-sh/devcell/blob/main/docs/cage.md)") {
		t.Errorf("linkBase should prefix doc links:\n%s", site)
	}
}

func TestJSON_RoundTrips(t *testing.T) {
	b, err := experiments.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var got []struct{ Name, Group, Summary, Doc, Ticket, State string }
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(experiments.List()) || got[0].Group == "" || got[0].State != "active" {
		t.Errorf("JSON = %s", b)
	}
}

// The generated blocks in the repo must match the registry; regenerate with
// `task docs:experimental` when they drift.
func TestGeneratedDocsInSync(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"../../README.md", experiments.Markdown("")},
		{"../../docs/experimental.md", experiments.Markdown("../")},
	} {
		b, err := os.ReadFile(filepath.Clean(tc.path))
		if err != nil {
			t.Fatal(err)
		}
		block, ok := experiments.Block(string(b))
		if !ok {
			t.Errorf("%s: no %s/%s markers", tc.path, experiments.BeginMarker, experiments.EndMarker)
			continue
		}
		if strings.TrimSpace(block) != strings.TrimSpace(tc.want) {
			t.Errorf("%s: generated block is stale, run `task docs:experimental`", tc.path)
		}
	}
	want, _ := experiments.JSON()
	got, err := os.ReadFile("../../web/src/data/experimental.json")
	if err != nil || strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Errorf("web/src/data/experimental.json is stale (%v), run `task docs:experimental`", err)
	}
}

func TestReplaceBlock(t *testing.T) {
	doc := "a\n" + experiments.BeginMarker + "\nold\n" + experiments.EndMarker + "\nb\n"
	got, err := experiments.ReplaceBlock(doc, "new")
	if err != nil || got != "a\n"+experiments.BeginMarker+"\nnew\n"+experiments.EndMarker+"\nb\n" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := experiments.ReplaceBlock("no markers", "x"); err == nil {
		t.Error("missing markers must be an error")
	}
}
