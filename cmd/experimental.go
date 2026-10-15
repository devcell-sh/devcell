package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/experiments"
	"github.com/spf13/cobra"
)

// experimentalCmd lists the registered experimental features and which are
// on here, from --experimental, DEVCELL_EXPERIMENTAL and [cell] experimental.
var experimentalCmd = &cobra.Command{
	Use:   "experimental",
	Short: "List experimental features and which are on",
	Long: `Lists every experimental feature with its state and whether it is on in
this directory. A feature is on when named in --experimental=a,b, in
DEVCELL_EXPERIMENTAL or in [cell] experimental; the three are unioned.

See docs/experimental.md for the lifecycle and how features are added.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var cell cfg.CellSection
		if c, err := config.LoadFromOS(); err == nil { // outside a project: nothing from TOML
			cell = cfg.LoadFromOS(c.ConfigDir, c.BaseDir).Cell
		}
		set, err := experimental(cell)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATE\tON\tSUMMARY")
		for _, x := range experiments.List() {
			on := "off"
			if set.Enabled(x) {
				on = "on"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", x.Name, x.State, on, x.Summary)
		}
		return w.Flush()
	},
}
