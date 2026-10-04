//go:build ignore

// catalogen publishes the home flake's module/stack/package catalog as
// static JSON for the site: web/public/schema/<version>/devcell-sh/home/{modules,stacks,packages}.json,
// served at https://devcell.sh/schema/<version>/devcell-sh/home/. Wired into
// `task schema:catalog`; not a dep of cell:build because it needs nix and
// fetches the flake's inputs.
//
// Usage: go run cmd/catalogen.go [-version v0.0.0] [-flake <ref>] [-out <dir>]
// -flake defaults to the home ref a cell binary of that version uses
// (nixhome.UpstreamFlakeRef: the matching tag, or main for v0.0.0); pass
// path:/path/to/home to publish a local checkout.
// Excluded from normal builds by the ignore tag above, like cmd/hmoptgen.go.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/nixhome"
)

func main() {
	version := flag.String("version", cfg.DefaultSchemaVersion, "published schema version (release tag)")
	flake := flag.String("flake", "", "flake ref exposing devcellCatalog (default: the upstream home ref for -version)")
	out := flag.String("out", "", "output directory (default: web/public/schema/<version>/devcell-sh/home)")
	flag.Parse()
	if *flake == "" {
		*flake = nixhome.UpstreamFlakeRef(*version)
	}
	if *out == "" {
		*out = filepath.Join("web", "public", nixhome.CatalogPath(*version))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	raw, err := nixhome.ReadFullCatalogFromFlake(ctx, *flake)
	if err != nil {
		fail(err)
	}
	files, err := nixhome.SplitCatalog(raw)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(*out, name), b, 0o644); err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s\n", filepath.Join(*out, name))
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "catalogen: %v\n", err)
	os.Exit(1)
}
