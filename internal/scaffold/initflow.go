package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/ollama"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
)

// InitFlowOptions configures the shared initialization flow.
type InitFlowOptions struct {
	BaseDir    string   // project root directory
	ConfigDir  string   // global config directory (~/.config/devcell)
	NixhomeSrc string   // nixhome source: local path, git URL, or "" for upstream
	Stack      string   // explicit stack name ("" means base)
	Modules    []string // explicit modules (non-empty implies base stack when Stack is "")
	Yes        bool     // skip interactive prompts, use defaults
	Force      bool     // overwrite existing files
	Offline    bool     // skip network I/O (git clone, ollama probe)
}

// Network-touching steps of RunInitFlow; tests replace them.
var (
	resolveNixhome = ResolveNixhome
	detectOllama   = detectOllamaModels
)

// InitFlowResult holds the output of a successful init flow.
type InitFlowResult struct {
	Stack    string
	Modules  []string
	BuildDir string
}

// RunInitFlow is the shared init logic used by both the docker engine's
// `cell init` and the first-run scaffold of every agent command.
// It resolves nixhome, takes the stack and modules from opts (stack defaults
// to "base"; there is no interactive picker), and scaffolds the project.
func RunInitFlow(opts InitFlowOptions) (*InitFlowResult, error) {
	buildDir := filepath.Join(opts.BaseDir, ".devcell")

	// Check if already initialized — ask to overwrite unless -y or --force.
	if !opts.Force && !opts.Yes {
		if _, err := os.Stat(filepath.Join(opts.BaseDir, ".devcell.toml")); err == nil {
			overwrite, cErr := ux.GetConfirmation("Project already initialized. Re-initialize?")
			if cErr != nil {
				return nil, fmt.Errorf("confirmation: %w", cErr)
			}
			if !overwrite {
				return nil, fmt.Errorf("cancelled")
			}
			opts.Force = true
		}
	}

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", buildDir, err)
	}

	// Resolve nixhome into .devcell/nixhome/.
	if !opts.Offline {
		if err := resolveNixhome(opts.NixhomeSrc, buildDir, version.Version, opts.Force); err != nil {
			ux.Debugf("Failed to resolve nixhome: %v (falling back to built-in lists)", err)
		}
	}
	if err := validateNixhomeStructure(filepath.Join(buildDir, "nixhome")); err != nil {
		return nil, err
	}

	// For scaffold: pass nixhomeSrc only if it's a local path (persisted in .devcell.toml).
	nixhomePath := ""
	if opts.NixhomeSrc != "" && !IsGitURL(opts.NixhomeSrc) {
		nixhomePath = opts.NixhomeSrc
	}

	stack := opts.Stack
	modules := opts.Modules

	if len(modules) > 0 && stack == "" {
		stack = "base" // explicit modules imply base stack
	}

	// Stack picker is deprecated. Default to "base" silently when no stack
	// is configured — stacks themselves are being phased out in favour of
	// explicit [cell].modules lists (Modules 2.0). The interactive picker
	// also broke `cell shell` / `cell claude` whenever stdin wasn't a TTY
	// (CI runs, `cell shell -- cmd`, scripted invocations). See CELL-1.
	if stack == "" {
		stack = "base"
	}

	// Detect ollama models.
	modelsSnippet := ""
	if !opts.Offline {
		modelsSnippet = detectOllama()
	}

	// Scaffold.
	fmt.Printf(" Initializing %s\n", opts.BaseDir)
	if err := ScaffoldWithModules(opts.BaseDir, modelsSnippet, nixhomePath, opts.Force, stack, modules); err != nil {
		return nil, fmt.Errorf("scaffold: %w", err)
	}

	return &InitFlowResult{
		Stack:    stack,
		Modules:  modules,
		BuildDir: buildDir,
	}, nil
}

// --- Helpers used by RunInitFlow (moved from init.go) ---

// validateNixhomeStructure checks that the nixhome directory has the expected
// stacks/ and modules/ subdirectories. Returns an error if the structure is
// incompatible with devcell (no stacks/ or no modules/).
func validateNixhomeStructure(nixhomePath string) error {
	if nixhomePath == "" {
		return nil
	}
	if _, err := os.Stat(nixhomePath); err != nil {
		return nil // nixhome not fetched — will use defaults
	}
	var missing []string
	if _, err := os.Stat(filepath.Join(nixhomePath, "stacks")); err != nil {
		missing = append(missing, "stacks/")
	}
	if _, err := os.Stat(filepath.Join(nixhomePath, "modules")); err != nil {
		missing = append(missing, "modules/")
	}
	if len(missing) > 0 {
		return fmt.Errorf("nixhome at %s is not devcell-compatible (missing %s). Expected stacks/*.nix and modules/*.nix",
			nixhomePath, strings.Join(missing, ", "))
	}
	return nil
}

// detectOllamaModels tries to detect ollama and returns a commented-out
// TOML snippet for .devcell.toml.
func detectOllamaModels() string {
	ctx := context.Background()
	if !ollama.Detect(ctx, ollama.DefaultBaseURL) {
		return ""
	}
	models, err := ollama.FetchModels(ctx, ollama.DefaultBaseURL)
	if err != nil || len(models) == 0 {
		return ""
	}
	systemRAM := ollama.GetSystemRAMGB()
	ranked := ollama.RankModels(models, 10, nil, nil, systemRAM, "")
	snippet := ollama.FormatTOMLSnippet(ranked)
	if snippet != "" {
		fmt.Printf(" Detected ollama with %d models\n", len(ranked))
	}
	return snippet
}
