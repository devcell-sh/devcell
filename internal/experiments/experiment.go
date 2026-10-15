// Package experiments is the registry of devcell's experimental features and
// the gate that turns them on.
//
// An experiment is off until named in --experimental=a,b, DEVCELL_EXPERIMENTAL
// or [cell] experimental (the three are unioned). Code guards a feature with
// Set.Enabled or Require; docs are generated from List (see Markdown and
// JSON, wired into `task docs:experimental`). Names are "<group>-<slug>":
// the group is the area the experiment touches (ui, engine).
//
// Lifecycle, after go-task's experiments package: an experiment is Active
// while it can be enabled; once shipped it becomes Released, once dropped
// Abandoned. Both stay registered so an old setting fails with a reason
// instead of an "unknown experiment" error.
package experiments

import "strings"

// State is where an experiment is in its lifecycle.
type State int

const (
	Active    State = iota // can be enabled
	Released               // shipped: always on, the setting is a no-op error
	Abandoned              // dropped: cannot be enabled
)

// String is the lower-case state name used in JSON and docs.
func (s State) String() string {
	switch s {
	case Released:
		return "released"
	case Abandoned:
		return "abandoned"
	}
	return "active"
}

// Experiment describes one experimental feature.
type Experiment struct {
	Name    string // what users write: "<group>-<slug>", e.g. "ui-cage"
	Summary string // one line: what turning it on does
	Doc     string // repo-relative markdown path, e.g. "docs/cage.md"; "" when none
	Ticket  string // tracking ticket, e.g. "CELL-655"; "" when none
	State   State
}

// Group is the area prefix of Name: "ui" for "ui-cage".
func (x Experiment) Group() string {
	group, _, _ := strings.Cut(x.Name, "-")
	return group
}

// Flag is the command-line form that enables x.
func (x Experiment) Flag() string { return Flag + "=" + x.Name }

// EnvSetting is the environment form that enables x.
func (x Experiment) EnvSetting() string { return Env + "=" + x.Name }

// TOMLSetting is the devcell.toml form that enables x.
func (x Experiment) TOMLSetting() string { return Key + ` = ["` + x.Name + `"]` }

// HowToEnable lists every way to turn x on, for error messages and docs.
func (x Experiment) HowToEnable() string {
	return x.Flag() + ", " + x.EnvSetting() + " or " + x.TOMLSetting()
}
