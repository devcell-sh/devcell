package docker

import (
	"testing"
)

func TestParsePSJSON(t *testing.T) {
	input := `{"Command":"/tmp/devcell-boot/entrypoint.sh sleep infinity","CreatedAt":"2026-11-02 10:00:00 +0000 UTC","ID":"abc123","Image":"ghcr.io/devcell-sh/devcell:v0.10.0","Labels":"devcell.basedir=/Users/dmitry/dev/nopk-backend,devcell.cellid=87,devcell.workspace=main,devcell.lifecycle=oneshot,devcell.stack=ultimate","LocalVolumes":"0","Mounts":"","Names":"devcell-nopk-backend-87","Networks":"bridge","Ports":"0.0.0.0:8789->3389/tcp, 0.0.0.0:8750->5900/tcp","RunningFor":"8 hours ago","Size":"0B","State":"running","Status":"Up 8 hours"}
{"Command":"/tmp/devcell-boot/entrypoint.sh sleep infinity","CreatedAt":"2026-11-02 08:00:00 +0000 UTC","ID":"def456","Image":"ghcr.io/devcell-sh/devcell:v0.10.0","Labels":"devcell.basedir=/Users/dmitry/dev/mad,devcell.cellid=76,devcell.lifecycle=oneshot,devcell.stack=ultimate","LocalVolumes":"0","Mounts":"","Names":"devcell-mad-76","Networks":"bridge","Ports":"0.0.0.0:7689->3389/tcp","RunningFor":"26 hours ago","Size":"0B","State":"running","Status":"Up 26 hours"}`

	cells, err := parsePSJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 {
		t.Fatalf("want 2 cells, got %d", len(cells))
	}

	c := cells[0]
	if c.Name != "devcell-nopk-backend-87" {
		t.Errorf("name: got %q", c.Name)
	}
	if c.Status != "running" {
		t.Errorf("status: got %q", c.Status)
	}
	if c.Project != "/Users/dmitry/dev/nopk-backend" {
		t.Errorf("project: got %q", c.Project)
	}
	if c.Workspace != "main" {
		t.Errorf("workspace: got %q", c.Workspace)
	}
	if c.CellID != "87" {
		t.Errorf("cellid: got %q", c.CellID)
	}
	if c.Lifecycle != "oneshot" {
		t.Errorf("lifecycle: got %q", c.Lifecycle)
	}
	if c.Uptime != "8 hours ago" {
		t.Errorf("uptime: got %q", c.Uptime)
	}

	c2 := cells[1]
	if c2.Workspace != "" {
		t.Errorf("c2 workspace should be empty for legacy container, got %q", c2.Workspace)
	}
}

func TestParsePSJSON_Empty(t *testing.T) {
	cells, err := parsePSJSON("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 0 {
		t.Fatalf("want 0 cells, got %d", len(cells))
	}
}

func TestParseLabels(t *testing.T) {
	s := "devcell.basedir=/Users/dmitry/dev/foo,devcell.cellid=42,devcell.workspace=main"
	m := parseLabels(s)
	if m["devcell.basedir"] != "/Users/dmitry/dev/foo" {
		t.Errorf("basedir: %q", m["devcell.basedir"])
	}
	if m["devcell.cellid"] != "42" {
		t.Errorf("cellid: %q", m["devcell.cellid"])
	}
	if m["devcell.workspace"] != "main" {
		t.Errorf("workspace: %q", m["devcell.workspace"])
	}
}

func TestFilterByWorkspace(t *testing.T) {
	cells := []CellInfo{
		{Name: "a", Workspace: "main"},
		{Name: "b", Workspace: "dev"},
		{Name: "c", Workspace: ""},
	}

	got := FilterByWorkspace(cells, "main", false)
	if len(got) != 1 || got[0].Name != "a" {
		t.Errorf("without legacy: want [a], got %v", got)
	}

	got = FilterByWorkspace(cells, "main", true)
	if len(got) != 2 {
		t.Errorf("with legacy: want 2, got %d", len(got))
	}
}

func TestParseStatsJSON(t *testing.T) {
	input := `{"BlockIO":"0B / 0B","CPUPerc":"145.23%","Container":"abc123","ID":"abc123","MemPerc":"12.40%","MemUsage":"2.1GiB / 16GiB","Name":"devcell-nopk-backend-87","NetIO":"1.2MB / 500kB","PIDs":"42"}
{"BlockIO":"0B / 0B","CPUPerc":"3.10%","Container":"def456","ID":"def456","MemPerc":"5.20%","MemUsage":"890MiB / 16GiB","Name":"devcell-devcell-133","NetIO":"100kB / 50kB","PIDs":"12"}`

	stats, err := parseStatsJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("want 2 stats, got %d", len(stats))
	}
	if stats[0].CPUPercent != 145.23 {
		t.Errorf("cpu: got %f", stats[0].CPUPercent)
	}
	if stats[0].MemPercent != 12.40 {
		t.Errorf("mem%%: got %f", stats[0].MemPercent)
	}
	if stats[1].Name != "devcell-devcell-133" {
		t.Errorf("name: got %q", stats[1].Name)
	}
}

func TestParsePercent(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"145.23%", 145.23},
		{"0.00%", 0.0},
		{"3.10%", 3.10},
		{"", 0.0},
	}
	for _, tt := range tests {
		got := parsePercent(tt.in)
		if got != tt.want {
			t.Errorf("parsePercent(%q) = %f, want %f", tt.in, got, tt.want)
		}
	}
}

func TestSortStatsByCPU(t *testing.T) {
	stats := []CellStats{
		{Name: "low", CPUPercent: 1.0},
		{Name: "high", CPUPercent: 100.0},
		{Name: "mid", CPUPercent: 50.0},
	}
	SortStatsByCPU(stats)
	if stats[0].Name != "high" || stats[1].Name != "mid" || stats[2].Name != "low" {
		t.Errorf("sort order: %v", stats)
	}
}

func TestShortenPath(t *testing.T) {
	if got := ShortenPath("/Users/dmitry/dev/foo", "/Users/dmitry"); got != "~/dev/foo" {
		t.Errorf("got %q", got)
	}
	if got := ShortenPath("/opt/other", "/Users/dmitry"); got != "/opt/other" {
		t.Errorf("got %q", got)
	}
	if got := ShortenPath("/foo", ""); got != "/foo" {
		t.Errorf("got %q", got)
	}
}
