package main

import (
	"os"
	"strconv"

	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/spf13/cobra"
)

// bootUICmd renders the boot panel inside the cage's tmux popup
// (CELL-657). devcell starts it itself; it is not for interactive use.
var bootUICmd = &cobra.Command{
	Use:    "boot-ui <socket>",
	Short:  "Render boot progress from a devcell event socket (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, _ := strconv.Atoi(os.Getenv("DEVCELL_BOOT_PID"))
		code := cage.RunBootUI(cmd.Context(), cage.BootUIOptions{
			Socket:     args[0],
			ForwardPID: pid,
			Stdin:      os.Stdin,
		})
		if code != cage.BootUIOK {
			exitProcess(code)
		}
		return nil
	},
}
