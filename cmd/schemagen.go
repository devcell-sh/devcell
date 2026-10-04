//go:build ignore

// schemagen writes the generated devcell.toml JSON Schema. Wired into
// `task schema:generate` (a dep of cell:build) so
// web/public/schema/<version>/devcell.json is regenerated on every build and
// cannot drift from internal/cfg.CellConfig; the schema-generate pre-commit
// hook runs it too. Astro serves web/public/ verbatim, so the file is
// published at cfg.SchemaURLFor(version) on the next site deploy. main
// commits v0.0.0; pass -version to publish a release tag's directory.
//
// Usage: go run cmd/schemagen.go [-version v0.0.0] [-src internal/cfg/cfg.go] [-catalog dir] [-out path]
// -out defaults to the versioned path under web/public; "-" prints to stdout.
// Excluded from normal builds by the ignore tag above, like cmd/hmoptgen.go.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/nixhome"
)

func main() {
	src := flag.String("src", "internal/cfg/cfg.go", "Go source to read field comments from (schema descriptions)")
	version := flag.String("version", cfg.DefaultSchemaVersion, "published schema version (release tag)")
	out := flag.String("out", "", "output path (default: web/public/schema/<version>/devcell.json; \"-\" for stdout)")
	catalog := flag.String("catalog", "", "published home catalog dir (default: web/public/schema/<version>/devcell-sh/home); module/stack names become completions")
	flag.Parse()
	if *out == "" {
		*out = filepath.Join("web", "public", "schema", *version, "devcell.json")
	}
	if *catalog == "" {
		*catalog = filepath.Join("web", "public", nixhome.CatalogPath(*version))
	}

	srcBytes, err := os.ReadFile(*src)
	if err != nil {
		fail(err)
	}
	descs, err := cfg.FieldComments(srcBytes)
	if err != nil {
		fail(err)
	}
	opts := cfg.SchemaOptions{Version: *version, Descriptions: descs}
	if err := opts.LoadCatalog(*catalog); err != nil {
		fail(err)
	}
	schema, err := cfg.JSONSchema(opts)
	if err != nil {
		fail(err)
	}
	if *out == "-" {
		fmt.Print(string(schema))
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, schema, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "schemagen: %v\n", err)
	os.Exit(1)
}
