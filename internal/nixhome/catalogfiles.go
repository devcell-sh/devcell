package nixhome

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// CatalogPath is the site path under which the home flake's catalog is
// published for one schema version:
// https://devcell.sh/schema/<version>/devcell-sh/home/{modules,stacks,packages}.json.
// The org/repo segment mirrors the flake's GitHub location so other homes
// can publish alongside it. Mirrored on disk under web/public/.
func CatalogPath(version string) string {
	return "schema/" + version + "/devcell-sh/home"
}

// fullCatalog is the shape of `nix eval <flake>#devcellCatalog --json`.
type fullCatalog struct {
	Modules  Catalog             `json:"modules"`
	Profiles map[string][]string `json:"profiles"`
	Stacks   map[string]struct {
		Modules  []string `json:"modules"`
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	} `json:"stacks"`
}

// hmInternal lists home-manager's own derivations that appear in
// home.packages but are not tools a user could add or already rely on.
var hmInternal = map[string]bool{
	"hm-session-vars.sh":                   true,
	"home-configuration-reference-manpage": true,
}

func isHMInternal(name string) bool {
	return hmInternal[name] || strings.HasPrefix(name, "dummy-xdg-")
}

// SplitCatalog turns the raw devcellCatalog JSON into the three published
// documents, keyed by file name:
//
//   - modules.json:  module name → {description, mcpServers, sizeMb}
//   - stacks.json:   stack name → {modules: [names], packages: [names]}
//   - packages.json: package name → {version, stacks: [names that ship it]}
//
// packages.json is the inverted index: it answers "is X already in my stack"
// without scanning every stack entry. Home-manager's internal derivations
// are dropped from both package views.
func SplitCatalog(raw []byte) (map[string][]byte, error) {
	var c fullCatalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse devcellCatalog: %w", err)
	}

	type stackOut struct {
		Modules  []string `json:"modules"`
		Packages []string `json:"packages"`
	}
	type pkgOut struct {
		Version string   `json:"version"`
		Stacks  []string `json:"stacks"`
	}
	stacks := make(map[string]stackOut, len(c.Stacks))
	packages := map[string]*pkgOut{}

	stackNames := make([]string, 0, len(c.Stacks))
	for name := range c.Stacks {
		stackNames = append(stackNames, name)
	}
	sort.Strings(stackNames)

	for _, name := range stackNames {
		s := c.Stacks[name]
		out := stackOut{Modules: append([]string{}, s.Modules...), Packages: []string{}}
		sort.Strings(out.Modules)
		seen := map[string]bool{}
		for _, p := range s.Packages {
			if isHMInternal(p.Name) || seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			out.Packages = append(out.Packages, p.Name)
			entry := packages[p.Name]
			if entry == nil {
				entry = &pkgOut{Version: p.Version}
				packages[p.Name] = entry
			}
			entry.Stacks = append(entry.Stacks, name)
		}
		sort.Strings(out.Packages)
		stacks[name] = out
	}

	files := map[string][]byte{}
	for name, v := range map[string]any{
		"modules.json":  c.Modules,
		"stacks.json":   stacks,
		"packages.json": packages,
	} {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		files[name] = append(b, '\n')
	}
	return files, nil
}

// ReadFullCatalogFromFlake invokes `nix eval --json <flakeRef>#devcellCatalog`
// and returns the raw JSON for SplitCatalog.
func ReadFullCatalogFromFlake(ctx context.Context, flakeRef string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nix",
		"eval", "--json",
		"--extra-experimental-features", "nix-command flakes",
		flakeRef+"#devcellCatalog",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nix eval %s#devcellCatalog: %w\n%s", flakeRef, err, ee.Stderr)
		}
		return nil, fmt.Errorf("nix eval %s#devcellCatalog: %w", flakeRef, err)
	}
	return out, nil
}
