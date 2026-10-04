package ux

import (
	"fmt"
	"io"
	"strings"
)

// deprecatedPhrase is the canonical wording every deprecation producer uses
// (cfg.DeprecatedUse.Warning, engine.Resolve, warnDeprecatedFlags).
// DeprecatedLine splits on it to render the short user-facing form.
const deprecatedPhrase = " is deprecated and will be removed in a future release: "

// Deprecated prints one deprecation row on w, aligned with the other
// status rows (same ⚠ prefix as ProgressSpinner.Warn):
//
//	⚠      <subject> is deprecated (<hint>)
//
// w is normally stderr so --format json/yaml stdout stays parseable.
func Deprecated(w io.Writer, subject, hint string) {
	fmt.Fprintf(w, " %s %s is deprecated (%s)\n", prefix(StyleWarning, "⚠"), subject, hint)
}

// DeprecatedAt is Deprecated with the source the deprecated thing came from
// (a config file name) appended:
//
//	⚠      <subject> is deprecated (<hint>) (<source>)
func DeprecatedAt(w io.Writer, subject, hint, source string) {
	fmt.Fprintf(w, " %s %s is deprecated (%s) (%s)\n", prefix(StyleWarning, "⚠"), subject, hint, source)
}

// DeprecatedLine renders a canonical deprecation message (see
// deprecatedPhrase) through Deprecated. Messages in any other shape are
// printed as-is behind the warning marker.
func DeprecatedLine(w io.Writer, msg string) {
	if subject, hint, ok := strings.Cut(msg, deprecatedPhrase); ok {
		Deprecated(w, subject, hint)
		return
	}
	fmt.Fprintf(w, " %s %s\n", prefix(StyleWarning, "⚠"), msg)
}
