// Package version holds build-time metadata injected via -ldflags.
package version

// Version is the release version, set by goreleaser or -ldflags.
var Version = "v0.0.0"

// GitCommit is the short SHA of the commit this binary was built from.
var GitCommit = "none"

// BuildDate is the ISO-8601 timestamp of the build.
var BuildDate = "unknown"

// Full returns a composite "version-date-commit" string for display.
func Full() string {
	return Version + "-" + BuildDate + "-" + GitCommit
}
