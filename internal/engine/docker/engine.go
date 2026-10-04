package docker

import (
	"context"
	"os"
	"strconv"

	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
)

// Engine runs a cell as a docker container built from a thin image: a small
// image whose /nix lives on the shared nix-store volume. It registers itself
// as engine.Docker.
//
// Docker-only commands (cell stop, cell cleanup, cell build df, cell build
// prune) are not engine verbs; cmd/ calls this package for them directly.
type Engine struct{}

var _ engine.Engine = Engine{}

func init() {
	engine.Register(engine.Docker, Engine{})
}

// Build builds the cell's thin image: it syncs the nixhome flake into the
// build dir, writes the overlay flake for the cell's stack and modules, and
// runs home-manager and `docker build` in a builder container. opts.Stack
// (else DEVCELL_STACK) overrides the stack, opts.Image the tag, and
// opts.Update empties the nix-store volume first. opts.DryRun, opts.Force,
// opts.NoCache and opts.Stage are not honored.
func (Engine) Build(ctx context.Context, opts engine.BuildOpts) error {
	p, err := newBuildParams(opts, os.Getenv)
	if err != nil {
		return err
	}
	if p.threads > 0 {
		os.Setenv("DEVCELL_BUILD_THREADS", strconv.Itoa(p.threads))
	}
	useCell(opts.Cell)
	return build(ctx, opts.Cell, p)
}

// useCell points the image-tag globals (Stack, Modules, PerCellImage) at
// cell c, so UserImageTagThin and PickImageTagThin name its image.
func useCell(c engine.Cell) {
	Stack = c.Stack
	Modules = c.Modules
	PerCellImage = c.Config.Cell.ResolvedPerCellImage()
}

// hostConfig is the config.Config that BuildArgv, the prompt files and the
// boot checklist read, rebuilt from the engine.Cell cmd/ resolved. The
// fields docker does not read (ImageTag, Image, PortPrefix, LocalMode) stay
// zero.
func hostConfig(c engine.Cell) config.Config {
	return config.Config{
		Bunk:          c.Bunk,
		AppName:       c.AppName,
		CellName:      c.Name,
		CellHome:      c.Home,
		ConfigDir:     c.ConfigDir,
		BuildDir:      c.BuildDir,
		ContainerName: c.ContainerName,
		Hostname:      c.Hostname,
		VNCPort:       c.VNCPort,
		RDPPort:       c.RDPPort,
		BaseDir:       c.BaseDir,
		HostUser:      c.HostUser,
		HostHome:      c.HostHome,
	}
}
