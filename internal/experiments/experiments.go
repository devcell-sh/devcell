package experiments

import "strings"

// Where an experiment can be switched on.
const (
	Flag = "--experimental"       // comma-separated names
	Env  = "DEVCELL_EXPERIMENTAL" // comma-separated names
	Key  = "[cell] experimental"  // TOML list or comma-separated string
)

// The registered experiments. Add a new one here and in all, then run
// `task docs:experimental`.
var (
	// Cage wraps the run in a host tmux session with a one-line bottom bar.
	Cage = Experiment{
		Name:    "ui-cage",
		Summary: "one-line bottom bar (workspace, cell name, clocks) drawn by a host tmux session around the agent",
		Doc:     "docs/cage.md",
		Ticket:  "CELL-655",
	}
	// BootPopup renders the boot panel in a tmux popup that closes before
	// the agent starts. Needs Cage.
	BootPopup = Experiment{
		Name:    "ui-boot-popup",
		Summary: "boot panel in a centred tmux popup, closed before the agent starts (needs ui-cage)",
		Doc:     "docs/cage.md",
		Ticket:  "CELL-657",
	}
	// Windows is the Windows guest: --os windows or winpe, engine winkit.
	Windows = Experiment{
		Name:    "engine-windows",
		Summary: "Windows guest (--os windows or winpe): a Windows PE + WSL VM run by the winkit engine on vz or QEMU",
		Doc:     "README.md#engines-and-guests",
	}
)

// all is the iteration order for List, docs and error messages.
var all = []Experiment{Cage, BootPopup, Windows}

// List returns every registered experiment, active or not.
func List() []Experiment { return append([]Experiment(nil), all...) }

// Lookup finds the experiment named name.
func Lookup(name string) (Experiment, bool) {
	for _, x := range all {
		if x.Name == name {
			return x, true
		}
	}
	return Experiment{}, false
}

// Names lists the names of the experiments that can be enabled.
func Names() []string {
	var names []string
	for _, x := range all {
		if x.State == Active {
			names = append(names, x.Name)
		}
	}
	return names
}

// Set is the resolved set of enabled experiments.
type Set map[string]bool

// Enabled reports whether x is on.
func (s Set) Enabled(x Experiment) bool { return s[x.Name] }

// Resolve unions the --experimental flag value (comma-separated) with the
// names the config resolved from [cell] experimental and DEVCELL_EXPERIMENTAL
// (cfg.CellSection.ResolvedExperimental; the environment is read there, like
// every other devcell option). Unknown names are an *UnknownError, inactive
// ones an *InactiveError, so a typo or a stale setting cannot silently leave
// a feature off.
func Resolve(flagCSV string, configured []string) (Set, error) {
	set := Set{}
	add := func(source string, names []string) error {
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			x, ok := Lookup(n)
			if !ok {
				return &UnknownError{Source: source, Name: n}
			}
			if x.State != Active {
				return &InactiveError{Experiment: x}
			}
			set[n] = true
		}
		return nil
	}
	if err := add(Flag, strings.Split(flagCSV, ",")); err != nil {
		return nil, err
	}
	if err := add(Key+" or "+Env, configured); err != nil {
		return nil, err
	}
	return set, nil
}

// Require returns a *RequiredError naming what (e.g. "the windows guest")
// unless x is enabled in s. It is the one-line gate for a code path that
// needs an experiment.
func Require(s Set, x Experiment, what string) error {
	if s.Enabled(x) {
		return nil
	}
	return &RequiredError{What: what, Experiment: x}
}
