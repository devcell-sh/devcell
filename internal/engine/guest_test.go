package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DimmKirr/devcell/internal/engine"
)

func TestDefaultGuest(t *testing.T) {
	for n, want := range map[engine.Name]engine.Guest{
		engine.Docker: engine.Linux,
		engine.Tart:   engine.MacOS,
		engine.Winkit: engine.WindowsPE,
	} {
		got, ok := engine.DefaultGuest(n)
		assert.True(t, ok, n)
		assert.Equal(t, want, got, n)
		assert.True(t, engine.Supports(n, got), "default guest of %s must be one it supports", n)
	}
	_, ok := engine.DefaultGuest("hyperv")
	assert.False(t, ok)
}

// GuestFor takes the --os value, then the [cell] os value, and skips any the
// engine cannot run: `--engine winkit` with a stale `[cell] os = "linux"`
// keeps running the default Windows guest instead of failing.
func TestGuestFor(t *testing.T) {
	cases := []struct {
		name     string
		engine   engine.Name
		osValues []string
		want     engine.Guest
	}{
		{"no os: engine default", engine.Winkit, []string{"", ""}, engine.WindowsPE},
		{"no values at all: engine default", engine.Tart, nil, engine.MacOS},
		{"--os value", engine.Winkit, []string{"winpe", ""}, engine.WindowsPE},
		{"[cell] os value", engine.Tart, []string{"", "macos"}, engine.MacOS},
		{"os the engine cannot run is skipped", engine.Winkit, []string{"", "linux"}, engine.WindowsPE},
		{"first supported value wins", engine.Docker, []string{"macos", "linux"}, engine.Linux},
		{"unknown os is skipped", engine.Docker, []string{"freebsd"}, engine.Linux},
		{"unknown engine", "hyperv", []string{"linux"}, ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, engine.GuestFor(c.engine, c.osValues...), c.name)
	}
}
