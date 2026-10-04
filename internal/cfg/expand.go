package cfg

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// MissingEnvError reports host env vars referenced from .devcell.toml [env]
// values that are unset (or empty) on the host. Aggregates all misses so the
// user fixes them in one pass rather than one boot per typo.
type MissingEnvError struct {
	// Refs maps each missing host var name to the [env].<key> paths that
	// referenced it (the same var may be referenced from multiple [env] keys).
	Refs map[string][]string
}

func (e *MissingEnvError) Error() string {
	var b strings.Builder
	b.WriteString("missing references in .devcell.toml [env]:\n")
	names := make([]string, 0, len(e.Refs))
	for k := range e.Refs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		paths := append([]string(nil), e.Refs[name]...)
		sort.Strings(paths)
		fmt.Fprintf(&b, "  • %-24s (referenced in %s)\n", name, strings.Join(paths, ", "))
	}
	b.WriteString("For $VAR references, set them in your shell.\n")
	b.WriteString("For ${secret:NAME} references, check your [secrets.onepassword] documents.")
	return b.String()
}

// ExpandEnv resolves two kinds of references in [env] values:
//
//   - ${VAR} and $VAR: resolved against the host environment via lookup
//     (pass os.LookupEnv in production). Set-but-empty is treated as a miss.
//   - ${secret:NAME}: resolved against the secrets map (1Password field
//     labels). Only the ${secret:NAME} form is recognized (no $secret:NAME
//     shorthand).
//
// Values are mutated in place. Plain values (no `$`) pass through unchanged
// and never allocate.
//
// Returns a non-nil *MissingEnvError if any reference could not be resolved.
func ExpandEnv(env map[string]string, lookup func(string) (string, bool), secrets map[string]string) *MissingEnvError {
	refs := map[string][]string{}
	for k, v := range env {
		if !strings.ContainsRune(v, '$') {
			continue
		}
		env[k] = os.Expand(v, func(name string) string {
			if prefix, label, ok := strings.Cut(name, ":"); ok && prefix == "secret" {
				if val, found := secrets[label]; found && val != "" {
					return val
				}
				refs["secret:"+label] = append(refs["secret:"+label], "[env]."+k)
				return ""
			}
			if val, ok := lookup(name); ok && val != "" {
				return val
			}
			refs[name] = append(refs[name], "[env]."+k)
			return ""
		})
	}
	if len(refs) == 0 {
		return nil
	}
	return &MissingEnvError{Refs: refs}
}
