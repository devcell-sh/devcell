//go:build ignore

// expgen writes the generated experimental-features blocks from
// internal/experiments: the table between the experiments markers in
// README.md and docs/experimental.md, web/src/data/experimental.json for the
// site's <Exp> markers, and the website page web/src/content/cell/experimental.md.
// Wired into `task docs:experimental` (a dep of docs:generate); the
// experiments package test fails when the committed files drift.
//
// Usage: go run cmd/expgen.go [-root .]
// Excluded from normal builds by the ignore tag above, like cmd/gendoc.go.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/experiments"
)

const siteLinkBase = "https://github.com/devcell-sh/devcell/blob/main/"

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	replace(filepath.Join(*root, "README.md"), experiments.Markdown(""))
	replace(filepath.Join(*root, "docs", "experimental.md"), experiments.Markdown("../"))

	data, err := experiments.JSON()
	if err != nil {
		fail(err)
	}
	write(filepath.Join(*root, "web", "src", "data", "experimental.json"), append(data, '\n'))
	write(filepath.Join(*root, "web", "src", "content", "cell", "experimental.md"), []byte(sitePage()))
}

// replace rewrites the block between the markers in path.
func replace(path, body string) {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	out, err := experiments.ReplaceBlock(string(b), body)
	if err != nil {
		fail(fmt.Errorf("%s: %w", path, err))
	}
	write(path, []byte(out))
}

func sitePage() string {
	return "---\ntitle: \"Experimental features\"\ndescription: \"devcell's experimental features, how to turn them on, and what each one does.\"\n---\n\n" +
		"## Experimental features\n\n" +
		"Features marked <sup>e</sup> across the site are experimental. " +
		experiments.Markdown(siteLinkBase) +
		"\n### Lifecycle\n\n" +
		"An experiment stays listed here while it can be enabled. Once it ships, the name is *released*: the setting becomes a no-op error that says so. Once dropped, it is *abandoned* and fails the same way. Either way an old `devcell.toml` tells you what happened instead of silently ignoring the name.\n\n" +
		"Unknown names stop devcell with the list of known experiments, so a typo cannot leave a feature off.\n"
}

func write(path string, b []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fail(err)
	}
	fmt.Println("wrote", path)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "expgen:", err)
	os.Exit(1)
}
