package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/scaffold"
	"github.com/DimmKirr/devcell/internal/ux"
)

// Init scaffolds .devcell.toml and the .devcell/ build context in the
// project, then locks the nix flake inputs, or updates them with
// opts.Update or opts.Force. It builds no image; Build does.
func (Engine) Init(_ context.Context, opts engine.InitOpts) error {
	c := opts.Cell
	ux.Debugf("BaseDir: %s, ConfigDir: %s", c.BaseDir, c.ConfigDir)
	if opts.Stack != "" {
		ux.Debugf("stack: %s (--stack flag)", opts.Stack)
	}

	if _, err := scaffold.RunInitFlow(scaffold.InitFlowOptions{
		BaseDir:    c.BaseDir,
		ConfigDir:  c.ConfigDir,
		NixhomeSrc: c.Config.Nix.NixhomePath,
		Stack:      opts.Stack,
		Modules:    opts.Modules,
		Yes:        opts.Yes,
		Force:      opts.Force,
	}); err != nil {
		return err
	}

	// The build dir is .devcell/ now that .devcell.toml exists.
	buildDir := config.ResolveBuildDir(c.BaseDir, c.ConfigDir, true)
	fmt.Printf(" Created .devcell.toml + .devcell/ in %s\n", c.BaseDir)

	lockOnly, label := flakeLockMode(opts.Update, opts.Force)
	if err := updateFlakeLockWithSpinner(buildDir, lockOnly, label); err != nil {
		return err
	}

	fmt.Println(" Run 'cell build' to build the image, or 'cell claude' to build and start.")
	return nil
}

// flakeLockMode reports whether Init only locks the flake inputs (neither
// update nor force) or updates them, and the spinner label for it.
func flakeLockMode(update, force bool) (lockOnly bool, label string) {
	if update || force {
		return false, "Updating nix flake inputs"
	}
	return true, "Resolving nix flake inputs"
}

// updateFlakeLockWithSpinner runs nix flake lock/update with a spinner.
func updateFlakeLockWithSpinner(configDir string, lockOnly bool, label string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var buf bytes.Buffer
	var out io.Writer = &buf
	if ux.Verbose {
		out = os.Stdout
	}
	sp := ux.NewProgressSpinner(label)
	if err := UpdateFlakeLock(ctx, configDir, lockOnly, ux.Verbose, out); err != nil {
		sp.Fail(label + " failed")
		if !ux.Verbose && buf.Len() > 0 {
			fmt.Fprint(os.Stderr, buf.String())
		}
		return err
	}
	sp.Success(label)
	return nil
}
