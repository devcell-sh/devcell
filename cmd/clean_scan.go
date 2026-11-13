package main

import (
	"context"

	"github.com/devcell-sh/devcell/internal/scan"
)

// Type aliases so callers in cmd/ keep compiling without import changes.
type cacheScanResult = scan.CacheResult
type solitaryScanResult = scan.SolitaryResult
type buildScanResult = scan.BuildResult
type dockerScanResult = scan.DockerResult
type nixScanResult = scan.NixResult

func scanCaches(all bool) cacheScanResult   { return scan.Caches(all) }
func scanSolitary() solitaryScanResult      { return scan.Solitary() }
func scanDocker(ctx context.Context) dockerScanResult { return scan.Docker(ctx) }
func scanNix(ctx context.Context) nixScanResult       { return scan.Nix(ctx) }
func mergeBuildScan(d dockerScanResult, n nixScanResult) buildScanResult {
	return scan.MergeBuild(d, n)
}
