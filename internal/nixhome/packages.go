package nixhome

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// PackageDefinition is one home.packages assignment: the module file that
// made it and the packages it contributed. Mirrors home-manager's
// options.home.packages.definitionsWithLocations.
type PackageDefinition struct {
	File     string   `json:"file"`
	Packages []string `json:"packages"`
}

// ModuleCount is the number of packages one devcell module contributes.
type ModuleCount struct {
	Module string
	Count  int
}

// packageEvalExpr is the --apply function for
// homeConfigurations.<name>.options.home.packages. It returns the file
// that declares the option (home-manager's own tree, used to filter
// home-manager internals out) plus every definition with its location.
const packageEvalExpr = `o: {
  hm = builtins.head o.declarations;
  defs = map (d: {
    file = d.file;
    packages = map (p: p.pname or (p.name or "<unnamed>")) d.value;
  }) o.definitionsWithLocations;
}`

var errNotFound = errors.New("not found")

// EvalPackageDefinitions evaluates the home.packages definitions of one
// homeConfiguration in flakeRef. Returns nil, "", nil when nix is not in
// PATH so the docker build path keeps working on hosts without nix.
func EvalPackageDefinitions(ctx context.Context, flakeRef, homeConfig string) ([]PackageDefinition, string, error) {
	return evalPackageDefinitionsWithLookPath(ctx, flakeRef, homeConfig, exec.LookPath)
}

func evalPackageDefinitionsWithLookPath(ctx context.Context, flakeRef, homeConfig string, lookPath func(string) (string, error)) ([]PackageDefinition, string, error) {
	nixBin, err := lookPath("nix")
	if err != nil {
		return nil, "", nil
	}
	attr := fmt.Sprintf("%s#homeConfigurations.%s.options.home.packages", flakeRef, homeConfig)
	cmd := exec.CommandContext(ctx, nixBin, "eval", "--json", "--no-write-lock-file", attr, "--apply", packageEvalExpr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("evaluate packages of %s:\n%s", homeConfig, extractNixErrors(stderr.String()))
	}
	return ParsePackageEval(stdout.Bytes())
}

// ParsePackageEval decodes the JSON produced by packageEvalExpr.
func ParsePackageEval(raw []byte) ([]PackageDefinition, string, error) {
	var out struct {
		HM   string              `json:"hm"`
		Defs []PackageDefinition `json:"defs"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("parse package eval: %w", err)
	}
	return out.Defs, out.HM, nil
}

// GroupPackageDefinitions counts packages per devcell module. The module is
// the first path component under modules/ with its .nix suffix dropped, so
// modules/llm/claude.nix and modules/llm/codex.nix both count as llm.
// Definitions from home-manager's own tree (identified by hmDeclaration, the
// file declaring home.packages) are left out: users cannot toggle them.
// Definitions with no file, such as packages inlined in the overlay flake,
// count as "config". Sorted by count descending, then name.
func GroupPackageDefinitions(defs []PackageDefinition, hmDeclaration string) []ModuleCount {
	hmRoot := storeRoot(hmDeclaration)
	counts := map[string]int{}
	for _, d := range defs {
		if hmRoot != "" && strings.HasPrefix(d.File, hmRoot) {
			continue
		}
		counts[moduleOf(d.File)] += len(d.Packages)
	}
	out := make([]ModuleCount, 0, len(counts))
	for m, n := range counts {
		out = append(out, ModuleCount{Module: m, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Module < out[j].Module
	})
	return out
}

// FormatModuleCounts renders the counts inline, without a heading:
//
//	desktop: 69, base: 35, infra: 11
//
// This is what the thin image's devcell.packages label carries.
func FormatModuleCounts(counts []ModuleCount) string {
	parts := make([]string, len(counts))
	for i, c := range counts {
		parts[i] = fmt.Sprintf("%s: %d", c.Module, c.Count)
	}
	return strings.Join(parts, ", ")
}

// FormatPackageSummary renders one inline line for the build log:
//
//	Packages: desktop: 69, base: 35, infra: 11
func FormatPackageSummary(counts []ModuleCount) string {
	if len(counts) == 0 {
		return "Packages: none"
	}
	return "Packages: " + FormatModuleCounts(counts)
}

// storeRoot returns the /nix/store/<hash>-source/ prefix of a store path,
// or "" when p is not under the store.
func storeRoot(p string) string {
	const store = "/nix/store/"
	if !strings.HasPrefix(p, store) {
		return ""
	}
	rest := p[len(store):]
	i := strings.Index(rest, "/")
	if i < 0 {
		return p + "/"
	}
	return store + rest[:i+1]
}

// moduleOf maps a definition file to its module name.
func moduleOf(file string) string {
	const marker = "/modules/"
	i := strings.LastIndex(file, marker)
	if i < 0 {
		return "config"
	}
	rest := file[i+len(marker):]
	first := rest
	if j := strings.Index(rest, "/"); j >= 0 {
		first = rest[:j]
	}
	return strings.TrimSuffix(first, path.Ext(first))
}
