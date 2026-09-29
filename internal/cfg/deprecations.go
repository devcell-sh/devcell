package cfg

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Deprecation is a TOML key that was renamed. The old key keeps working
// (its value is merged into the replacement) but warns on every load.
type Deprecation struct {
	Path        []string // old TOML key path, e.g. {"ports", "forward"}
	Name        string   // old key as users write it, e.g. "[ports] forward"
	Replacement string   // new key, e.g. "[cell] ports"
	Message     string   // what the user should do, shown verbatim, e.g. `use [cell] ports = ["3000"] instead`
}

// Deprecations lists every deprecated TOML key. Remove an entry together
// with its struct field when the old key is dropped.
var Deprecations = []Deprecation{
	{
		Path:        []string{"volumes"},
		Name:        "[[volumes]]",
		Replacement: "[cell] volumes",
		Message:     `use [cell] volumes = ["/host:/container:ro"] instead`,
	},
	{
		Path:        []string{"ports", "forward"},
		Name:        "[ports] forward",
		Replacement: "[cell] ports",
		Message:     `use [cell] ports = ["3000", "8080:3000"] instead`,
	},
	{
		Path:        []string{"mcp", "enabled"},
		Name:        "[mcp] enabled",
		Replacement: "[cell] mcps",
		Message:     `use [cell] mcps = ["playwright"] instead`,
	},
	{
		Path:        []string{"op"},
		Name:        "[op]",
		Replacement: "[secrets.onepassword]",
		Message:     `use [secrets.onepassword] documents = ["prod-api-keys"] instead`,
	},
}

// DeprecatedUse records a deprecated key found in a specific config file.
type DeprecatedUse struct {
	Deprecation
	File string
}

// Warning is the user-facing line for this use.
func (u DeprecatedUse) Warning() string {
	return fmt.Sprintf("%s: %s is deprecated and will be removed in a future release: %s",
		u.File, u.Name, u.Message)
}

func detectDeprecations(md toml.MetaData, file string) []DeprecatedUse {
	var out []DeprecatedUse
	for _, d := range Deprecations {
		if md.IsDefined(d.Path...) {
			out = append(out, DeprecatedUse{Deprecation: d, File: file})
		}
	}
	return out
}
