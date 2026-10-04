package nixhome

import (
	"reflect"
	"testing"
)

const hmDecl = "/nix/store/hm-source/modules/home-environment.nix"

func TestGroupPackageDefinitions_ByModuleDirectory(t *testing.T) {
	defs := []PackageDefinition{
		{File: "/nix/store/abc-source/nixhome/modules/desktop", Packages: []string{"xrdp", "icewm"}},
		{File: "/nix/store/abc-source/nixhome/modules/base.nix", Packages: []string{"jq", "tmux", "wget"}},
		{File: "/nix/store/abc-source/nixhome/modules/llm/claude.nix", Packages: []string{"claude-code"}},
		{File: "/nix/store/abc-source/nixhome/modules/llm/codex.nix", Packages: []string{"codex"}},
		{File: "/nix/store/hm-source/modules/programs/git.nix", Packages: []string{"git", "git-lfs"}},
		{File: "<unknown-file>", Packages: []string{"cowsay"}},
	}
	got := GroupPackageDefinitions(defs, hmDecl)
	want := []ModuleCount{
		{Module: "base", Count: 3},
		{Module: "desktop", Count: 2},
		{Module: "llm", Count: 2},
		{Module: "config", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGroupPackageDefinitions_Empty(t *testing.T) {
	if got := GroupPackageDefinitions(nil, hmDecl); len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}

func TestFormatPackageSummary(t *testing.T) {
	got := FormatPackageSummary([]ModuleCount{{"desktop", 69}, {"base", 35}, {"infra", 11}})
	want := "Packages: desktop: 69, base: 35, infra: 11"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := FormatPackageSummary(nil); got != "Packages: none" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestParsePackageEval(t *testing.T) {
	raw := []byte(`{"hm":"/nix/store/hm-source/modules/home-environment.nix","defs":[{"file":"/nix/store/abc-source/nixhome/modules/go.nix","packages":["gopls","gotools"]}]}`)
	defs, hm, err := ParsePackageEval(raw)
	if err != nil {
		t.Fatal(err)
	}
	if hm != hmDecl || len(defs) != 1 || defs[0].Packages[1] != "gotools" {
		t.Fatalf("got defs=%+v hm=%q", defs, hm)
	}
}

func TestEvalPackageDefinitions_SkipsWithoutNix(t *testing.T) {
	defs, hm, err := evalPackageDefinitionsWithLookPath(t.Context(), "path:/nowhere", "devcell-local", func(string) (string, error) {
		return "", errNotFound
	})
	if err != nil || defs != nil || hm != "" {
		t.Fatalf("got defs=%v hm=%q err=%v", defs, hm, err)
	}
}

func TestFormatModuleCounts(t *testing.T) {
	if got := FormatModuleCounts([]ModuleCount{{"desktop", 69}, {"base", 35}}); got != "desktop: 69, base: 35" {
		t.Fatalf("got %q", got)
	}
	if got := FormatModuleCounts(nil); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}
