package scan

import "github.com/devcell-sh/devcell/internal/engine/docker"

// CacheResult holds the filesystem scan for all cache locations.
type CacheResult struct {
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

func (r CacheResult) Count() int {
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

func (r CacheResult) AllDirs() []string {
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

// SolitaryResult holds the scan for solitary clones.
type SolitaryResult struct {
	Dirs       []string
	Sizes      map[string]int64
	TotalBytes int64
}

// BuildResult holds the combined Docker + nix preflight scan.
type BuildResult struct {
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
func (r BuildResult) NixLiveHashes() int {
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

// DockerResult holds the Docker-only portion of the build scan.
type DockerResult struct {
	DiskUsage       docker.DiskUsage
	VMDisk          docker.VMDiskInfo
	VMDiskOK        bool
	Stale           []docker.StaleCellInfo
	LiveClosures    []docker.LiveClosure
	ClosureFailures []docker.ClosureFailure
	ClosureErr      error
}

// NixResult holds the nix volume portion of the build scan.
type NixResult struct {
	NixReport docker.NixStoreReport
	NixOK     bool
}

// CellsResult holds running cell info and resource stats.
type CellsResult struct {
	Cells    []docker.CellInfo
	Stats    []docker.CellStats
	CellsErr error
	StatsErr error
}

// MergeBuild combines docker and nix results into a single BuildResult.
func MergeBuild(d DockerResult, n NixResult) BuildResult {
	return BuildResult{
		DiskUsage:       d.DiskUsage,
		NixOK:           n.NixOK,
		NixReport:       n.NixReport,
		Stale:           d.Stale,
		LiveClosures:    d.LiveClosures,
		ClosureFailures: d.ClosureFailures,
		ClosureErr:      d.ClosureErr,
	}
}
