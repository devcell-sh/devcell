package tart

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

// DiscoveredVM is a running Tart macOS VM discovered via `tart list`.
type DiscoveredVM struct {
	CellName string
	IP       string
	VNCPort  string // macOS Screen Sharing on port 5900
}

// tartListEntry is one element of `tart list --format json`.
type tartListEntry struct {
	Name  string `json:"Name"`
	State string `json:"State"`
}

// Seams for tests.
var (
	tartListFunc = func(ctx context.Context) ([]byte, error) {
		return exec.CommandContext(ctx, "tart", "list", "--format", "json").Output()
	}
	tartIPFunc = TartIP
)

// DiscoverRunningVMs lists running Tart instances that match the devcell
// naming convention (<cellName>-tart) and resolves each VM's guest IP.
func DiscoverRunningVMs(ctx context.Context) []DiscoveredVM {
	raw, err := tartListFunc(ctx)
	if err != nil {
		return nil
	}
	entries, err := parseTartList(raw)
	if err != nil {
		return nil
	}

	var vms []DiscoveredVM
	for _, e := range entries {
		cellName, ok := cellNameFromInstance(e.Name)
		if !ok {
			continue
		}
		ip, err := tartIPFunc(ctx, e.Name)
		if err != nil || ip == "" {
			continue
		}
		vms = append(vms, DiscoveredVM{
			CellName: cellName,
			IP:       ip,
			VNCPort:  "5900",
		})
	}
	return vms
}

// parseTartList extracts running devcell instance VMs from tart list JSON.
func parseTartList(raw []byte) ([]tartListEntry, error) {
	var all []tartListEntry
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	var running []tartListEntry
	for _, e := range all {
		if e.State != "running" {
			continue
		}
		if _, ok := cellNameFromInstance(e.Name); !ok {
			continue
		}
		running = append(running, e)
	}
	return running, nil
}

// cellNameFromInstance extracts the cell name from a Tart instance VM name.
// Instance VMs are named "<cellName>-tart" (see InstanceVMName).
// Template VMs (devcell-tart-*) are excluded.
func cellNameFromInstance(vmName string) (string, bool) {
	if !strings.HasSuffix(vmName, "-tart") {
		return "", false
	}
	cell := strings.TrimSuffix(vmName, "-tart")
	if cell == "" {
		return "", false
	}
	// Exclude template VMs: devcell-tart-base, devcell-tart-llm, etc.
	if strings.HasPrefix(vmName, "devcell-tart-") {
		return "", false
	}
	return cell, true
}
