package cfg

import "sort"

// ImpliedModules returns nix module names that are required by other config
// sections but not explicitly listed in [cell] modules. For example, an
// enabled [[wireguard]] entry needs the "wireguard" nix module for wg-quick
// and the s6 boot service.
func ImpliedModules(c CellConfig) []string {
	var out []string
	if WireguardEnabled(c) {
		out = append(out, "wireguard")
	}
	return out
}

// ApplyImpliedModules appends any implied modules that are not already present
// in c.Cell.Modules, then sorts and deduplicates the list. Call this after
// config merge so all downstream consumers (ConfigHash, flake generation,
// image label comparison) see the full module set.
func ApplyImpliedModules(c *CellConfig) {
	implied := ImpliedModules(*c)
	if len(implied) == 0 {
		return
	}
	have := make(map[string]bool, len(c.Cell.Modules))
	for _, m := range c.Cell.Modules {
		have[m] = true
	}
	for _, m := range implied {
		if !have[m] {
			c.Cell.Modules = append(c.Cell.Modules, m)
		}
	}
	sort.Strings(c.Cell.Modules)
}
