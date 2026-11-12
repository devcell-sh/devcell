package main

import "github.com/spf13/cobra"

// `cell clean nix` is an alias for `cell clean build`: both prune the
// Docker daemon and nix store together because nix GC must know which
// containers are alive.
var cleanNixCmd = &cobra.Command{
	Use:   "nix",
	Short: "Alias for 'cell clean build' (prune Docker daemon and nix store)",
	RunE:  runCleanBuild,
}

func init() {
	cleanNixCmd.Flags().Bool("force", false, "nuclear cleanup (wipe daemon data dir / nuke linux-builder VM / aggressive nix GC)")
	cleanNixCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cleanCmd.AddCommand(cleanNixCmd)
}
