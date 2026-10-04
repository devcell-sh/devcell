package tart

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// TartGetInfo is the JSON output of `tart get <name> --format json`.
// Parallel to Docker: `docker image inspect <tag>`.
type TartGetInfo struct {
	OS         string `json:"OS"`
	CPU        int    `json:"CPU"`
	Memory     uint64 `json:"Memory"`
	Disk       int    `json:"Disk"`
	DiskFormat string `json:"DiskFormat"`
	Size       string `json:"Size"`
	Display    string `json:"Display"`
	Running    bool   `json:"Running"`
	State      string `json:"State"`
}

// TartClone runs `tart clone <ref> <localName>`.
// Parallel to Docker: exec.Command("docker", "pull", tag).
func TartClone(ctx context.Context, ref, localName string) error {
	cmd := exec.CommandContext(ctx, "tart", "clone", ref, localName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TartGet runs `tart get <name> --format json` and returns the parsed info.
// Parallel to Docker: exec.Command("docker", "image", "inspect", tag).
func TartGet(ctx context.Context, name string) (TartGetInfo, error) {
	out, err := exec.CommandContext(ctx, "tart", "get", name, "--format", "json").Output()
	if err != nil {
		return TartGetInfo{}, fmt.Errorf("tart get %s: %w", name, err)
	}
	var info TartGetInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return TartGetInfo{}, fmt.Errorf("parsing tart get output: %w", err)
	}
	return info, nil
}

// TartDelete runs `tart delete <name>`.
// Parallel to Docker: exec.Command("docker", "rm", name).
func TartDelete(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "tart", "delete", name).Run()
}
