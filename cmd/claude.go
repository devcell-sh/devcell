package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/ollama"
	"github.com/spf13/cobra"
)

var claudeCmd = &cobra.Command{
	Use:   "claude [args...]",
	Short: "Run Claude Code in a devcell container",
	Long: `Starts a Claude Code session inside an isolated devcell container.

The current working directory is mounted as /workspace. All additional
args are forwarded to the claude binary unchanged.

Use --ollama to route Claude Code through a local ollama instance
(Anthropic Messages API compatibility). This sets ANTHROPIC_BASE_URL
to point at ollama on the host. Can also be enabled permanently via
provider = "ollama" in the [llm] section of devcell.toml.

Use --openrouter to route Claude Code through OpenRouter. Requires
OPENROUTER_API_KEY env var. Can also be enabled permanently via
provider = "openrouter" in the [llm] section of devcell.toml.
A flag overrides [llm] provider for that run.

The model is resolved in order:
  1. [llm] model in devcell.toml, when [llm] provider is the active one
  2. ollama: best-ranked model from the running instance (auto-detect)
     openrouter: first of [llm.providers.openrouter] models

[llm.providers.<name>] base_url overrides the built-in endpoint.

Examples:

    cell claude
    cell claude --resume
    cell claude --ollama
    cell claude --openrouter`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if wantsHelp(args) {
			return cmd.Help()
		}
		return runAgent("claude", []string{"--dangerously-skip-permissions"}, args, claudeEnv())
	},
}

// claudeEnv returns extra env vars for the claude container.
// When --ollama or [llm] provider = "ollama" is set, it injects env vars
// that redirect Claude Code's API calls to a local ollama instance and
// sets ANTHROPIC_MODEL to the configured or best-available model.
// When --openrouter or [llm] provider = "openrouter" is set, it injects
// env vars that redirect Claude Code's API calls through OpenRouter.
func claudeEnv() map[string]string {
	dbg := scanFlag("--debug")
	useOllama := scanFlag("--ollama")
	useOpenRouter := scanFlag("--openrouter")

	// Always load config — needed for [llm] provider and model selection.
	var llm cfg.LLMSection
	c, err := config.LoadFromOS()
	if err == nil {
		llm = cfg.LoadFromOS(c.ConfigDir, c.BaseDir).LLM
		if !useOllama && !useOpenRouter {
			useOllama = llm.Provider == cfg.LLMProviderOllama
			useOpenRouter = llm.Provider == cfg.LLMProviderOpenRouter
		}
	}

	// Base env vars for all claude sessions.
	env := map[string]string{}

	if useOpenRouter {
		return openrouterEnv(llm, dbg)
	}

	if !useOllama {
		return env
	}

	if dbg {
		fmt.Fprintf(os.Stderr, " claude: ollama mode enabled, redirecting API to host ollama\n")
	}

	env["ANTHROPIC_BASE_URL"] = llmBaseURL(llm, cfg.LLMProviderOllama)
	env["ANTHROPIC_AUTH_TOKEN"] = "ollama"
	env["CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"] = "1"

	if model := resolveOllamaModel(llm.ModelFor(cfg.LLMProviderOllama), dbg); model != "" {
		env["ANTHROPIC_MODEL"] = model
	}

	return env
}

// openrouterEnv returns env vars that redirect Claude Code through OpenRouter.
// The empty OPENROUTER_API_KEY / ANTHROPIC_AUTH_TOKEN placeholders are filled
// after 1Password resolution (see runAgent).
func openrouterEnv(llm cfg.LLMSection, dbg bool) map[string]string {
	baseURL := llmBaseURL(llm, cfg.LLMProviderOpenRouter)
	if dbg {
		fmt.Fprintf(os.Stderr, " claude: openrouter mode enabled, redirecting API to %s\n", baseURL)
	}

	env := map[string]string{
		"ANTHROPIC_BASE_URL":                         baseURL,
		"ANTHROPIC_API_KEY":                          "",
		"ANTHROPIC_AUTH_TOKEN":                       "",
		"OPENROUTER_API_KEY":                         "",
		"CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1",
		"CLAUDE_CODE_SKIP_FAST_MODE_ORG_CHECK":       "1",
	}

	model := resolveOpenRouterModel(llm, dbg)
	if model != "" {
		env["ANTHROPIC_MODEL"] = model
	}

	return env
}

// resolveOpenRouterModel picks the model for OpenRouter mode: [llm] model
// when provider = "openrouter", else the first of [llm.providers.openrouter]
// models, else "" (the agent's own default).
func resolveOpenRouterModel(llm cfg.LLMSection, dbg bool) string {
	if model := llm.ModelFor(cfg.LLMProviderOpenRouter); model != "" {
		if dbg {
			fmt.Fprintf(os.Stderr, " claude: openrouter model from config: %s\n", model)
		}
		return model
	}
	if p := llm.Providers[cfg.LLMProviderOpenRouter]; len(p.Models) > 0 {
		if dbg {
			fmt.Fprintf(os.Stderr, " claude: openrouter model from providers list: %s\n", p.Models[0])
		}
		return p.Models[0]
	}
	return ""
}

// resolveOllamaModel returns the bare ollama model name to use as ANTHROPIC_MODEL.
// Priority: config [llm] model > best-ranked model from running ollama.
// Returns "" if no model can be determined (ollama unreachable, no models).
func resolveOllamaModel(configModel string, dbg bool) string {
	if configModel != "" {
		if dbg {
			fmt.Fprintf(os.Stderr, " claude: model from config: %s\n", configModel)
		}
		return configModel
	}

	// Auto-detect: probe local ollama and pick the best-ranked model.
	if dbg {
		fmt.Fprintf(os.Stderr, " claude: no model in config — auto-selecting from local ollama\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if !ollama.Detect(ctx, ollama.DefaultBaseURL) {
		if dbg {
			fmt.Fprintf(os.Stderr, " claude: ollama not reachable at %s — no model set\n", ollama.DefaultBaseURL)
		}
		return ""
	}
	if dbg {
		fmt.Fprintf(os.Stderr, " claude: ollama reachable at %s\n", ollama.DefaultBaseURL)
	}

	models, err := ollama.FetchModels(ctx, ollama.DefaultBaseURL)
	if err != nil {
		if dbg {
			fmt.Fprintf(os.Stderr, " claude: fetch models failed: %v\n", err)
		}
		return ""
	}
	if dbg {
		fmt.Fprintf(os.Stderr, " claude: %d model(s) available\n", len(models))
	}
	if len(models) == 0 {
		return ""
	}

	// Rank local models with real system RAM so the composite score
	// penalises models that won't fit (same algo as `cell models`).
	systemRAM := ollama.GetSystemRAMGB()
	if dbg {
		fmt.Fprintf(os.Stderr, " claude: system RAM %.0f GB — ranking by composite score (swe×0.6 + speed×0.25) × ram_fit\n", systemRAM)
	}

	ranked := ollama.RankModels(models, 0, nil, nil, systemRAM, "")
	if len(ranked) == 0 {
		return ""
	}

	if dbg {
		fmt.Fprintf(os.Stderr, " claude: %d model(s) ranked (composite score = swe×0.6 + speed×0.25, ×0.1 if RAM tight):\n", len(ranked))
		for _, r := range ranked {
			_, needed := ollama.CheckHardwareSafe(r.ParameterSize, systemRAM)
			ramStr := "ok"
			if needed > 0 && systemRAM > 0 && needed > systemRAM*0.75 {
				ramStr = fmt.Sprintf("tight (%.0fGB needed, %.0fGB avail)", needed, systemRAM)
			} else if needed > 0 {
				ramStr = fmt.Sprintf("%.0fGB", needed)
			}
			fmt.Fprintf(os.Stderr, " claude:   [%d] %-35s  swe=%-5.1f  speed=%-6.0f  score=%.2f  ram=%s\n",
				r.Rank, r.Name, r.SWEScore, r.SpeedTPM, r.RecommendedScore, ramStr)
		}
		top := ranked[0]
		fmt.Fprintf(os.Stderr, " claude: picking %s — highest score (%.2f: swe=%.1f, speed=%.0fT/m)\n",
			top.Name, top.RecommendedScore, top.SWEScore, top.SpeedTPM)
	}

	model := ranked[0].Name
	fmt.Printf(" → ollama model: %s (pin with [llm] provider = \"ollama\" and model in devcell.toml)\n", model)
	return model
}
