package cell

import (
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
)

// CodexPromptFlags builds the overlay prompt (container context + TOML append
// prompt) and returns it as inline argv for Codex's -c developer_instructions.
//
// Unlike ClaudePromptFlags, which writes files and passes --system-prompt-file
// / --append-system-prompt-file, Codex has no file-based flag — the content
// travels as a TOML config value on the command line.
func CodexPromptFlags(c config.Config, cellCfg cfg.CellConfig, opts ResolveOpts) ([]string, error) {
	content, err := AssembleOverlayPrompt(c, cellCfg, opts)
	if err != nil {
		return nil, err
	}
	escaped := strings.ReplaceAll(content, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return []string{"-c", "developer_instructions=" + escaped}, nil
}
