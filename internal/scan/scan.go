package scan

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/storage"
	"github.com/devcell-sh/devcell/internal/ux"
)

// Caches scans all cache locations. When all is true, every workspace under
// ~/.devcell/ is scanned; otherwise only the workspace for cwd.
func Caches(all bool) CacheResult {
	home, _ := os.UserHomeDir()
	devcellHome := filepath.Join(home, ".devcell")

	var workspaces []string
	if all {
		workspaces, _ = ResolveWorkspaces(devcellHome, "", true)
	} else {
		c := config.Load(MustCwd(), os.Getenv)
		workspaces = []string{c.WorkspaceName}
	}

	var oldCaches []string
	for _, ws := range workspaces {
		wsHome := filepath.Join(devcellHome, ws)
		oldCaches = append(oldCaches, storage.FindOldWorkspaceCaches(wsHome)...)
	}

	oldWinkit, _ := storage.FindOldWinkitCaches(devcellHome)
	oldMackit, hasMackit := storage.FindOldMackitCache(home)

	oldUnified := filepath.Join(devcellHome, "cache")
	hasOldUnified := DirNonEmpty(oldUnified)

	unifiedCache := filepath.Join(home, ".cache", "devcell")
	hasUnified := DirNonEmpty(unifiedCache)

	r := CacheResult{
		OldCaches:    oldCaches,
		OldWinkit:    oldWinkit,
		OldMackit:    oldMackit,
		HasMackit:    hasMackit,
		OldUnified:   oldUnified,
		HasOldUnif:   hasOldUnified,
		UnifiedDir:   unifiedCache,
		HasUnified:   hasUnified,
		DevcellHome:  devcellHome,
		HomeDir:      home,
		UnifiedCache: unifiedCache,
	}

	dirs := r.AllDirs()
	r.Sizes = make(map[string]int64, len(dirs))
	for _, d := range dirs {
		sz := DirSize(d)
		r.Sizes[d] = sz
		r.TotalBytes += sz
	}
	return r
}

// Solitary scans for solitary clones in the current directory.
func Solitary() SolitaryResult {
	cwd := MustCwd()
	solitaryDir := filepath.Join(cwd, ".devcell", "solitary")

	entries, err := os.ReadDir(solitaryDir)
	if err != nil {
		return SolitaryResult{}
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(solitaryDir, e.Name()))
		}
	}

	sizes := make(map[string]int64, len(dirs))
	var totalBytes int64
	for _, d := range dirs {
		sz := DirSize(d)
		sizes[d] = sz
		totalBytes += sz
	}
	return SolitaryResult{Dirs: dirs, Sizes: sizes, TotalBytes: totalBytes}
}

// Docker scans Docker disk usage, stale cells, and live closures.
func Docker(ctx context.Context) DockerResult {
	du := docker.QueryDiskUsage(ctx)
	du.RunningCells = docker.QueryRunningCells(ctx)

	vmDisk, vmErr := docker.CollectVMDisk(ctx)

	stale, err := docker.FindStaleCells(ctx, "")
	if err != nil {
		stale = nil
	}
	closures, failures, closureErr := docker.CollectLiveClosures(
		func() ([]string, error) { return docker.DockerRunningDevcellContainers(ctx) },
		func(container, link string) (string, error) {
			return docker.DockerResolveContainerLink(ctx, container, link)
		},
		ux.Debugf,
	)
	return DockerResult{
		DiskUsage:       du,
		VMDisk:          vmDisk,
		VMDiskOK:        vmErr == nil,
		Stale:           stale,
		LiveClosures:    closures,
		ClosureFailures: failures,
		ClosureErr:      closureErr,
	}
}

// Cells scans running cell info and resource stats.
func Cells(ctx context.Context) CellsResult {
	cells, cellsErr := docker.ListCells(ctx, false)
	var names []string
	for _, c := range cells {
		names = append(names, c.Name)
	}
	var stats []docker.CellStats
	var statsErr error
	if len(names) > 0 {
		stats, statsErr = docker.ListCellStats(ctx, names)
	}
	return CellsResult{
		Cells:    cells,
		Stats:    stats,
		CellsErr: cellsErr,
		StatsErr: statsErr,
	}
}

// Nix scans the nix store volume.
func Nix(ctx context.Context) NixResult {
	report, ok := docker.CollectNixStore(ctx)
	return NixResult{NixReport: report, NixOK: ok}
}
