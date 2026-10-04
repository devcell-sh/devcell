package cell

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// StackTag returns the canonical tag for a stack plus optional modules, used
// to name per-stack VM templates and images. Module order does not matter.
// Examples: "ultimate", "dev-linear-plex-a1b2c3d4".
func StackTag(stack string, modules []string) string {
	if len(modules) == 0 {
		return stack
	}
	sorted := make([]string, len(modules))
	copy(sorted, modules)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	sha8 := fmt.Sprintf("%x", h[:4])
	return fmt.Sprintf("%s-%s-%s", stack, strings.Join(sorted, "-"), sha8)
}
