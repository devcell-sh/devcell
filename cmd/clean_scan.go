package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/storage"
	"github.com/devcell-sh/devcell/internal/ux"
)

// cacheScanResult holds the filesystem scan for all cache locations.
type cacheScanResult struct {
	OldCaches    []string
	OldWinkit    []string
	OldMackit    string
	HasMackit    bool
	OldUnified   string
	HasOldUnif   bool
	UnifiedDir   string
	HasUnified   bool
	DevcellHome  string
	HomeDir      string
	UnifiedCache string
	Sizes        map[string]int64
	TotalBytes   int64
}

func (r cacheScanResult) Count() int {
	n := len(r.OldCaches) + len(r.OldWinkit)
	if r.HasMackit {
		n++
	}
	if r.HasOldUnif {
		n++
	}
	if r.HasUnified {
		n++
	}
	return n
}

func (r cacheScanResult) AllDirs() []string {
	dirs := append([]string{}, r.OldCaches...)
	dirs = append(dirs, r.OldWinkit...)
	if r.HasMackit {
		dirs = append(dirs, r.OldMackit)
	}
	if r.HasOldUnif {
		dirs = append(dirs, r.OldUnified)
	}
	if r.HasUnified {
		dirs = append(dirs, r.UnifiedCache)
	}
	return dirs
}

func scanCaches(all bool) cacheScanResult {
	home, _ := os.UserHomeDir()
	devcellHome := filepath.Join(home, ".devcell")

	var workspaces []string
	if all {
		workspaces, _ = resolveWorkspaces(devcellHome, "", true)
	} else {
		c := config.Load(mustCwd(), os.Getenv)
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
	hasOldUnified := dirNonEmpty(oldUnified)

	unifiedCache := filepath.Join(home, ".cache", "devcell")
	hasUnified := dirNonEmpty(unifiedCache)

	r := cacheScanResult{
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
		sz := dirSize(d)
		r.Sizes[d] = sz
		r.TotalBytes += sz
	}
	return r
}

// solitaryScanResult holds the scan for solitary clones.
type solitaryScanResult struct {
	Dirs       []string
	Sizes      map[string]int64
	TotalBytes int64
}

func scanSolitary() solitaryScanResult {
	cwd := mustCwd()
	solitaryDir := filepath.Join(cwd, ".devcell", "solitary")

	entries, err := os.ReadDir(solitaryDir)
	if err != nil {
		return solitaryScanResult{}
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
		sz := dirSize(d)
		sizes[d] = sz
		totalBytes += sz
	}
	return solitaryScanResult{Dirs: dirs, Sizes: sizes, TotalBytes: totalBytes}
}

// buildScanResult holds the Docker + nix preflight scan.
type buildScanResult struct {
	DiskUsage       docker.DiskUsage
	NixReport       docker.NixStoreReport
	NixOK           bool
	Stale           []docker.StaleCellInfo
	LiveClosures    []docker.LiveClosure
	ClosureFailures []docker.ClosureFailure
	ClosureErr      error
}

// NixLiveHashes returns the count of profile hashes on the volume that are
// actively used by running containers.
func (r buildScanResult) NixLiveHashes() int {
	if !r.NixOK {
		return 0
	}
	liveSet := make(map[string]bool)
	for _, h := range docker.LiveHashes(r.LiveClosures) {
		liveSet[h] = true
	}
	count := 0
	for _, m := range r.NixReport.Metas {
		if liveSet[m.Hash] {
			count++
		}
	}
	return count
}

// dockerScanResult holds the Docker-only portion of the build scan.
type dockerScanResult struct {
	DiskUsage       docker.DiskUsage
	Stale           []docker.StaleCellInfo
	LiveClosures    []docker.LiveClosure
	ClosureFailures []docker.ClosureFailure
	ClosureErr      error
}

func scanDocker(ctx context.Context) dockerScanResult {
	du := docker.QueryDiskUsage(ctx)
	du.RunningCells = docker.QueryRunningCells(ctx)
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
	return dockerScanResult{
		DiskUsage:       du,
		Stale:           stale,
		LiveClosures:    closures,
		ClosureFailures: failures,
		ClosureErr:      closureErr,
	}
}

// nixScanResult holds the nix volume portion of the build scan.
type nixScanResult struct {
	NixReport docker.NixStoreReport
	NixOK     bool
}

func scanNix(ctx context.Context) nixScanResult {
	report, ok := docker.CollectNixStore(ctx)
	return nixScanResult{NixReport: report, NixOK: ok}
}

func mergeBuildScan(d dockerScanResult, n nixScanResult) buildScanResult {
	return buildScanResult{
		DiskUsage:       d.DiskUsage,
		NixOK:           n.NixOK,
		NixReport:       n.NixReport,
		Stale:           d.Stale,
		LiveClosures:    d.LiveClosures,
		ClosureFailures: d.ClosureFailures,
		ClosureErr:      d.ClosureErr,
	}
}
