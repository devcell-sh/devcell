package cfg

import (
	"fmt"
	"regexp"
	"strings"
)

// validNixAttr matches valid nix attribute paths: letters, digits, hyphens,
// underscores, dots (for nested attrs like python3Packages.requests).
var validNixAttr = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// nixChannels lists the channels oldest first, with the key users write.
// Stable is read from [cell] packages (the deprecated [packages.nix] stable
// is merged into it).
func nixChannels(np NixPackages) []struct {
	name, key string
	pkgs      []string
} {
	return []struct {
		name, key string
		pkgs      []string
	}{
		{"stable", "[cell] packages", np.Stable},
		{"unstable", "[packages.nix] unstable", np.Unstable},
		{"edge", "[packages.nix] edge", np.Edge},
	}
}

// ValidateNixPackageNames checks that every package name in every channel is
// a syntactically valid nix attribute name. Returns nil if all names are valid.
func ValidateNixPackageNames(np NixPackages) error {
	for _, ch := range nixChannels(np) {
		for _, pkg := range ch.pkgs {
			if pkg == "" || !validNixAttr.MatchString(pkg) {
				return fmt.Errorf("invalid package name %q in %s: must match %s", pkg, ch.key, validNixAttr.String())
			}
		}
	}
	return nil
}

// ValidateNixPackages runs all nix package validations.
func ValidateNixPackages(np NixPackages) error {
	return ValidateNixPackageNames(np)
}

// ResolveNixChannels drops a package from older channels when a newer one
// also lists it: listing a package in unstable or edge is a request for the
// newer version. overrides describes each such case, e.g.
// "uv from unstable (overrides stable)".
func ResolveNixChannels(np NixPackages) (resolved NixPackages, overrides []string) {
	chans := nixChannels(np)
	newest := map[string]int{}
	for i, ch := range chans {
		for _, pkg := range ch.pkgs {
			newest[pkg] = i
		}
	}
	out := make([][]string, len(chans))
	var order []string
	dropped := map[string][]string{}
	for i, ch := range chans {
		for _, pkg := range ch.pkgs {
			if newest[pkg] == i {
				out[i] = append(out[i], pkg)
				continue
			}
			if len(dropped[pkg]) == 0 {
				order = append(order, pkg)
			}
			dropped[pkg] = append(dropped[pkg], ch.name)
		}
	}
	for _, pkg := range order {
		overrides = append(overrides, fmt.Sprintf("%s from %s (overrides %s)", pkg, chans[newest[pkg]].name, strings.Join(dropped[pkg], ", ")))
	}
	return NixPackages{Stable: out[0], Unstable: out[1], Edge: out[2]}, overrides
}
