package docker_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

// CELL-390: startup nix-store health check. Read-only by construction —
// the probe reports, only `cell build prune` / `cell cleanup` act.

// The probe must be pure filesystem inspection. nix must never be invoked
// (`nix-store --gc --print-dead` mutates: its root-finding pass deletes
// indirect roots — measured live, CELL-333). Nothing may be created,
// deleted, or retargeted.
func TestNixHealthProbeScript_ContainsNoMutatingTokens(t *testing.T) {
	forbidden := []string{
		"rm ", "rm\t", "ln -s", "mkdir", "mv ", "touch",
		"nix-store", "nix-collect-garbage", "nix ",
		"> /", ">> /",
	}
	for _, tok := range forbidden {
		if strings.Contains(docker.NixHealthProbeScript, tok) {
			t.Errorf("probe script must be read-only, found %q:\n%s", tok, docker.NixHealthProbeScript)
		}
	}
}

// auto/ roots are namespace-local (CELL-330) — the probe must not even
// evaluate them, from any namespace their targets are unanswerable.
func TestNixHealthProbeScript_IgnoresAutoRoots(t *testing.T) {
	if strings.Contains(docker.NixHealthProbeScript, "gcroots/auto") {
		t.Error("probe must not evaluate auto/ roots — targets are container-private")
	}
	if !strings.Contains(docker.NixHealthProbeScript, "gcroots/devcell") {
		t.Error("probe must inspect gcroots/devcell/ (volume-resident targets)")
	}
}

// The probe runs in the nixos/nix image whose sh has NO sed, and whose ls
// follows symlinks (a root symlink to a store dir lists the dir contents).
// Verified live 2026-08-01: `sed: command not found`, hashes silently 0.
// Every container-side script must stick to basename/cut/grep/readlink.
func TestVolumeScripts_NoSedNoLs(t *testing.T) {
	scripts := map[string]string{
		"NixHealthProbeScript":  docker.NixHealthProbeScript,
		"NixDFReportScript":     docker.NixDFReportScript,
		"SafeNixGCScript":       docker.SafeNixGCScript,
		"NixGCRootReportScript": docker.NixGCRootReportScript,
	}
	for name, s := range scripts {
		if strings.Contains(s, "sed ") || strings.Contains(s, "| sed") {
			t.Errorf("%s uses sed — not present in the nixos/nix probe image", name)
		}
		// ls as a command ($(ls …), piped, or line-start) — plain-word
		// matches like "cells" are fine.
		if strings.Contains(s, "$(ls ") || strings.Contains(s, "| ls ") ||
			strings.Contains(s, "\nls ") || strings.HasPrefix(s, "ls ") {
			t.Errorf("%s uses ls — it follows root symlinks into store dirs", name)
		}
	}
}

func TestParseNixStoreHealth_ParsesProbeOutput(t *testing.T) {
	h, err := docker.ParseNixStoreHealth("total=8 stale=3 hashes=2 generations=5 orphaned=4\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.TotalRoots != 8 || h.StaleRoots != 3 || h.ProfileHashes != 2 ||
		h.Generations != 5 || h.OrphanedGenerations != 4 {
		t.Errorf("wrong parse: %+v", h)
	}
}

// Docker may prepend pull noise when the probe image isn't local — the
// parser must find the datapoint line anywhere in the output.
func TestParseNixStoreHealth_SkipsLeadingNoise(t *testing.T) {
	out := "Unable to find image locally\nlatest: Pulling...\ntotal=1 stale=0 hashes=1 generations=1 orphaned=0\n"
	h, err := docker.ParseNixStoreHealth(out)
	if err != nil {
		t.Fatal(err)
	}
	if h.TotalRoots != 1 || h.ProfileHashes != 1 {
		t.Errorf("wrong parse: %+v", h)
	}
}

func TestParseNixStoreHealth_NoDatapointLineIsError(t *testing.T) {
	if _, err := docker.ParseNixStoreHealth("garbage\n"); err == nil {
		t.Error("output without a datapoint line must error (probe degraded)")
	}
}

// CELL-391: the probe (still read-only) must also report lock-drift
// datapoints from the *-meta files CELL-332 stamps: how many distinct
// nixpkgs revs are live, which is newest, and how many projects sit on it.
func TestNixHealthProbeScript_ReadsMetaRevs(t *testing.T) {
	for _, want := range []string{"*-meta", "nixpkgs="} {
		if !strings.Contains(docker.NixHealthProbeScript, want) {
			t.Errorf("probe must scan -meta files for nixpkgs revs, missing %q", want)
		}
	}
}

func TestParseNixStoreHealth_ParsesRevDatapoints(t *testing.T) {
	h, err := docker.ParseNixStoreHealth(
		"total=4 stale=0 hashes=2 generations=3 orphaned=0 revs=2 newest_rev=9f8e7d6abc newest_projects=3\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.DistinctRevs != 2 || h.NewestRev != "9f8e7d6abc" || h.NewestProjects != 3 {
		t.Errorf("rev datapoints not parsed: %+v", h)
	}
}

// Pre-CELL-332 volumes have no -meta files; the old datapoint line (no rev
// fields) must still parse: missing keys are zero values, not errors.
func TestParseNixStoreHealth_RevFieldsOptional(t *testing.T) {
	h, err := docker.ParseNixStoreHealth("total=1 stale=0 hashes=1 generations=1 orphaned=0\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.DistinctRevs != 0 || h.NewestRev != "" {
		t.Errorf("missing rev fields must be zero values: %+v", h)
	}
}

// Summary returns (tag, detail, warn) for the "Nix store" phase row.
func TestNixStoreHealth_SummaryClean(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 4, ProfileHashes: 1, Generations: 2}
	tag, detail, warn := h.Summary()
	if warn {
		t.Error("clean store must not be a warning")
	}
	if tag != "clean" {
		t.Errorf("clean tag = %q, want %q", tag, "clean")
	}
	for _, want := range []string{"4 roots", "1 profile hash"} {
		if !strings.Contains(detail, want) {
			t.Errorf("clean detail missing %q, got %q", want, detail)
		}
	}
}

func TestNixStoreHealth_SummaryFindingsIncludePruneHint(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 6, StaleRoots: 2, ProfileHashes: 3, Generations: 8, OrphanedGenerations: 5}
	tag, detail, warn := h.Summary()
	if !warn {
		t.Error("findings must be a warning")
	}
	if tag != "drifted" {
		t.Errorf("tag = %q, want %q (drift takes precedence)", tag, "drifted")
	}
	for _, want := range []string{"2 stale root", "5 orphaned generation", "cell build prune --pure"} {
		if !strings.Contains(detail, want) {
			t.Errorf("findings detail missing %q, got %q", want, detail)
		}
	}
}

func TestNixStoreHealth_SummaryReportsDrift(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 4, ProfileHashes: 3, Generations: 2}
	tag, detail, warn := h.Summary()
	if !warn {
		t.Error("drift must be a warning")
	}
	if tag != "drifted" {
		t.Errorf("tag = %q, want %q", tag, "drifted")
	}
	if !strings.Contains(detail, "3 profile hashes") {
		t.Errorf("drift (multiple hashes) must be visible in detail, got %q", detail)
	}
	if !strings.Contains(detail, "cell build prune --pure") {
		t.Errorf("drift-only detail must include prune hint, got %q", detail)
	}
}

func TestNixStoreHealth_SummaryStaleOnly(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 4, StaleRoots: 2, ProfileHashes: 1, Generations: 3}
	tag, _, warn := h.Summary()
	if !warn {
		t.Error("stale roots must be a warning")
	}
	if tag != "stale" {
		t.Errorf("tag = %q, want %q", tag, "stale")
	}
}

func TestNixStoreHealth_SummaryOrphanedOnly(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 4, ProfileHashes: 1, Generations: 5, OrphanedGenerations: 3}
	tag, _, warn := h.Summary()
	if !warn {
		t.Error("orphaned generations must be a warning")
	}
	if tag != "orphaned" {
		t.Errorf("tag = %q, want %q", tag, "orphaned")
	}
}

func TestNixStoreHealth_SummaryStaleAndOrphaned(t *testing.T) {
	h := docker.NixStoreHealth{TotalRoots: 6, StaleRoots: 2, ProfileHashes: 1, Generations: 5, OrphanedGenerations: 3}
	tag, _, warn := h.Summary()
	if !warn {
		t.Error("stale+orphaned must be a warning")
	}
	if tag != "unhealthy" {
		t.Errorf("tag = %q, want %q (mixed findings without drift)", tag, "unhealthy")
	}
}

func TestNixHealthProbeArgv_RunsInContainerOnNixVolume(t *testing.T) {
	argv := docker.NixHealthProbeArgv("devcell-nix-store")
	joined := strings.Join(argv, " ")
	for _, want := range []string{"docker", "run", "--rm", "devcell-nix-store:/nix"} {
		if !strings.Contains(joined, want) {
			t.Errorf("probe argv missing %q: %v", want, argv)
		}
	}
}

// The --debug rendering of a docker-run argv must not dump embedded shell
// scripts to the console — a multi-line `sh -c` payload made the health
// check look like it printed the script instead of running it.
func TestDebugArgv_ElidesMultilineScripts(t *testing.T) {
	argv := docker.NixHealthProbeArgv("devcell-nix-store")
	got := docker.DebugArgv(argv)
	if strings.Contains(got, "\n") {
		t.Errorf("DebugArgv must be a single line, got:\n%s", got)
	}
	if strings.Contains(got, "gcroots") {
		t.Errorf("DebugArgv must not include the script body, got:\n%s", got)
	}
	for _, want := range []string{"docker run --rm", "devcell-nix-store:/nix", "sh -c"} {
		if !strings.Contains(got, want) {
			t.Errorf("DebugArgv missing %q, got: %s", want, got)
		}
	}
	if !strings.Contains(got, "script:") {
		t.Errorf("DebugArgv should mark the elided script, got: %s", got)
	}
}

// Single-line argvs pass through untouched.
func TestDebugArgv_KeepsSingleLineArgs(t *testing.T) {
	got := docker.DebugArgv([]string{"docker", "volume", "inspect", "devcell-nix-store"})
	if got != "docker volume inspect devcell-nix-store" {
		t.Errorf("unexpected rendering: %s", got)
	}
}
