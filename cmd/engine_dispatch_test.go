package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
)

func withOSArgs(t *testing.T, args ...string) {
	t.Helper()
	old := osArgs
	osArgs = append([]string{"cell"}, args...)
	t.Cleanup(func() { osArgs = old })
}

func TestEngineCell_FillsEveryField(t *testing.T) {
	withOSArgs(t, "shell")
	c := config.Config{
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
	cellCfg := cfg.CellConfig{
		Cell: cfg.CellSection{Stack: "go", Modules: []string{"infra"}, OS: "macos"},
		Env:  map[string]string{"A": "1"},
	}

	got := engineCell(c, cellCfg)

	want := engine.Cell{
		Name:          "dev",
		Home:          "/home/u/.devcell/dev",
		HostHome:      "/home/u",
		HostUser:      "u",
		BaseDir:       "/home/u/proj",
		ConfigDir:     "/home/u/.config/devcell",
		BuildDir:      "/home/u/proj/.devcell",
		Config:        cellCfg,
		Stack:         "go",
		Modules:       []string{"infra"},
		VNCPort:       "35950",
		RDPPort:       "35989",
		Bunk:          "3",
		AppName:       "proj-3",
		ContainerName: "cell-proj-3-run",
		Hostname:      "cell-proj-3",
		Guest:         engine.MacOS,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("engineCell =\n  %+v\nwant\n  %+v", got, want)
	}
	// A field added to engine.Cell must be filled here too.
	v := reflect.ValueOf(got)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Errorf("engineCell leaves engine.Cell.%s empty", v.Type().Field(i).Name)
		}
	}
}

func TestDockerEngineIsLinkedIn(t *testing.T) {
	if _, err := engine.For(engine.Docker); err != nil {
		t.Errorf("engine.For(docker): %v", err)
	}
}

// Every cell flag an agent command scans lands in its RunOpts field, for
// whichever engine reads it.
func TestRunOpts_MapsCellFlags(t *testing.T) {
	c := engine.Cell{Name: "dev"}
	env := map[string]string{"A": "1"}

	t.Run("all set", func(t *testing.T) {
		withOSArgs(t, "claude", "--build", "--background", "--dry-run", "--debug", "--nix-daemon",
			"--no-ports", "--use-flake", "--auto-cleanup", "--no-secrets", "-c")
		startDetach = true
		t.Cleanup(func() { startDetach = false })

		got := runOpts(c, "claude", []string{"--yolo"}, []string{"-c"}, env)

		want := engine.RunOpts{
			Cell: c, Binary: "claude", DefaultFlags: []string{"--yolo"}, Args: []string{"-c"}, Env: env,
			Rebuild: true, Background: true, DryRun: true, Debug: true, Detach: true, NoSecrets: true,
			NixDaemon: true, NoPorts: true, UseFlake: true, AutoCleanup: true,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("runOpts =\n  %+v\nwant\n  %+v", got, want)
		}
	})
	t.Run("none set", func(t *testing.T) {
		withOSArgs(t, "claude")
		got := runOpts(c, "claude", nil, nil, nil)
		want := engine.RunOpts{Cell: c, Binary: "claude"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("runOpts =\n  %+v\nwant\n  %+v", got, want)
		}
	})
	for _, flag := range []string{"--skip-secrets", "--no-1password"} {
		t.Run(flag, func(t *testing.T) {
			withOSArgs(t, "claude", flag)
			if !runOpts(c, "claude", nil, nil, nil).NoSecrets {
				t.Errorf("%s did not set NoSecrets", flag)
			}
		})
	}
}

// commandWithFlags returns a fresh command whose flags are declared by
// declare, parsed from args. It fails the test unless real declares every
// one of those flags too, so the fresh command cannot drift from it.
func commandWithFlags(t *testing.T, real *cobra.Command, declare func(*cobra.Command), args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: real.Use}
	declare(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	names := map[string]bool{}
	for _, a := range args {
		name, _, _ := strings.Cut(strings.TrimLeft(a, "-"), "=")
		names[name] = true
	}
	for name := range names {
		if real.Flags().Lookup(name) == nil && real.Flags().ShorthandLookup(name) == nil {
			t.Errorf("cell %s has no --%s flag", real.Use, name)
		}
	}
	return cmd
}

func TestBuildOpts_MapsFlags(t *testing.T) {
	withOSArgs(t, "build", "--dry-run")
	declare := func(cmd *cobra.Command) {
		cmd.Flags().Bool("update", false, "")
		cmd.Flags().String("stack", "", "")
		cmd.Flags().String("image", "", "")
		cmd.Flags().Bool("force", false, "")
		cmd.Flags().Bool("no-cache", false, "")
		cmd.Flags().String("stage", "full", "")
		cmd.Flags().Int("build-threads", 0, "")
	}
	cmd := commandWithFlags(t, buildCmd, declare, "--update", "--stack=go", "--image=devcell-user:x",
		"--force", "--no-cache", "--stage=base", "--build-threads=16")
	c := engine.Cell{Name: "dev"}

	got := buildOpts(cmd, c)

	want := engine.BuildOpts{
		Cell: c, Stack: "go", Update: true, DryRun: true, Force: true, NoCache: true,
		Stage: "base", Image: "devcell-user:x", BuildThreads: 16,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildOpts =\n  %+v\nwant\n  %+v", got, want)
	}
}

func TestInitOpts_MapsFlags(t *testing.T) {
	declare := func(cmd *cobra.Command) {
		cmd.Flags().BoolP("yes", "y", false, "")
		cmd.Flags().Bool("force", false, "")
		cmd.Flags().Bool("update", false, "")
		cmd.Flags().String("stack", "", "")
		cmd.Flags().StringSlice("modules", nil, "")
	}
	cmd := commandWithFlags(t, initCmd, declare, "--yes", "--force", "--update", "--stack=go", "--modules=infra,go")
	c := engine.Cell{Name: "dev"}

	got := initOpts(cmd, c)

	want := engine.InitOpts{Cell: c, Stack: "go", Force: true, Modules: []string{"infra", "go"}, Yes: true, Update: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("initOpts =\n  %+v\nwant\n  %+v", got, want)
	}
}

func TestEngineCell_StackIsResolved(t *testing.T) {
	withOSArgs(t, "shell")
	var cellCfg cfg.CellConfig
	if got, want := engineCell(config.Config{}, cellCfg).Stack, cellCfg.Cell.ResolvedStack(); got != want {
		t.Errorf("Stack = %q, want the resolved default %q", got, want)
	}
}

func TestEngineCell_Guest(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		tomlOS string
		want   engine.Guest
	}{
		{"--os wins over [cell] os", []string{"--os", "winpe"}, "macos", engine.WindowsPE},
		{"--os=value form", []string{"--os=linux"}, "macos", engine.Linux},
		{"--macos stands for --os macos", []string{"--macos"}, "", engine.MacOS},
		{"[cell] os when --os is unset", nil, "windows", engine.WindowsFull},
		{"unknown [cell] os is skipped: docker's default", nil, "freebsd", engine.Linux},
		{"neither set: docker's default", nil, "", engine.Linux},
		{"neither set: the --engine's default", []string{"--engine", "winkit"}, "", engine.WindowsFull},
		{"[cell] os the --engine cannot run is skipped", []string{"--engine=winkit"}, "macos", engine.WindowsFull},
		{"deprecated --engine alias", []string{"--engine=qemu"}, "", engine.WindowsFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withOSArgs(t, append(tt.args, "shell")...)
			cellCfg := cfg.CellConfig{Cell: cfg.CellSection{OS: tt.tomlOS}}
			if got := engineCell(config.Config{}, cellCfg).Guest; got != tt.want {
				t.Errorf("Guest = %q, want %q", got, tt.want)
			}
		})
	}
}

// The tart engine runs commands with tart exec, so --tart-ssh-port and
// --tart-ssh-host have no effect: they are accepted (and warn) but no
// longer folded into the Config the engine gets.
func TestEngineCell_IgnoresTartSSHFlags(t *testing.T) {
	withOSArgs(t, "--tart-ssh-port=2222", "--tart-ssh-host", "10.0.0.5", "shell")
	cellCfg := cfg.CellConfig{Cell: cfg.CellSection{TartSSHPort: 22, TartSSHHost: "localhost"}}

	got := engineCell(config.Config{}, cellCfg).Config.Cell

	if got.TartSSHPort != 22 || got.TartSSHHost != "localhost" {
		t.Errorf("tart SSH flags must not be folded, got port=%d host=%q", got.TartSSHPort, got.TartSSHHost)
	}
}

// fakeEngine is registered once as "test-fake"; each test sets run.
type fakeEngine struct {
	run func(engine.RunOpts) error
}

func (f *fakeEngine) Init(context.Context, engine.InitOpts) error   { return nil }
func (f *fakeEngine) Build(context.Context, engine.BuildOpts) error { return nil }
func (f *fakeEngine) Run(_ context.Context, opts engine.RunOpts) error {
	return f.run(opts)
}

var (
	fake         = &fakeEngine{}
	registerFake sync.Once
)

func useFakeEngine(t *testing.T, run func(engine.RunOpts) error) engine.Name {
	t.Helper()
	registerFake.Do(func() { engine.Register("test-fake", fake) })
	fake.run = run
	t.Cleanup(func() { fake.run = nil })
	return "test-fake"
}

// recordExit replaces the process exit for the test and returns the codes
// runEngine exited with.
func recordExit(t *testing.T) *[]int {
	t.Helper()
	var codes []int
	old := exitProcess
	exitProcess = func(code int) { codes = append(codes, code) }
	t.Cleanup(func() { exitProcess = old })
	return &codes
}

func TestRunEngine_PassesOpts(t *testing.T) {
	var got engine.RunOpts
	n := useFakeEngine(t, func(opts engine.RunOpts) error { got = opts; return nil })
	want := engine.RunOpts{Cell: engine.Cell{Name: "dev"}, Binary: "claude", Args: []string{"-p", "hi"}, DryRun: true}

	if err := runEngine(n, want); err != nil {
		t.Fatalf("runEngine: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("engine got %+v, want %+v", got, want)
	}
}

func TestRunEngine_ExitsWithAgentStatus(t *testing.T) {
	codes := recordExit(t)
	n := useFakeEngine(t, func(engine.RunOpts) error { return &engine.ExitError{Code: 3} })

	_ = runEngine(n, engine.RunOpts{})

	if !reflect.DeepEqual(*codes, []int{3}) {
		t.Errorf("exit codes = %v, want [3]", *codes)
	}
}

func TestRunEngine_ReturnsOtherErrors(t *testing.T) {
	codes := recordExit(t)
	boom := errors.New("boom")
	n := useFakeEngine(t, func(engine.RunOpts) error { return boom })

	if err := runEngine(n, engine.RunOpts{}); !errors.Is(err, boom) {
		t.Errorf("runEngine = %v, want %v", err, boom)
	}
	if len(*codes) != 0 {
		t.Errorf("runEngine exited with %v on a non-exit error", *codes)
	}
}

func TestRunEngine_UnregisteredEngine(t *testing.T) {
	if err := runEngine("test-unregistered", engine.RunOpts{}); err == nil {
		t.Error("runEngine on an unregistered engine returned nil")
	}
}

func TestTartEngineIsLinkedIn(t *testing.T) {
	if _, err := engine.For(engine.Tart); err != nil {
		t.Errorf("engine.For(tart): %v", err)
	}
}
