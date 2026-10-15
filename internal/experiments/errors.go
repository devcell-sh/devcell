package experiments

import (
	"fmt"
	"strings"
)

// UnknownError is a name that is not a registered experiment.
type UnknownError struct {
	Source string // Flag, Env or Key
	Name   string
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("%s: unknown experimental feature %q; available: %s", e.Source, e.Name, strings.Join(Names(), ", "))
}

// InactiveError is a registered experiment that can no longer be enabled.
type InactiveError struct{ Experiment }

func (e *InactiveError) Error() string {
	switch e.State {
	case Released:
		return fmt.Sprintf("experimental feature %q is released and no longer needs to be enabled; drop it from %s, %s or %s", e.Name, Flag, Env, Key)
	default:
		return fmt.Sprintf("experimental feature %q has been abandoned and cannot be enabled", e.Name)
	}
}

// RequiredError is a code path reached without the experiment it needs.
type RequiredError struct {
	What string // what was attempted, e.g. "the windows guest"
	Experiment
}

func (e *RequiredError) Error() string {
	return fmt.Sprintf("%s is experimental: enable it with %s (see docs/experimental.md)", e.What, e.HowToEnable())
}
