package engine

import (
	"fmt"
	"slices"
	"strings"
)

// Name is an execution engine, how a cell is run: the value of --engine and
// [cell] engine.
type Name string

const (
	Docker Name = "docker" // Linux container
	Tart   Name = "tart"   // macOS VM on Apple Silicon
	Winkit Name = "winkit" // Windows VM via go-winkit: vz on macOS hosts, qemu on Linux hosts
)

// Guest is the OS image inside the cell. The --os and [cell] os values map
// to guests with GuestForOS.
type Guest string

const (
	Linux       Guest = "linux"
	MacOS       Guest = "macos"
	WindowsPE   Guest = "winpe"        // Windows PE + WSL1; no Windows state persists
	WindowsFull Guest = "windows-full" // full Windows + WSL1 (winkit stage full-wsl); not reachable from the CLI yet
)

// capabilities is the one place that knows which (engine, guest) pairs
// exist. Rows are in the order error messages list engines. An engine's
// first guest is its default (DefaultGuest).
var capabilities = []struct {
	engine Name
	guests []Guest
}{
	{Docker, []Guest{Linux}},
	// Linux guests on Tart are planned, not implemented.
	{Tart, []Guest{MacOS}},
	{Winkit, []Guest{WindowsPE, WindowsFull}},
}

// defaults is the engine each guest runs on when only the guest is chosen.
var defaults = []struct {
	guest  Guest
	engine Name
}{
	{Linux, Docker},
	{MacOS, Tart},
	{WindowsPE, Winkit},
	{WindowsFull, Winkit},
}

// retired maps a removed engine to what the user should run instead.
var retired = map[Name]string{
	"libvirt": "the libvirt engine was retired; run `cell --engine winkit` on the macOS host instead",
	"vagrant": `the vagrant engine was retired; use --os macos or [cell] os = "macos" for a Tart VM instead`,
}

// aliases maps a deprecated engine name to the engine it now means.
// Resolve follows them and returns a deprecation warning.
var aliases = map[Name]Name{
	"qemu": Winkit,
}

// osValues lists the values GuestForOS accepts, for error messages.
const osValues = "linux, macos, windows, winpe"

// GuestForOS maps an --os or [cell] os value to its guest, and reports
// whether the value is known.
//
// The user has not picked the final CLI names for the two Windows guests
// yet. Until then "windows" keeps meaning the PE guest (the behavior before
// the rename), "winpe" is its explicit spelling, and WindowsFull has no CLI
// value. Keep osValues in step with this switch.
func GuestForOS(v string) (Guest, bool) {
	switch v {
	case "linux":
		return Linux, true
	case "macos":
		return MacOS, true
	case "windows", "winpe":
		return WindowsPE, true
	}
	return "", false
}

// Valid reports whether n is an engine cell can dispatch to.
func Valid(n Name) bool {
	_, ok := guestsOf(n)
	return ok
}

// Retired returns what to run instead of the removed engine n, and whether
// n is one.
func Retired(n Name) (hint string, ok bool) {
	hint, ok = retired[n]
	return hint, ok
}

// Supports reports whether engine n can run guest g.
func Supports(n Name, g Guest) bool {
	guests, _ := guestsOf(n)
	return slices.Contains(guests, g)
}

// DefaultFor returns the engine guest g runs on when no engine is chosen,
// and whether g is a known guest.
func DefaultFor(g Guest) (Name, bool) {
	for _, d := range defaults {
		if d.guest == g {
			return d.engine, true
		}
	}
	return "", false
}

// DefaultGuest returns the guest engine n runs when no OS is chosen, and
// whether n is a known engine.
func DefaultGuest(n Name) (Guest, bool) {
	guests, ok := guestsOf(n)
	if !ok || len(guests) == 0 {
		return "", false
	}
	return guests[0], true
}

// GuestFor returns the guest engine n runs: the guest of the first value in
// osValues (--os, then [cell] os) that maps to a guest n supports, else n's
// default guest. Empty, unknown and unsupported values are skipped, so a
// stale [cell] os under an explicit --engine does not fail the run. It
// returns "" for an unknown engine.
func GuestFor(n Name, osValues ...string) Guest {
	for _, v := range osValues {
		if g, ok := GuestForOS(v); ok && Supports(n, g) {
			return g
		}
	}
	g, _ := DefaultGuest(n)
	return g
}

// Resolve returns the engine from the first non-empty source:
//
//  1. CLI --engine flag
//  2. CLI --os flag (mapped via GuestForOS and DefaultFor)
//  3. TOML [cell].engine
//  4. TOML [cell].os (mapped the same way)
//  5. Docker (default)
//
// The winning --engine or [cell] engine value must be Valid or a deprecated
// alias. When both --engine and --os are set, the pair must be in the
// capability table: incompatible pairs (e.g. --os windows --engine tart)
// return an error.
//
// deprecation is a warning for a deprecated engine name the result came
// from (e.g. "qemu"), for the caller to print once; it is empty otherwise.
func Resolve(flagEngine, flagOS, tomlEngine, tomlOS string) (n Name, deprecation string, err error) {
	if flagEngine != "" {
		n, deprecation, err = parse(flagEngine, "--engine")
		if err != nil {
			return "", "", err
		}
	}

	if flagOS != "" {
		g, ok := GuestForOS(flagOS)
		if !ok {
			return "", "", fmt.Errorf("unsupported --os value %q (valid: %s)", flagOS, osValues)
		}
		if flagEngine == "" {
			n, _ = DefaultFor(g)
			return n, "", nil
		}
		if !Supports(n, g) {
			return "", "", fmt.Errorf("--os %s and --engine %s are incompatible", flagOS, flagEngine)
		}
	}

	if flagEngine != "" {
		return n, deprecation, nil
	}

	if tomlEngine != "" {
		return parse(tomlEngine, "[cell] engine")
	}

	if tomlOS != "" {
		if g, ok := GuestForOS(tomlOS); ok {
			n, _ = DefaultFor(g)
			return n, "", nil
		}
	}

	return Docker, "", nil
}

// parse returns the engine an --engine or [cell] engine value names,
// following deprecated aliases, and rejects values no dispatch branch
// handles. source names where the value came from.
func parse(v, source string) (Name, string, error) {
	n := Name(v)
	if to, ok := aliases[n]; ok {
		return to, fmt.Sprintf("engine %q is deprecated and will be removed in a future release: use engine = %q instead", n, to), nil
	}
	if Valid(n) {
		return n, "", nil
	}
	if hint, ok := Retired(n); ok {
		return "", "", fmt.Errorf("%s %q: %s", source, n, hint)
	}
	return "", "", fmt.Errorf("unsupported %s value %q (valid: %s)", source, n, engineList())
}

func guestsOf(n Name) ([]Guest, bool) {
	for _, c := range capabilities {
		if c.engine == n {
			return c.guests, true
		}
	}
	return nil, false
}

func engineList() string {
	names := make([]string, len(capabilities))
	for i, c := range capabilities {
		names[i] = string(c.engine)
	}
	return strings.Join(names, ", ")
}
