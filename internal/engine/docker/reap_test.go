package docker

import (
	"testing"
)

func TestParseDockerTopComm(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "typical output",
			raw:  "COMMAND\nsleep\ns6-svscan\ns6-supervise\n",
			want: []string{"sleep", "s6-svscan", "s6-supervise"},
		},
		{
			name: "with agent running",
			raw:  "COMMAND\nsleep\ns6-svscan\nclaude\nnode\n",
			want: []string{"sleep", "s6-svscan", "claude", "node"},
		},
		{
			name: "empty",
			raw:  "",
			want: nil,
		},
		{
			name: "header only",
			raw:  "COMMAND\n",
			want: nil,
		},
		{
			name: "whitespace lines",
			raw:  "COMMAND\n  sleep  \n\n  bash\n",
			want: []string{"sleep", "bash"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDockerTopComm(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestIsStaleOneshot(t *testing.T) {
	tests := []struct {
		name      string
		lifecycle string
		procs     []string
		want      bool
	}{
		{
			name:      "only infra processes",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "s6-svscan", "s6-supervise", "devcell-event"},
			want:      true,
		},
		{
			name:      "claude running",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "s6-svscan", "claude"},
			want:      false,
		},
		{
			name:      "codex running",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "codex"},
			want:      false,
		},
		{
			name:      "zsh shell session",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "s6-svscan", "zsh"},
			want:      false,
		},
		{
			name:      "bash shell session",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "bash"},
			want:      false,
		},
		{
			name:      "persistent is never stale",
			lifecycle: "persistent",
			procs:     []string{"sleep"},
			want:      false,
		},
		{
			name:      "empty lifecycle treated as oneshot",
			lifecycle: "",
			procs:     []string{"sleep"},
			want:      false,
		},
		{
			name:      "empty processes is safe (not stale)",
			lifecycle: "oneshot",
			procs:     nil,
			want:      false,
		},
		{
			name:      "node MCP server",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "node"},
			want:      false,
		},
		{
			name:      "opencode running",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "opencode"},
			want:      false,
		},
		{
			name:      "gemini running",
			lifecycle: "oneshot",
			procs:     []string{"sleep", "gemini"},
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsStaleOneshot(tc.lifecycle, tc.procs)
			if got != tc.want {
				t.Errorf("IsStaleOneshot(%q, %v) = %v, want %v", tc.lifecycle, tc.procs, got, tc.want)
			}
		})
	}
}

func TestFindStaleCells(t *testing.T) {
	cells := []CellInfo{
		{Name: "devcell-app-1", Lifecycle: "oneshot", Project: "/home/app", Workspace: "main", Uptime: "2 hours"},
		{Name: "devcell-web-2", Lifecycle: "oneshot", Project: "/home/web", Workspace: "main", Uptime: "30 min"},
		{Name: "devcell-db-3", Lifecycle: "persistent", Project: "/home/db", Workspace: "main", Uptime: "3 days"},
		{Name: "devcell-dev-0", Lifecycle: "oneshot", Project: "/home/dev", Workspace: "dimm", Uptime: "1 hour"},
	}
	topResults := map[string][]string{
		"devcell-app-1": {"sleep", "s6-svscan"},
		"devcell-web-2": {"sleep", "s6-svscan", "claude"},
		"devcell-db-3":  {"sleep", "zsh"},
		"devcell-dev-0": {"sleep"},
	}

	ps := func() ([]CellInfo, error) { return cells, nil }
	top := func(name string) ([]string, error) { return topResults[name], nil }

	stale, err := findStaleCellsWith("devcell-skip-99", ps, top)
	if err != nil {
		t.Fatalf("FindStaleCells: %v", err)
	}

	// devcell-app-1: oneshot + only infra = stale
	// devcell-web-2: oneshot + claude running = not stale
	// devcell-db-3: persistent = skipped
	// devcell-dev-0: oneshot + only sleep = stale
	if len(stale) != 2 {
		t.Fatalf("got %d stale, want 2: %+v", len(stale), stale)
	}

	names := map[string]bool{}
	for _, s := range stale {
		names[s.Name] = true
	}
	if !names["devcell-app-1"] {
		t.Error("expected devcell-app-1 to be stale")
	}
	if !names["devcell-dev-0"] {
		t.Error("expected devcell-dev-0 to be stale")
	}
}

func TestFindStaleCells_SkipsSelf(t *testing.T) {
	cells := []CellInfo{
		{Name: "devcell-app-1", Lifecycle: "oneshot", Project: "/home/app", Workspace: "main", Uptime: "2 hours"},
	}
	topResults := map[string][]string{
		"devcell-app-1": {"sleep"},
	}

	ps := func() ([]CellInfo, error) { return cells, nil }
	top := func(name string) ([]string, error) { return topResults[name], nil }

	stale, err := findStaleCellsWith("devcell-app-1", ps, top)
	if err != nil {
		t.Fatalf("FindStaleCells: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("should skip self, got %d stale: %+v", len(stale), stale)
	}
}
