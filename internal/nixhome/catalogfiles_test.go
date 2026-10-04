package nixhome

import (
	"encoding/json"
	"testing"
)

// The home flake's devcellCatalog output is split into the three documents
// published at https://devcell.sh/schema/<version>/devcell-sh/home/{modules,stacks,packages}.json
// by `task schema:catalog` (cmd/catalogen.go). Paths are versioned; see CatalogPath.

const sampleCatalog = `{
  "modules": {
    "go": {"description": "Go toolchain", "mcpServers": [], "sizeMb": 350},
    "infra": {"description": "IaC", "mcpServers": ["aws-api"], "sizeMb": 1200}
  },
  "profiles": {"base": [], "dev": ["infra"]},
  "stacks": {
    "base": {"modules": [], "packages": [
      {"name": "git", "version": "2.51.2"},
      {"name": "home-manager", "version": ""},
      {"name": "dummy-xdg-mime-dirs1", "version": ""},
      {"name": "hm-session-vars.sh", "version": ""},
      {"name": "home-configuration-reference-manpage", "version": ""}
    ]},
    "go": {"modules": ["go", "infra"], "packages": [
      {"name": "git", "version": "2.51.2"},
      {"name": "gopls", "version": "0.20.0"}
    ]}
  }
}`

func splitSample(t *testing.T) (modules, stacks, packages map[string]any) {
	t.Helper()
	files, err := SplitCatalog([]byte(sampleCatalog))
	if err != nil {
		t.Fatalf("SplitCatalog: %v", err)
	}
	decode := func(name string) map[string]any {
		raw, ok := files[name]
		if !ok {
			t.Fatalf("missing %s in %v", name, files)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: invalid JSON: %v\n%s", name, err, raw)
		}
		return m
	}
	return decode("modules.json"), decode("stacks.json"), decode("packages.json")
}

func TestSplitCatalog_EmitsExactlyThreeFiles(t *testing.T) {
	files, err := SplitCatalog([]byte(sampleCatalog))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Errorf("want modules/stacks/packages, got %d files", len(files))
	}
}

func TestSplitCatalog_ModulesKeepMetadata(t *testing.T) {
	modules, _, _ := splitSample(t)
	infra := modules["infra"].(map[string]any)
	if infra["description"] != "IaC" || infra["sizeMb"].(float64) != 1200 {
		t.Errorf("module metadata not preserved: %v", infra)
	}
	if _, ok := modules["profiles"]; ok {
		t.Errorf("profiles must not leak into modules.json")
	}
}

func TestSplitCatalog_StacksListModulesAndPackageNames(t *testing.T) {
	_, stacks, _ := splitSample(t)
	goStack := stacks["go"].(map[string]any)
	if mods, _ := goStack["modules"].([]any); len(mods) != 2 || mods[0] != "go" {
		t.Errorf("go stack modules = %v", goStack["modules"])
	}
	pkgs, _ := goStack["packages"].([]any)
	if len(pkgs) != 2 || pkgs[0] != "git" || pkgs[1] != "gopls" {
		t.Errorf("stack packages should be bare names, got %v", pkgs)
	}
}

func TestSplitCatalog_PackagesInvertedIndexDropsHomeManagerNoise(t *testing.T) {
	_, _, packages := splitSample(t)
	git := packages["git"].(map[string]any)
	if git["version"] != "2.51.2" {
		t.Errorf("git version = %v", git["version"])
	}
	if stacks, _ := git["stacks"].([]any); len(stacks) != 2 || stacks[0] != "base" || stacks[1] != "go" {
		t.Errorf("git stacks = %v, want [base go]", git["stacks"])
	}
	if _, ok := packages["home-manager"]; !ok {
		t.Errorf("home-manager is a real package and must stay")
	}
	for _, noise := range []string{"dummy-xdg-mime-dirs1", "hm-session-vars.sh", "home-configuration-reference-manpage"} {
		if _, ok := packages[noise]; ok {
			t.Errorf("home-manager internal %q must be filtered", noise)
		}
	}
}

func TestSplitCatalog_RejectsMalformedInput(t *testing.T) {
	if _, err := SplitCatalog([]byte(`{"modules": 1}`)); err == nil {
		t.Error("expected error for malformed catalog")
	}
}

func TestCatalogPath_IsVersioned(t *testing.T) {
	if got := CatalogPath("v1.2.3"); got != "schema/v1.2.3/devcell-sh/home" {
		t.Errorf("CatalogPath = %q", got)
	}
}
