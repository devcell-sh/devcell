package main

import (
	"github.com/spf13/cobra"
)

var geminiCmd = &cobra.Command{
	Use:   "gemini [args...]",
	Short: "Run Gemini CLI in a devcell container",
	Long: `Starts a Google Gemini CLI session inside an isolated devcell container.

The current working directory is mounted as /workspace. All additional
args are forwarded to the gemini binary unchanged.

Auth: gemini-cli reads GEMINI_API_KEY from the environment, or prompts
for Google OAuth on first run. OAuth state persists in ~/.gemini/ via
the home bind mount, surviving container restarts.

No --ollama mode: gemini-cli does not currently expose an
OpenAI-compatible base URL override.

Examples:

    cell gemini
    cell gemini -p "summarize this repo"
    cell gemini --model=gemini-2.5-pro`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgent("gemini", []string{"--yolo"}, args, nil)
	},
}
