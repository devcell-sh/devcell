package main

import "github.com/spf13/cobra"

// `cell build prune` is a deprecated alias for `cell clean build`.
var pruneCmd = &cobra.Command{
	Use:        "prune",
	Short:      "Prune Docker daemon and nix store (use `cell clean build` instead)",
	Deprecated: "use `cell clean build` instead",
	RunE:       runCleanBuild,
}

func init() {
	pruneCmd.Flags().Bool("pure", false, "deprecated: the default now prunes both Docker and nix")
	_ = pruneCmd.Flags().MarkDeprecated("pure", "the default now prunes both Docker and nix")
	pruneCmd.Flags().Bool("force", false, "nuclear cleanup (wipe daemon data dir / nuke linux-builder VM / aggressive nix GC)")
	pruneCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	buildCmd.AddCommand(pruneCmd)
}
