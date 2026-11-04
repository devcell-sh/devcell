package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// CellInfo holds metadata for a single running devcell container.
type CellInfo struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Image     string `json:"image"`
	Project   string `json:"project"`
	Workspace string `json:"workspace"`
	CellID    string `json:"cell_id"`
	Lifecycle string `json:"lifecycle"`
	Stack     string `json:"stack"`
	Ports     string `json:"ports"`
}

// psJSON matches docker ps --format json output.
type psJSON struct {
	Names      string `json:"Names"`
	Status     string `json:"Status"`
	RunningFor string `json:"RunningFor"`
	Image      string `json:"Image"`
	Labels     string `json:"Labels"`
	Ports      string `json:"Ports"`
	State      string `json:"State"`
}

// ListCells returns running devcell containers. When includeAll is true,
// stopped containers are included (docker ps -a).
func ListCells(ctx context.Context, includeAll bool) ([]CellInfo, error) {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"ps", "--filter", "label=devcell.basedir", "--format", "json", "--no-trunc"}
	if includeAll {
		args = append(args, "-a")
	}
	out, err := exec.CommandContext(tctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return parsePSJSON(string(out))
}

// parsePSJSON parses docker ps --format json output (one JSON object per line).
func parsePSJSON(raw string) ([]CellInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var cells []CellInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var p psJSON
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			continue
		}
		labels := parseLabels(p.Labels)
		cells = append(cells, CellInfo{
			Name:      p.Names,
			Status:    p.State,
			Uptime:    p.RunningFor,
			Image:     p.Image,
			Project:   labels["devcell.basedir"],
			Workspace: labels["devcell.workspace"],
			CellID:    labels["devcell.cellid"],
			Lifecycle: labels["devcell.lifecycle"],
			Stack:     labels["devcell.stack"],
			Ports:     p.Ports,
		})
	}
	return cells, nil
}

// parseLabels splits a comma-separated "key=value,key=value" label string.
func parseLabels(s string) map[string]string {
	m := make(map[string]string)
	for _, kv := range strings.Split(s, ",") {
		kv = strings.TrimSpace(kv)
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			m[kv[:eq]] = kv[eq+1:]
		}
	}
	return m
}

// FilterByWorkspace returns only cells whose workspace label matches ws.
// Cells without a workspace label are included when includeLegacy is true.
func FilterByWorkspace(cells []CellInfo, ws string, includeLegacy bool) []CellInfo {
	var out []CellInfo
	for _, c := range cells {
		if c.Workspace == ws {
			out = append(out, c)
		} else if c.Workspace == "" && includeLegacy {
			out = append(out, c)
		}
	}
	return out
}

// CellStats holds resource usage for a single container.
type CellStats struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsage   string  `json:"mem_usage"`
	MemPercent float64 `json:"mem_percent"`
	NetIO      string  `json:"net_io"`
	BlockIO    string  `json:"block_io"`
}

// statsJSON matches docker stats --no-stream --format json output.
type statsJSON struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
}

// ListCellStats returns resource stats for the given container names.
// Pass nil to get stats for all running containers (caller filters).
func ListCellStats(ctx context.Context, names []string) ([]CellStats, error) {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"stats", "--no-stream", "--format", "json"}
	args = append(args, names...)
	out, err := exec.CommandContext(tctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker stats: %w", err)
	}
	return parseStatsJSON(string(out))
}

// parseStatsJSON parses docker stats --no-stream --format json output.
func parseStatsJSON(raw string) ([]CellStats, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var stats []CellStats
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s statsJSON
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		stats = append(stats, CellStats{
			Name:       s.Name,
			CPUPercent: parsePercent(s.CPUPerc),
			MemUsage:   s.MemUsage,
			MemPercent: parsePercent(s.MemPerc),
			NetIO:      s.NetIO,
			BlockIO:    s.BlockIO,
		})
	}
	return stats, nil
}

// parsePercent strips "%" and parses to float64.
func parsePercent(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// SortStatsByCPU sorts stats by CPU usage descending.
func SortStatsByCPU(stats []CellStats) {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].CPUPercent > stats[j].CPUPercent
	})
}

// ShortenPath replaces a home directory prefix with "~".
// Accepts multiple home candidates (e.g., container home and host home).
func ShortenPath(path string, homes ...string) string {
	for _, home := range homes {
		if home != "" && strings.HasPrefix(path, home+"/") {
			return "~" + path[len(home):]
		}
	}
	return path
}
