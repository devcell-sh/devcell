package docker

import "strings"

// ParseImageModules extracts module names from a devcell.packages label value
// like "desktop: 69, base: 35, wireguard: 2". Returns a set of lowercase
// module names.
func ParseImageModules(packagesLabel string) map[string]bool {
	out := make(map[string]bool)
	if packagesLabel == "" {
		return out
	}
	for _, part := range strings.Split(packagesLabel, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, ":"); idx > 0 {
			out[strings.TrimSpace(part[:idx])] = true
		}
	}
	return out
}

// MissingImpliedModules returns the names from implied that are not present
// in the image's baked module set. Empty when everything is covered.
func MissingImpliedModules(packagesLabel string, implied []string) []string {
	if len(implied) == 0 {
		return nil
	}
	have := ParseImageModules(packagesLabel)
	var missing []string
	for _, m := range implied {
		if !have[m] {
			missing = append(missing, m)
		}
	}
	return missing
}
