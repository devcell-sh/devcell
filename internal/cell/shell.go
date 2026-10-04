package cell

import "strings"

// ShellQuote returns s as exactly one POSIX shell word. Words made only of
// letters, digits and _@%+=:,./- are returned unchanged for readability.
// Everything else is wrapped in single quotes; an embedded single quote
// closes the quoting, is emitted backslash-escaped, and reopens it. The empty
// string becomes an empty pair of single quotes.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if isShellSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellJoin quotes each word with ShellQuote and joins them with spaces,
// producing a string a POSIX shell (e.g. `bash -c`) splits back into words.
func ShellJoin(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = ShellQuote(w)
	}
	return strings.Join(quoted, " ")
}

func isShellSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("_@%+=:,./-", c) >= 0:
		default:
			return false
		}
	}
	return true
}
