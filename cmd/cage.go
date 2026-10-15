package main

import (
	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/ux"
)

// startBootPopup opens the CELL-657 boot popup for this run, or returns nil
// when the inline panel must be used (old tmux, tiny terminal, popup failed
// to start). Every reason is logged at debug level.
func startBootPopup(c config.Config, cellCfg cfg.CellConfig, opts engine.RunOpts) *cage.Popup {
	geom, err := cage.PopupPreflight(cage.TmuxRunner("tmux"), docker.BootGroupCount(cellCfg))
	if err != nil {
		ux.Debugf("boot popup: %v; using the inline panel", err)
		return nil
	}
	// No prompt can be answered while the popup owns the keyboard: settle
	// the flake-trust question first. The answer is recorded per workspace,
	// so the engine's own call then finds it without asking.
	if opts.UseFlake || cellCfg.Cell.FlakeEnabled() {
		_ = docker.ResolveTrustFlake(c.BaseDir, c.WorkspaceHome)
	}
	theme, _ := cage.ResolveTheme(cellCfg.Theme)
	p, err := cage.StartPopup(cage.PopupSpec{
		SocketPath: cage.SocketPath(c.CellName),
		LogPath:    cage.StatePath(c.WorkspaceHome, "boot-"+c.CellName+".log"),
		Title:      " " + c.WorkspaceName + "  " + c.CellName + " ",
		Accent:     theme.Accent,
		Geometry:   geom,
	})
	if err != nil {
		ux.Debugf("boot popup: %v; using the inline panel", err)
		return nil
	}
	return p
}
