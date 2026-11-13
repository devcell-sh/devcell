package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/scan"
)

func TestNewDoctorModel_StartsScanning(t *testing.T) {
	m := newDoctorModel()
	if !m.scanning {
		t.Error("expected scanning=true on init")
	}
	if m.active != screenOverview {
		t.Errorf("expected screenOverview, got %d", m.active)
	}
}

func TestDoctorModel_ScanResultStopsSpinner(t *testing.T) {
	m := newDoctorModel()
	msg := scanResultMsg{scannedAt: time.Now()}
	updated, _ := m.Update(msg)
	dm := updated.(doctorModel)
	if dm.scanning {
		t.Error("scanning should be false after scanResultMsg")
	}
	if dm.scanData == nil {
		t.Error("scanData should be set after scanResultMsg")
	}
}

func TestDoctorModel_TabSwitching(t *testing.T) {
	m := newDoctorModel()
	m.scanning = false
	m.width = 80
	m.height = 24

	cases := []struct {
		key    string
		expect screen
	}{
		{"d", screenDisk},
		{"o", screenOverview},
		{"2", screenDisk},
		{"3", screenCells},
		{"4", screenHealth},
		{"1", screenOverview},
	}
	for _, tc := range cases {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		m = updated.(doctorModel)
		if m.active != tc.expect {
			t.Errorf("key %q: got screen %d, want %d", tc.key, m.active, tc.expect)
		}
	}
}

func TestDoctorModel_QuitCmd(t *testing.T) {
	m := newDoctorModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestDoctorModel_RefreshRestarsScan(t *testing.T) {
	m := newDoctorModel()
	m.scanning = false
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	dm := updated.(doctorModel)
	if !dm.scanning {
		t.Error("R should restart scanning")
	}
	if cmd == nil {
		t.Error("R should return a command")
	}
}

func TestDoctorModel_RefreshNopWhileScanning(t *testing.T) {
	m := newDoctorModel()
	m.scanning = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	if cmd != nil {
		t.Error("R while scanning should be a no-op")
	}
}

func TestDoctorModel_ViewShowsSpinnerWhileScanning(t *testing.T) {
	m := newDoctorModel()
	m.width = 80
	m.height = 24
	out := m.View()
	if !strings.Contains(out, "Scanning") {
		t.Errorf("expected 'Scanning' in view while scanning, got: %s", out)
	}
}

func TestDoctorModel_ViewEmptyOnZeroWidth(t *testing.T) {
	m := newDoctorModel()
	if m.View() != "" {
		t.Error("expected empty view when width=0")
	}
}

func TestOverviewModel_NilData(t *testing.T) {
	m := overviewModel{}
	out := m.View()
	if !strings.Contains(out, "No scan data") {
		t.Errorf("expected 'No scan data' placeholder, got: %s", out)
	}
}

func TestOverviewModel_RendersDiskUsage(t *testing.T) {
	data := &scanResultMsg{
		build: scan.BuildResult{
			DiskUsage: docker.DiskUsage{
				Volumes:    docker.DiskUsageRow{Size: "11.5GB", Total: 3, Active: 1},
				Images:     docker.DiskUsageRow{Size: "8.2GB", Total: 5, Active: 2},
				Containers: docker.DiskUsageRow{Size: "200MB", Total: 4, Active: 1},
				BuildCache: docker.DiskUsageRow{Size: "1.1GB", Total: 10, Active: 3},
			},
			NixOK: true,
			NixReport: docker.NixStoreReport{
				VolumeBytes: 5_000_000_000,
				Metas: []docker.NixRootMeta{
					{Hash: "abc", Cell: "test"},
				},
			},
		},
		nix: scan.NixResult{
			NixOK: true,
			NixReport: docker.NixStoreReport{
				VolumeBytes: 5_000_000_000,
				Metas: []docker.NixRootMeta{
					{Hash: "abc", Cell: "test"},
				},
			},
		},
		cache: scan.CacheResult{
			TotalBytes: 1_000_000,
		},
		scannedAt: time.Date(2026, 11, 13, 10, 30, 0, 0, time.UTC),
	}

	m := overviewModel{data: data, width: 80, height: 40}
	out := m.View()

	for _, want := range []string{
		"Disk Usage",
		"11.5GB",
		"8.2GB",
		"200MB",
		"1.1GB",
		"Nix store",
		"Nix profiles",
		"Host caches",
		"nix store ok",
		"no stale containers",
		"10:30:00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q in output:\n%s", want, out)
		}
	}
}

func TestOverviewModel_RendersRunningCells(t *testing.T) {
	data := &scanResultMsg{
		build: scan.BuildResult{
			DiskUsage: docker.DiskUsage{
				RunningCells: []docker.RunningCell{
					{Name: "cell-myproject-10", Cell: "myproject-10", Size: "50MB", Uptime: "Up 2 hours"},
				},
			},
		},
		nix:       scan.NixResult{},
		scannedAt: time.Now(),
	}

	m := overviewModel{data: data, width: 80, height: 40}
	out := m.View()

	if !strings.Contains(out, "cell-myproject-10") {
		t.Errorf("expected running cell name in output:\n%s", out)
	}
	if !strings.Contains(out, "Up 2 hours") {
		t.Errorf("expected uptime in output:\n%s", out)
	}
}

func TestOverviewModel_ShowsStaleCellWarning(t *testing.T) {
	data := &scanResultMsg{
		build: scan.BuildResult{
			Stale: []docker.StaleCellInfo{
				{Name: "old-cell"},
			},
		},
		scannedAt: time.Now(),
	}

	m := overviewModel{data: data, width: 80, height: 40}
	out := m.View()

	if !strings.Contains(out, "1 stale container") {
		t.Errorf("expected stale container warning in output:\n%s", out)
	}
}

func TestRenderTabs_HighlightsActive(t *testing.T) {
	out := renderTabs(screenDisk)
	if !strings.Contains(out, "Disk") {
		t.Error("expected Disk tab in output")
	}
	if !strings.Contains(out, "Overview") {
		t.Error("expected Overview tab in output")
	}
}

func TestJoinWith(t *testing.T) {
	got := joinWith([]string{"a", "b", "c"}, " | ")
	if len(got) != 5 {
		t.Errorf("joinWith produced %d elements, want 5", len(got))
	}
	if got[1] != " | " {
		t.Errorf("separator = %q, want ' | '", got[1])
	}
}

func TestJoinWith_Empty(t *testing.T) {
	got := joinWith(nil, ",")
	if got != nil {
		t.Errorf("joinWith(nil) = %v, want nil", got)
	}
}

// --- Bar tests ---

func TestRenderBar_Full(t *testing.T) {
	out := renderBar(1.0, 10, "#E85D26")
	if len(out) == 0 {
		t.Error("expected non-empty bar for fraction=1.0")
	}
}

func TestRenderBar_Empty(t *testing.T) {
	out := renderBar(0.0, 10, "#E85D26")
	if len(out) == 0 {
		t.Error("expected non-empty bar (empty blocks) for fraction=0.0")
	}
}

func TestRenderBar_ZeroWidth(t *testing.T) {
	out := renderBar(0.5, 0, "#E85D26")
	if out != "" {
		t.Errorf("expected empty string for width=0, got %q", out)
	}
}

func TestRenderBar_NegativeFraction(t *testing.T) {
	out := renderBar(-0.5, 10, "#E85D26")
	if len(out) == 0 {
		t.Error("expected non-empty bar for negative fraction (clamped to 0)")
	}
}

// --- Disk screen tests ---

func TestDiskModel_NilData(t *testing.T) {
	m := diskModel{}
	out := m.View()
	if !strings.Contains(out, "No scan data") {
		t.Errorf("expected 'No scan data' placeholder, got: %s", out)
	}
}

func testDiskData() *scanResultMsg {
	return &scanResultMsg{
		build: scan.BuildResult{
			DiskUsage: docker.DiskUsage{
				Volumes:    docker.DiskUsageRow{Size: "17.2GB", Total: 5, Active: 2},
				Images:     docker.DiskUsageRow{Size: "3.7GB", Total: 3, Active: 1},
				Containers: docker.DiskUsageRow{Size: "627MB", Total: 2, Active: 1},
				BuildCache: docker.DiskUsageRow{Size: "0B", Total: 0, Active: 0},
			},
		},
		docker: scan.DockerResult{
			VMDisk:   docker.VMDiskInfo{TotalBytes: 120_000_000_000, UsedBytes: 30_000_000_000, AvailBytes: 90_000_000_000},
			VMDiskOK: true,
		},
		nix: scan.NixResult{
			NixOK: true,
			NixReport: docker.NixStoreReport{
				VolumeBytes: 16_000_000_000,
				TopPaths: []docker.NixStorePath{
					{Name: "clang-21.1.7-lib", Bytes: 843_000_000},
					{Name: "chromium-unwrapped", Bytes: 673_000_000},
				},
				Metas: []docker.NixRootMeta{
					{Hash: "abc12345", Cell: "devcell", Stack: "default", ClosureBytes: 11_700_000_000},
					{Hash: "def67890", Cell: "fm1", Stack: "default", ClosureBytes: 11_200_000_000},
				},
			},
		},
		cache:     scan.CacheResult{TotalBytes: 607_000_000},
		scannedAt: time.Now(),
	}
}

func TestDiskModel_Rebuild(t *testing.T) {
	data := testDiskData()
	m := diskModel{data: data, width: 80, height: 40}
	m.rebuild()
	if len(m.rows) == 0 {
		t.Fatal("expected rows after rebuild")
	}
	if len(m.allRows) == 0 {
		t.Fatal("expected allRows after rebuild")
	}
}

func TestDiskModel_RendersVMBar(t *testing.T) {
	data := testDiskData()
	m := diskModel{data: data, width: 80, height: 40}
	m.rebuild()
	out := m.View()
	if !strings.Contains(out, "Docker VM") {
		t.Errorf("expected 'Docker VM' in disk view:\n%s", out)
	}
}

func TestDiskModel_RendersCategories(t *testing.T) {
	data := testDiskData()
	m := diskModel{data: data, width: 80, height: 40}
	m.rebuild()
	out := m.View()
	for _, want := range []string{"Volumes", "Images", "Containers", "Build cache", "Caches (host)", "Free"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in disk view:\n%s", want, out)
		}
	}
}

func TestDiskModel_Navigate(t *testing.T) {
	data := testDiskData()
	m := diskModel{data: data, width: 80, height: 40}
	m.rebuild()
	initial := m.cursor
	m = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != initial+1 {
		t.Errorf("j should move cursor down: got %d, want %d", m.cursor, initial+1)
	}
	m = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != initial {
		t.Errorf("k should move cursor up: got %d, want %d", m.cursor, initial)
	}
}

func TestDiskModel_ExpandCollapse(t *testing.T) {
	data := testDiskData()
	m := diskModel{data: data, width: 80, height: 40}
	m.rebuild()
	rowsBefore := len(m.rows)

	// First row (Volumes) has children (nix store)
	m.cursor = 0
	m = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if len(m.rows) <= rowsBefore {
		t.Error("expand should increase visible rows")
	}

	// Collapse
	m.cursor = 0
	m = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if len(m.rows) != rowsBefore {
		t.Errorf("collapse should restore row count: got %d, want %d", len(m.rows), rowsBefore)
	}
}

func TestDiskModel_HandlesKey(t *testing.T) {
	m := diskModel{}
	if !m.handlesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) {
		t.Error("j should be handled")
	}
	if m.handlesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}) {
		t.Error("q should not be handled by disk")
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"11.5GB", 11_500_000_000},
		{"200MB", 200_000_000},
		{"0B", 0},
		{"", 0},
		{"1.1kB", 1100},
	}
	for _, tc := range cases {
		got := parseSize(tc.in)
		if got != tc.want {
			t.Errorf("parseSize(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// --- Cells screen tests ---

func TestCellsModel_NilData(t *testing.T) {
	m := cellsModel{}
	out := m.View()
	if !strings.Contains(out, "No scan data") {
		t.Errorf("expected placeholder, got: %s", out)
	}
}

func TestCellsModel_NoCells(t *testing.T) {
	m := cellsModel{data: &scanResultMsg{
		cells:     scan.CellsResult{},
		scannedAt: time.Now(),
	}}
	out := m.View()
	if !strings.Contains(out, "No running cells") {
		t.Errorf("expected 'No running cells', got: %s", out)
	}
}

func TestCellsModel_RendersCellTable(t *testing.T) {
	m := cellsModel{
		data: &scanResultMsg{
			cells: scan.CellsResult{
				Cells: []docker.CellInfo{
					{Name: "cell-devcell-30", Status: "running", Workspace: "main", CellID: "30", Uptime: "4 hours"},
					{Name: "cell-fm1-110", Status: "running", Workspace: "main", CellID: "110", Uptime: "2 days"},
				},
				Stats: []docker.CellStats{
					{Name: "cell-devcell-30", CPUPercent: 2.1, MemUsage: "1.8GiB / 16GiB"},
					{Name: "cell-fm1-110", CPUPercent: 0.3, MemUsage: "900MiB / 16GiB"},
				},
			},
			build:     scan.BuildResult{},
			scannedAt: time.Now(),
		},
		width:  120,
		height: 40,
	}
	out := m.View()
	for _, want := range []string{"cell-devcell-30", "cell-fm1-110", "2.1%", "0.3%", "Running Cells"} {
		if !strings.Contains(out, want) {
			t.Errorf("cells view missing %q:\n%s", want, out)
		}
	}
}

func TestCellsModel_ShowsStaleCells(t *testing.T) {
	m := cellsModel{
		data: &scanResultMsg{
			cells: scan.CellsResult{
				Cells: []docker.CellInfo{{Name: "cell-x-1", Status: "running"}},
			},
			build: scan.BuildResult{
				Stale: []docker.StaleCellInfo{{Name: "old-cell", Uptime: "5 days"}},
			},
			scannedAt: time.Now(),
		},
		width: 80, height: 40,
	}
	out := m.View()
	if !strings.Contains(out, "1 stale container") {
		t.Errorf("expected stale warning:\n%s", out)
	}
}

// --- Health screen tests ---

func TestHealthModel_NilData(t *testing.T) {
	m := healthModel{}
	out := m.View()
	if !strings.Contains(out, "No scan data") {
		t.Errorf("expected placeholder, got: %s", out)
	}
}

func TestHealthModel_NixUnavailable(t *testing.T) {
	m := healthModel{data: &scanResultMsg{
		nix:       scan.NixResult{NixOK: false},
		build:     scan.BuildResult{},
		docker:    scan.DockerResult{},
		scannedAt: time.Now(),
	}}
	out := m.View()
	if !strings.Contains(out, "nix store unavailable") {
		t.Errorf("expected unavailable message:\n%s", out)
	}
}

func TestHealthModel_RendersNixHealth(t *testing.T) {
	m := healthModel{data: &scanResultMsg{
		nix: scan.NixResult{
			NixOK: true,
			NixReport: docker.NixStoreReport{
				VolumeBytes: 16_000_000_000,
				Health: docker.NixStoreHealth{
					TotalRoots:          4,
					StaleRoots:          1,
					ProfileHashes:       2,
					Generations:         10,
					OrphanedGenerations: 3,
					DistinctRevs:        2,
				},
			},
		},
		build:     scan.BuildResult{},
		docker:    scan.DockerResult{},
		scannedAt: time.Now(),
	}}
	out := m.View()
	for _, want := range []string{
		"nix store accessible",
		"1 stale",
		"2 distinct hashes",
		"3 orphaned",
		"2 distinct nixpkgs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("health view missing %q:\n%s", want, out)
		}
	}
}

func TestHealthModel_RendersDockerHealth(t *testing.T) {
	m := healthModel{data: &scanResultMsg{
		nix: scan.NixResult{NixOK: false},
		build: scan.BuildResult{
			DiskUsage: docker.DiskUsage{
				Images: docker.DiskUsageRow{Size: "5GB"},
			},
			Stale: []docker.StaleCellInfo{{Name: "old"}},
			LiveClosures: []docker.LiveClosure{
				{Container: "c1", ProfilePath: "/nix/store/abc-home-manager-path"},
			},
		},
		docker: scan.DockerResult{
			VMDisk:   docker.VMDiskInfo{TotalBytes: 100_000_000_000, UsedBytes: 90_000_000_000},
			VMDiskOK: true,
		},
		scannedAt: time.Now(),
	}}
	out := m.View()
	for _, want := range []string{
		"Docker daemon responding",
		"1 stale containers",
		"1 live closures",
		"90%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("health view missing %q:\n%s", want, out)
		}
	}
}

func TestHealthModel_HealthyState(t *testing.T) {
	m := healthModel{data: &scanResultMsg{
		nix: scan.NixResult{
			NixOK: true,
			NixReport: docker.NixStoreReport{
				Health: docker.NixStoreHealth{
					TotalRoots:    2,
					ProfileHashes: 1,
					DistinctRevs:  1,
				},
			},
		},
		build:     scan.BuildResult{},
		docker:    scan.DockerResult{},
		scannedAt: time.Now(),
	}}
	out := m.View()
	if !strings.Contains(out, "none stale") {
		t.Errorf("expected 'none stale' for healthy state:\n%s", out)
	}
	if !strings.Contains(out, "1 hash") {
		t.Errorf("expected '1 hash' for single profile:\n%s", out)
	}
	if !strings.Contains(out, "all cells on same rev") {
		t.Errorf("expected 'all cells on same rev':\n%s", out)
	}
}

// --- Integration: doctor model delegates to screens ---

func TestDoctorModel_DiskScreenRendersAfterScan(t *testing.T) {
	m := newDoctorModel()
	m.width = 80
	m.height = 24
	m.active = screenDisk

	data := testDiskData()
	updated, _ := m.Update(*data)
	dm := updated.(doctorModel)
	out := dm.View()
	if !strings.Contains(out, "Volumes") {
		t.Errorf("disk screen should show Volumes after scan:\n%s", out)
	}
}

func TestDoctorModel_CellsScreenRendersAfterScan(t *testing.T) {
	m := newDoctorModel()
	m.width = 80
	m.height = 24
	m.active = screenCells

	data := scanResultMsg{
		cells: scan.CellsResult{
			Cells: []docker.CellInfo{{Name: "test-cell", Status: "running", Uptime: "1h"}},
		},
		build:     scan.BuildResult{},
		scannedAt: time.Now(),
	}
	updated, _ := m.Update(data)
	dm := updated.(doctorModel)
	out := dm.View()
	if !strings.Contains(out, "test-cell") {
		t.Errorf("cells screen should show cell name after scan:\n%s", out)
	}
}

func TestDoctorModel_HealthScreenRendersAfterScan(t *testing.T) {
	m := newDoctorModel()
	m.width = 80
	m.height = 24
	m.active = screenHealth

	data := scanResultMsg{
		nix:       scan.NixResult{NixOK: true, NixReport: docker.NixStoreReport{Health: docker.NixStoreHealth{TotalRoots: 2}}},
		build:     scan.BuildResult{},
		docker:    scan.DockerResult{},
		scannedAt: time.Now(),
	}
	updated, _ := m.Update(data)
	dm := updated.(doctorModel)
	out := dm.View()
	if !strings.Contains(out, "Nix Store") {
		t.Errorf("health screen should show Nix Store section:\n%s", out)
	}
}

// --- Disk screen key forwarding ---

func TestDoctorModel_DiskKeyForwarding(t *testing.T) {
	m := newDoctorModel()
	m.width = 80
	m.height = 24

	data := testDiskData()
	updated, _ := m.Update(*data)
	m = updated.(doctorModel)

	// Switch to disk
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(doctorModel)

	// j should be forwarded to disk model, not switch tabs
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(doctorModel)
	if m.active != screenDisk {
		t.Error("j on disk screen should stay on disk, not switch tabs")
	}
	if m.disk.cursor != 1 {
		t.Errorf("j should move disk cursor to 1, got %d", m.disk.cursor)
	}
}
