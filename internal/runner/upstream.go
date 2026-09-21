package runner

import (
	"fmt"
	"os"
	"strings"
)

// Canonical upstream nixhome source — single source of truth across the CLI.
// Previously these constants were re-encoded in 4 separate fmt.Sprintf calls
// (pure_nixhome_resolver, cmd/modules, scaffold templates). Centralised here
// so a fork/rename is a one-line change.
const (
	UpstreamOwner = "devcell-sh"
	UpstreamRepo  = "home"
	// UpstreamSubdir is empty since the nixhome moved to its own repo
	// (devcell-sh/home); the flake now lives at the repo root.
	UpstreamSubdir = ""
)

// UpstreamFlakeRef returns the canonical github flake reference for the
// devcell nixhome, pinned to `ref`. Empty / "v0.0.0" / dev-version coerces
// to DefaultNixhomeGitRef so dev builds always point at a real branch.
//
// Example: UpstreamFlakeRef("v1.0.0") → "github:devcell-sh/home/v1.0.0"
func UpstreamFlakeRef(ref string) string {
	if ref == "" || ref == "v0.0.0" || isDevVersion(ref) {
		ref = DefaultNixhomeGitRef
	}
	s := fmt.Sprintf("github:%s/%s/%s", UpstreamOwner, UpstreamRepo, ref)
	if UpstreamSubdir != "" {
		s += "?dir=" + UpstreamSubdir
	}
	return s
}

// ResolveNixhomeRef returns the nixhome source to use for builds.
// Precedence: DEVCELL_NIXHOME > DEVCELL_NIXHOME_PATH (legacy) > default upstream flake ref.
// Accepts local paths, github: flake refs, or https:// git URLs.
func ResolveNixhomeRef(ver string) string {
	if v := os.Getenv("DEVCELL_NIXHOME"); v != "" {
		return v
	}
	if v := os.Getenv("DEVCELL_NIXHOME_PATH"); v != "" {
		return v
	}
	return UpstreamFlakeRef(ver)
}

// isDevVersion returns true for git-describe versions that don't correspond
// to a real remote tag/branch: the describe form ("v0.8.2-94-g0ac6be1-dirty")
// and the bare short/full SHA that `git describe --tags --always` degrades to
// in a shallow, tagless checkout (CI runners). A devcell commit SHA can never
// exist as a ref in the nixhome repo, so treating it as one breaks the build.
// Trade-off: an all-hex branch/tag name of 7–40 chars is swallowed too — pin
// those via DEVCELL_NIXHOME instead.
func isDevVersion(v string) bool {
	return strings.Contains(v, "-g") || strings.Contains(v, "-dirty") || isHexSHA(v)
}

// isHexSHA reports whether v looks like a bare git object name — 7 to 40
// lowercase hex digits and nothing else.
func isHexSHA(v string) bool {
	if len(v) < 7 || len(v) > 40 {
		return false
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// UpstreamFlakeRefNoVersion returns the unpinned ref — used by introspection
// commands (`cell modules list`) that want the catalog as it exists upstream
// right now, not pinned to the CLI binary's compile-time version.
func UpstreamFlakeRefNoVersion() string {
	s := fmt.Sprintf("github:%s/%s", UpstreamOwner, UpstreamRepo)
	if UpstreamSubdir != "" {
		s += "?dir=" + UpstreamSubdir
	}
	return s
}
