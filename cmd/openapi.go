package main

import (
	"fmt"
	"os"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/spf13/cobra"
)

var openapiCmd = &cobra.Command{
	Use:   "openapi",
	Short: "Print the JSON Schema for .devcell.toml",
	Long:  "Prints the JSON Schema (draft 2020-12) describing the devcell.toml config format. The schema is embedded at build time and matches the running binary's config surface.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprint(os.Stdout, string(cfg.EmbeddedSchema))
		return err
	},
}
