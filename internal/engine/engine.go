// Package engine names the runtimes a cell can run on, records which guest
// OS each one supports, and defines the verbs every runtime implements.
//
// cmd/ picks a Name with Resolve. The verbs are the target interface for
// every engine: an implementation lives in a subpackage, adds itself with
// Register from an init func, and cmd/ looks it up with For and calls Init,
// Build or Run. Docker (internal/engine/docker), tart
// (internal/engine/tart) and winkit (internal/engine/winkit) are registered.
package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// Engine is one runtime a cell can run on. Each verb gets everything it
// needs in its option struct. Engine-specific settings (tart SSH port,
// winkit ISO) come from Cell.Config, not from the caller.
type Engine interface {
	// Init prepares the host for this engine once, before the first Build:
	// base images, boxes, keys. It may prompt on stdin.
	Init(ctx context.Context, opts InitOpts) error

	// Build creates or refreshes the guest the cell runs in and applies the
	// cell's stack and modules to it.
	Build(ctx context.Context, opts BuildOpts) error

	// Run acquires the cell's guest (building it when it is missing or
	// opts.Rebuild is set), boots it, and runs the agent in the project
	// directory with the terminal attached. It returns when the agent exits.
	// A non-zero agent exit is returned as *ExitError.
	Run(ctx context.Context, opts RunOpts) error
}

// Cell is the cell and project a verb acts on, as cmd/ resolved them from
// flags, the environment and the merged config.
type Cell struct {
	Name      string         // cell name; the cell's home is ~/.devcell/<Name>
	Home      string         // the cell's home on the host, ~/.devcell/<Name>
	HostHome  string         // host user's home directory
	HostUser  string         // host user's login name ($USER); docker creates the same user in the container
	BaseDir   string         // project directory on the host
	ConfigDir string         // global config dir: $XDG_CONFIG_HOME/devcell or ~/.config/devcell
	BuildDir  string         // build context: <BaseDir>/.devcell when the project has a .devcell.toml, else ConfigDir
	Config    cfg.CellConfig // merged config, [env] expanded, engine flags folded in
	Stack     string         // resolved stack
	Modules   []string       // resolved [cell] modules
	VNCPort   string         // host port forwarded to the guest's VNC server
	RDPPort   string         // host port forwarded to the guest's RDP server

	// Bunk tells apart cells opened on the same project from different
	// terminal panes: DEVCELL_BUNK, else the tmux or zellij pane id.
	Bunk string
	// AppName is <project dir name>-<Bunk>. Docker names the container
	// after it and mounts the project at /<AppName>.
	AppName string
	// ContainerName is the docker container name, cell-<AppName>-run.
	ContainerName string
	// Hostname is the guest hostname when neither [cell] hostname nor
	// DEVCELL_HOSTNAME sets one: cell-<AppName>.
	Hostname string

	// Guest is the OS image the engine boots, from --os, else [cell] os,
	// else the engine's default (see GuestFor). Engines with more than one
	// guest (winkit: WindowsPE, WindowsFull) pick their build stage and
	// template from it; empty means the engine's DefaultGuest.
	Guest Guest
}

// WithStack returns c with Stack set to stack, or c unchanged when stack is
// empty. Engines apply a --stack override (BuildOpts.Stack, InitOpts.Stack)
// with it.
func (c Cell) WithStack(stack string) Cell {
	if stack != "" {
		c.Stack = stack
	}
	return c
}

// RunOpts configures Engine.Run.
type RunOpts struct {
	Cell         Cell
	Binary       string            // agent binary to run in the guest, e.g. "claude"
	DefaultFlags []string          // flags devcell always passes to Binary
	Args         []string          // the user's args, devcell's own flags stripped
	Env          map[string]string // agent env added after the cell's guest env, e.g. OPENCODE_CONFIG_CONTENT
	Rebuild      bool              // rebuild the guest before running (--build)
	Force        bool              // build a missing guest without asking first (--force)
	Update       bool              // update nix flake inputs, then rebuild (--update)
	Background   bool              // leave the guest running after the agent exits (--background)
	DryRun       bool              // print the commands instead of running them (--dry-run)
	Debug        bool              // log every step (--debug)

	// Detach starts the guest in the background and returns once it is up,
	// instead of attaching the terminal (cell start). Docker detaches the
	// container; winkit detaches the QEMU process.
	Detach bool
	// NoSecrets skips 1Password: no document is read and the agent does not
	// run under `op run` (--no-secrets, --skip-secrets, --no-1password).
	// Docker only.
	NoSecrets bool
	// NixDaemon starts nix-daemon in the guest so the agent can install
	// packages at runtime (--nix-daemon). Docker only.
	NixDaemon bool
	// NoPorts publishes no host ports, neither [ports] nor the GUI ports
	// (--no-ports). Docker only.
	NoPorts bool
	// UseFlake installs the packages of the project's flake.nix in the guest
	// once the user trusts it (--use-flake; [cell] flake turns it on too).
	// Docker only.
	UseFlake bool
	// AutoCleanup reaps nix-store roots no running cell uses before the
	// agent starts (--auto-cleanup). Docker only.
	AutoCleanup bool
}

// BuildOpts configures Engine.Build.
type BuildOpts struct {
	Cell Cell
	// Stack builds this stack instead of Cell.Stack (--stack); "" keeps
	// Cell.Stack. Docker validates it and falls back to DEVCELL_STACK.
	Stack   string
	Update  bool   // update nix flake inputs before applying the stack (--update)
	DryRun  bool   // print the commands instead of running them (--dry-run)
	Force   bool   // replace the guest image even if it already exists (--force)
	NoCache bool   // download the base image again instead of using the engine's cache (--no-cache)
	Stage   string // tart only: "base" builds the stack-agnostic base template; "full" or "" also applies the stack (--stage)

	// Image tags the built image instead of the default user image tag
	// (--image). Docker only; DEVCELL_BUILD_IMAGE is the fallback.
	Image string
	// BuildThreads sets nix's download parallelism, max-substitution-jobs
	// and http-connections (--build-threads); 0 keeps the default. Docker
	// only; it wins over DEVCELL_BUILD_THREADS and [build] threads.
	BuildThreads int
}

// InitOpts configures Engine.Init.
type InitOpts struct {
	Cell Cell
	// Stack is the --stack value, "" when it is not given. Tart and winkit
	// use it instead of Cell.Stack; docker writes it to the new
	// .devcell.toml, which defaults to the base stack.
	Stack string
	Force bool // regenerate what an earlier Init created, e.g. SSH keys (--force)

	// Modules is the explicit [cell] modules list written to a new
	// .devcell.toml (--modules). Docker only.
	Modules []string
	// Yes answers yes to every prompt, e.g. re-initializing a project that
	// already has a .devcell.toml (--yes). Docker only.
	Yes bool
	// Update updates the nix flake inputs to their latest revisions instead
	// of only locking them (--update; --force implies it). Docker only.
	Update bool
}

// ExitError reports that the agent ran and exited with a non-zero status.
// cmd/ exits with the same status.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("agent exited with status %d", e.Code)
}

var (
	mu      sync.RWMutex
	engines = map[Name]Engine{}
)

// Register makes e the implementation For returns for n. Engine packages
// call it from an init func. Like database/sql.Register, it panics if e is
// nil or n is already registered.
//
// Registration, rather than a switch in For, is what lets engine
// subpackages import this package for the option types without an import
// cycle.
func Register(n Name, e Engine) {
	mu.Lock()
	defer mu.Unlock()
	if e == nil {
		panic("engine: Register of nil Engine for " + string(n))
	}
	if _, dup := engines[n]; dup {
		panic("engine: Register called twice for " + string(n))
	}
	engines[n] = e
}

// For returns the implementation registered for n.
func For(n Name) (Engine, error) {
	mu.RLock()
	defer mu.RUnlock()
	if e, ok := engines[n]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("engine %q is not available in this build; see https://github.com/DimmKirr/devcell/issues", n)
}
