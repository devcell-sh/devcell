package main

import (
	"fmt"
	"os"

	"github.com/devcell-sh/devcell/internal/peer"
	"github.com/spf13/cobra"
)

var peerMemoryCmd = &cobra.Command{
	Use:    "peer-memory",
	Short:  "MCP server: expose peer agent memories (internal, stdio)",
	Hidden: true,
	RunE:   runPeerMemory,
}

func init() {
	peerMemoryCmd.Flags().String("source", "", "which agent's memories to read (claude or codex)")
	peerMemoryCmd.Flags().String("project-dir", "", "project directory (for deriving Claude's memory path)")
	peerMemoryCmd.MarkFlagRequired("source")
	rootCmd.AddCommand(peerMemoryCmd)
}

func runPeerMemory(cmd *cobra.Command, _ []string) error {
	source, _ := cmd.Flags().GetString("source")
	projectDir, _ := cmd.Flags().GetString("project-dir")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	var reader peer.MemoryReader
	switch source {
	case peer.SourceClaude:
		if projectDir == "" {
			projectDir = os.Getenv("DEVCELL_PROJECT_DIR")
		}
		if projectDir == "" {
			return fmt.Errorf("--project-dir or DEVCELL_PROJECT_DIR required for --source=claude")
		}
		reader = &peer.ClaudeReader{MemoryDir: peer.ClaudeMemoryDir(home, projectDir)}
	case peer.SourceCodex:
		reader = &peer.CodexReader{DBPath: peer.CodexMemoryDB(home)}
	default:
		return fmt.Errorf("--source must be 'claude' or 'codex', got %q", source)
	}

	srv := &peer.Server{
		Reader: reader,
		Source: source,
		In:     os.Stdin,
		Out:    os.Stdout,
	}
	return srv.Run()
}
