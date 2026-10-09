package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/engine/docker"
	"github.com/DimmKirr/devcell/internal/engine/winkit"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running devcell container or VM",
	Long: `Stops the devcell container or VM started by 'cell start'.
Docker containers are automatically removed after stopping.
Windows VMs are shut down gracefully via SSH, then SIGTERM.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		applyOutputFlags()
		c, err := config.LoadFromOS()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
		engineName, engineErr := resolveEngine(os.Stderr, cellCfg)
		if engineErr != nil {
			return engineErr
		}

		ctx := context.Background()

		switch engineName {
		case engine.Winkit:
			return winkit.StopVM(ctx, c.HostHome, c.CellName)
		default:
			if !docker.ContainerRunning(ctx, c.ContainerName) {
				fmt.Printf("No running container %s found\n", c.ContainerName)
				return nil
			}
			if err := exec.CommandContext(ctx, "docker", "stop", c.ContainerName).Run(); err != nil {
				return fmt.Errorf("stop container %s: %w", c.ContainerName, err)
			}
			fmt.Printf("Container %s stopped\n", c.ContainerName)
			return nil
		}
	},
}
