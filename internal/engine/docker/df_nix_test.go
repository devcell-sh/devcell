package docker_test

import (
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// `cell build df` nix section: default output shows nix-store state —
// counts, root NAMES, stale markers, per-root metadata, and closure sizes.
// Read-only: volume-mounted container, no GC/mutation, no auto/ roots.
// nix-store -qR and --query --size are read-only DB queries (unlike
// --gc/--print-dead which mutate roots, CELL-333).

func TestNixDFReportScript_IsReadOnly(t *testing.T) {
	forbidden := []string{
		"rm ", "rm\t", "ln -s", "mkdir", "mv ", "touch",
		"nix-collect-garbage", "nix-store --gc", "nix-store --delete",
		"> /", ">> /",
	}
	for _, tok := range forbidden {
		if strings.Contains(docker.NixDFReportScript, tok) {
			t.Errorf("df report script must be read-only, found %q", tok)
		}
	}
	if strings.Contains(docker.NixDFReportScript, "gcroots/auto") {
		t.Error("df report script must not evaluate auto/ roots")
	}
	// Block the `nix` CLI (nix build, nix profile, etc.) but allow
	// path references like /nix and nix-store (read-only queries).
	for _, line := range strings.Split(docker.NixDFReportScript, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"nix ", "nix\t"} {
			if strings.HasPrefix(trimmed, prefix) {
				t.Errorf("df report script must not invoke the nix CLI: %q", trimmed)
			}
		}
	}
}

func TestParseNixStoreReport_ParsesHealthRootsAndMetas(t *testing.T) {
	out := `total=3 stale=1 hashes=2 generations=4 orphaned=2 revs=2 newest_rev=9f8e7d6 newest_projects=1
root name=dikb1y8-profile stale=0
root name=dikb1y8-generation stale=0
root name=old4321-profile stale=1
meta hash=dikb1y8 cell=devcell stack=ultimate nixpkgs=9f8e7d6 size=12581603648
volume_bytes=18456743936
top bytes=1073741824 name=abc12345678901234567890123456789-gcc-14.1.0-lib
top bytes=536870912 name=def12345678901234567890123456789-python3-3.12.4
`
	r, err := docker.ParseNixStoreReport("devcell-nix-store", out)
	if err != nil {
		t.Fatal(err)
	}
	if r.Volume != "devcell-nix-store" {
		t.Errorf("volume not carried: %+v", r)
	}
	if r.Health.TotalRoots != 3 || r.Health.OrphanedGenerations != 2 {
		t.Errorf("health not parsed: %+v", r.Health)
	}
	if len(r.Roots) != 3 {
		t.Fatalf("want 3 roots, got %d: %+v", len(r.Roots), r.Roots)
	}
	if r.Roots[0].Name != "dikb1y8-profile" || r.Roots[0].Stale {
		t.Errorf("root[0] wrong: %+v", r.Roots[0])
	}
	if r.Roots[2].Name != "old4321-profile" || !r.Roots[2].Stale {
		t.Errorf("stale root not flagged: %+v", r.Roots[2])
	}
	if len(r.Metas) != 1 || r.Metas[0].Cell != "devcell" ||
		r.Metas[0].Stack != "ultimate" || r.Metas[0].Nixpkgs != "9f8e7d6" {
		t.Errorf("meta wrong: %+v", r.Metas)
	}
	if r.Metas[0].ClosureBytes != 12581603648 {
		t.Errorf("closure bytes = %d, want 12581603648", r.Metas[0].ClosureBytes)
	}
	if r.VolumeBytes != 18456743936 {
		t.Errorf("volume bytes = %d, want 18456743936", r.VolumeBytes)
	}
	if len(r.TopPaths) != 2 {
		t.Fatalf("want 2 top paths, got %d", len(r.TopPaths))
	}
	if r.TopPaths[0].Bytes != 1073741824 {
		t.Errorf("top[0] bytes = %d, want 1073741824", r.TopPaths[0].Bytes)
	}
	if r.TopPaths[0].Name != "abc12345678901234567890123456789-gcc-14.1.0-lib" {
		t.Errorf("top[0] name = %q", r.TopPaths[0].Name)
	}
}

func TestParseNixStoreReport_SizeFieldOptional(t *testing.T) {
	out := `total=1 stale=0 hashes=1 generations=1 orphaned=0
meta hash=abc123 cell=test stack=base nixpkgs=rev1
`
	r, err := docker.ParseNixStoreReport("v", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Metas) != 1 || r.Metas[0].ClosureBytes != 0 {
		t.Errorf("missing size= must be 0, got %d", r.Metas[0].ClosureBytes)
	}
}

func TestFormatNixStoreSection_ShowsCountsNamesMetaAndHints(t *testing.T) {
	r := docker.NixStoreReport{
		Volume: "devcell-nix-store",
		Health: docker.NixStoreHealth{
			TotalRoots: 3, StaleRoots: 1, ProfileHashes: 2,
			Generations: 4, OrphanedGenerations: 2,
		},
		Roots: []docker.NixRootEntry{
			{Name: "dikb1y8-profile"},
			{Name: "dikb1y8-generation"},
			{Name: "old4321-profile", Stale: true},
		},
		Metas: []docker.NixRootMeta{
			{Hash: "dikb1y8", Cell: "devcell", Stack: "ultimate", Nixpkgs: "9f8e7d6abcdef"},
		},
	}
	var b strings.Builder
	docker.FormatNixStoreSection(r, &b)
	out := b.String()

	for _, want := range []string{
		"Nix store (devcell-nix-store)",
		"3 roots", "2 profile hashes", "4 generations", "2 orphaned",
		"dikb1y8-profile", "dikb1y8-generation",
		"old4321-profile", "(stale)",
		"cell=devcell", "stack=ultimate", "nixpkgs=9f8e7d6",
		"cell build prune",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("nix section missing %q\noutput:\n%s", want, out)
		}
	}
}

// A root whose hash has a -meta file gets the metadata inline on its
// -profile row; unmetad roots render bare.
func TestFormatNixStoreSection_MetaJoinedByHash(t *testing.T) {
	r := docker.NixStoreReport{
		Volume: "v",
		Health: docker.NixStoreHealth{TotalRoots: 2, ProfileHashes: 2},
		Roots: []docker.NixRootEntry{
			{Name: "aaa1111-profile"},
			{Name: "bbb2222-profile"},
		},
		Metas: []docker.NixRootMeta{{Hash: "aaa1111", Cell: "trips"}},
	}
	var b strings.Builder
	docker.FormatNixStoreSection(r, &b)
	out := b.String()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "aaa1111-profile") && !strings.Contains(line, "cell=trips") {
			t.Errorf("meta must be joined onto its root's row: %q", line)
		}
		if strings.Contains(line, "bbb2222-profile") && strings.Contains(line, "cell=") {
			t.Errorf("unmetad root must render bare: %q", line)
		}
	}
}

// Old meta files wrote "project=" instead of "cell=". The parser must
// fall back so pre-existing volumes still display the cell name.
func TestParseNixStoreReport_BackwardCompatProjectKey(t *testing.T) {
	out := `total=1 stale=0 hashes=1 generations=1 orphaned=0
meta hash=abc12345 project=legacy-proj stack=base
`
	r, err := docker.ParseNixStoreReport("v", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Metas) != 1 {
		t.Fatalf("want 1 meta, got %d", len(r.Metas))
	}
	if r.Metas[0].Cell != "legacy-proj" {
		t.Errorf("Cell = %q, want %q (project= fallback)", r.Metas[0].Cell, "legacy-proj")
	}
}

// Probe failure → empty section, df must not break.
func TestFormatNixStoreSection_EmptyReportRendersNothing(t *testing.T) {
	var b strings.Builder
	docker.FormatNixStoreSection(docker.NixStoreReport{}, &b)
	if b.Len() != 0 {
		t.Errorf("empty report must render nothing, got %q", b.String())
	}
}
