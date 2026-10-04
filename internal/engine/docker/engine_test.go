package docker

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
)

func TestEngine_RegisteredAsDocker(t *testing.T) {
	e, err := engine.For(engine.Docker)
	if err != nil {
		t.Fatalf("engine.For(docker): %v", err)
	}
	if _, ok := e.(Engine); !ok {
		t.Fatalf("engine.For(docker) = %T, want docker.Engine", e)
	}
}

// Every engine.Cell field docker reads reaches the config.Config that
// BuildArgv, the prompt files and the boot checklist take.
func TestHostConfig_MapsEveryCellField(t *testing.T) {
	c := engine.Cell{
		Name:          "dev",
		Home:          "/home/u/.devcell/dev",
		HostHome:      "/home/u",
		HostUser:      "u",
		BaseDir:       "/home/u/proj",
		ConfigDir:     "/home/u/.config/devcell",
		BuildDir:      "/home/u/proj/.devcell",
		VNCPort:       "35950",
		RDPPort:       "35989",
		Bunk:          "3",
		AppName:       "proj-3",
		ContainerName: "cell-proj-3-run",
		Hostname:      "cell-proj-3",
	}
	want := config.Config{
		Bunk:          "3",
		AppName:       "proj-3",
		CellName:      "dev",
		CellHome:      "/home/u/.devcell/dev",
		ConfigDir:     "/home/u/.config/devcell",
		BuildDir:      "/home/u/proj/.devcell",
		ContainerName: "cell-proj-3-run",
		Hostname:      "cell-proj-3",
		VNCPort:       "35950",
		RDPPort:       "35989",
		BaseDir:       "/home/u/proj",
		HostUser:      "u",
		HostHome:      "/home/u",
	}
	if got := hostConfig(c); got != want {
		t.Errorf("hostConfig =\n  %+v\nwant\n  %+v", got, want)
	}
}

func TestNewBuildParams(t *testing.T) {
	cell := engine.Cell{Stack: "base"}
	tomlGo := engine.Cell{Stack: "go", Config: cfg.CellConfig{Cell: cfg.CellSection{Stack: "go"}}}
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}

	tests := []struct {
		name   string
		opts   engine.BuildOpts
		getenv func(string) string
		want   buildParams
	}{
		{"defaults", engine.BuildOpts{Cell: cell}, env(),
			buildParams{stack: "base"}},
		{"[cell] stack is named in the label", engine.BuildOpts{Cell: tomlGo}, env(),
			buildParams{stack: "go", explicitStack: true}},
		{"--stack", engine.BuildOpts{Cell: cell, Stack: "node"}, env(),
			buildParams{stack: "node", explicitStack: true}},
		{"DEVCELL_STACK", engine.BuildOpts{Cell: cell}, env("DEVCELL_STACK", "python"),
			buildParams{stack: "python", explicitStack: true}},
		{"--stack beats DEVCELL_STACK", engine.BuildOpts{Cell: cell, Stack: "node"}, env("DEVCELL_STACK", "python"),
			buildParams{stack: "node", explicitStack: true}},
		{"--image", engine.BuildOpts{Cell: cell, Image: "devcell-user:x"}, env("DEVCELL_BUILD_IMAGE", "devcell-user:env"),
			buildParams{stack: "base", image: "devcell-user:x"}},
		{"DEVCELL_BUILD_IMAGE", engine.BuildOpts{Cell: cell}, env("DEVCELL_BUILD_IMAGE", "devcell-user:env"),
			buildParams{stack: "base", image: "devcell-user:env"}},
		{"--update recreates the nix volume", engine.BuildOpts{Cell: cell, Update: true}, env(),
			buildParams{stack: "base", recreateVolume: true}},
		{"--build-threads", engine.BuildOpts{Cell: cell, BuildThreads: 16}, env(),
			buildParams{stack: "base", threads: 16}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newBuildParams(tt.opts, tt.getenv)
			if err != nil {
				t.Fatalf("newBuildParams: %v", err)
			}
			if got != tt.want {
				t.Errorf("newBuildParams =\n  %+v\nwant\n  %+v", got, tt.want)
			}
		})
	}
}

func TestNewBuildParams_RejectsUnknownStack(t *testing.T) {
	_, err := newBuildParams(engine.BuildOpts{Stack: "bogus"}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("newBuildParams(Stack: bogus) = %v, want an unknown-stack error", err)
	}
}

// The build Run starts itself uses the cell's own stack and tag and keeps
// the nix-store volume.
func TestAutoBuildParams(t *testing.T) {
	c := engine.Cell{Stack: "go", Config: cfg.CellConfig{Cell: cfg.CellSection{Stack: "go"}}}
	if got, want := autoBuildParams(c), (buildParams{stack: "go", explicitStack: true}); got != want {
		t.Errorf("autoBuildParams = %+v, want %+v", got, want)
	}
}

func TestNewRunSpec_MapsOpts(t *testing.T) {
	cellCfg := cfg.CellConfig{Cell: cfg.CellSection{Stack: "go"}}
	opts := engine.RunOpts{
		Cell:         engine.Cell{Name: "dev", Config: cellCfg},
		Binary:       "claude",
		DefaultFlags: []string{"--yolo"},
		Args:         []string{"-c"},
		Env:          map[string]string{"A": "1"},
		Debug:        true,
		Detach:       true,
		NoSecrets:    true,
		NixDaemon:    true,
		NoPorts:      true,
	}
	c := config.Config{CellName: "dev", AppName: "proj-0"}

	got := newRunSpec(opts, c)

	want := RunSpec{
		Config:       c,
		CellCfg:      cellCfg,
		Binary:       "claude",
		DefaultFlags: []string{"--yolo"},
		UserArgs:     []string{"-c"},
		Debug:        true,
		NixDaemon:    true,
		NoPorts:      true,
		ExtraEnv:     map[string]string{"A": "1"},
		ThinImage:    true,
		Detach:       true,
		NoSecrets:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newRunSpec =\n  %+v\nwant\n  %+v", got, want)
	}
}

// A non-zero agent exit is *engine.ExitError with the agent's status, which
// cmd/ exits with.
func TestAgentExit(t *testing.T) {
	if err := agentExit(nil); err != nil {
		t.Errorf("agentExit(nil) = %v, want nil", err)
	}

	waitErr := exec.Command("sh", "-c", "exit 3").Run()
	var exitErr *engine.ExitError
	if !errors.As(agentExit(waitErr), &exitErr) || exitErr.Code != 3 {
		t.Errorf("agentExit(exit 3) = %v, want *engine.ExitError{Code: 3}", agentExit(waitErr))
	}

	other := errors.New("boom")
	if err := agentExit(other); !errors.Is(err, other) {
		t.Errorf("agentExit(other) = %v, want it unchanged", err)
	}
}

// Init only locks the flake inputs unless --update or --force asks for the
// latest revisions.
func TestFlakeLockMode(t *testing.T) {
	tests := []struct {
		update, force bool
		wantLockOnly  bool
		wantLabel     string
	}{
		{false, false, true, "Resolving nix flake inputs"},
		{true, false, false, "Updating nix flake inputs"},
		{false, true, false, "Updating nix flake inputs"},
	}
	for _, tt := range tests {
		lockOnly, label := flakeLockMode(tt.update, tt.force)
		if lockOnly != tt.wantLockOnly || label != tt.wantLabel {
			t.Errorf("flakeLockMode(update=%v, force=%v) = %v, %q; want %v, %q",
				tt.update, tt.force, lockOnly, label, tt.wantLockOnly, tt.wantLabel)
		}
	}
}
