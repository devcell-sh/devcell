package main

import (
	"fmt"
	"os"

	"github.com/devcell-sh/devcell/internal/peer"
)

func main() {
	source := flag("--source")
	projectDir := flag("--project-dir")

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve home: %v\n", err)
		os.Exit(1)
	}

	if projectDir == "" {
		projectDir = os.Getenv("DEVCELL_PROJECT_DIR")
	}

	var reader peer.MemoryReader
	switch source {
	case peer.SourceClaude:
		if projectDir == "" {
			fmt.Fprintln(os.Stderr, "--project-dir or DEVCELL_PROJECT_DIR required for --source=claude")
			os.Exit(1)
		}
		reader = &peer.ClaudeReader{MemoryDir: peer.ClaudeMemoryDir(home, projectDir), ProjectDir: projectDir}
	case peer.SourceCodex:
		reader = &peer.CodexReader{DBPath: peer.CodexMemoryDB(home), StateDB: peer.CodexStateDB(home), ProjectDir: projectDir}
	case "", "all":
		var readers []peer.MemoryReader
		readers = append(readers, &peer.CodexReader{DBPath: peer.CodexMemoryDB(home), StateDB: peer.CodexStateDB(home), ProjectDir: projectDir})
		if projectDir != "" {
			readers = append(readers, &peer.ClaudeReader{MemoryDir: peer.ClaudeMemoryDir(home, projectDir), ProjectDir: projectDir})
		}
		reader = &peer.MultiReader{Readers: readers}
		source = "all"
	default:
		fmt.Fprintf(os.Stderr, "--source must be 'claude', 'codex', or 'all', got %q\n", source)
		os.Exit(1)
	}

	srv := &peer.Server{
		Reader: reader,
		Source: source,
		In:     os.Stdin,
		Out:    os.Stdout,
	}
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

func flag(name string) string {
	for i, arg := range os.Args[1:] {
		if arg == name && i+1 < len(os.Args)-1 {
			return os.Args[i+2]
		}
		if len(arg) > len(name) && arg[:len(name)+1] == name+"=" {
			return arg[len(name)+1:]
		}
	}
	return ""
}
