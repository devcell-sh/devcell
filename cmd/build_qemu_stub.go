//go:build !((darwin || linux) && qemu)

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func runBuildQemu(cellName, hostHome, baseDir, stack string, force, noCache, dryRun bool, _ cfg.CellSection) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("cell build --engine=qemu requires macOS or Linux (current: %s/%s)", runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Errorf("cell build --engine=qemu requires the 'qemu' build tag; rebuild with: go build -tags qemu ./cmd/")
}

func qemuKeyDir(home, cellName string) string {
	legacy := filepath.Join(home, ".devcell", cellName, "qemu")
	if _, err := os.Stat(filepath.Join(legacy, "id_ed25519")); err == nil {
		return legacy
	}
	return filepath.Join(home, ".devcell", cellName, ".ssh")
}
