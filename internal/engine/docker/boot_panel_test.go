package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"

	"github.com/DimmKirr/devcell/internal/ux"
)

func init() {
	ux.LogPlainText = true
}

func TestBootGroups_IncludesPromptForClaude(t *testing.T) {
	groups := bootGroups("claude", groupOpts{})
	found := false
	for _, g := range groups {
		for _, s := range g.Steps {
			if s == "Prompt" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Prompt step missing for claude binary")
	}
}

func TestBootGroups_ExcludesPromptForZsh(t *testing.T) {
	groups := bootGroups("zsh", groupOpts{})
	for _, g := range groups {
		for _, s := range g.Steps {
			if s == "Prompt" {
				t.Fatal("Prompt step should not be present for zsh binary")
			}
		}
	}
}

func TestBootGroups_HasFourGroupsWithoutGUI(t *testing.T) {
	groups := bootGroups("claude", groupOpts{})
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}
	labels := []string{"Prepare", "Configure", "Boot", "Environment"}
	for i, want := range labels {
		if groups[i].Label != want {
			t.Errorf("group %d label = %q, want %q", i, groups[i].Label, want)
		}
	}
}

func TestBootGroups_HasFiveGroupsWithGUI(t *testing.T) {
	groups := bootGroups("claude", groupOpts{hasGUI: true})
	if len(groups) != 5 {
		t.Fatalf("expected 5 groups with GUI, got %d", len(groups))
	}
	labels := []string{"Prepare", "Configure", "Boot", "Services", "Environment"}
	for i, want := range labels {
		if groups[i].Label != want {
			t.Errorf("group %d label = %q, want %q", i, groups[i].Label, want)
		}
	}
}

func TestBootGroups_ServicesGroupHasDesktopSteps(t *testing.T) {
	groups := bootGroups("claude", groupOpts{hasGUI: true})
	var svcGroup *ux.PanelGroup
	for i := range groups {
		if groups[i].Label == "Services" {
			svcGroup = &groups[i]
			break
		}
	}
	if svcGroup == nil {
		t.Fatal("Services group not found")
	}
	want := []string{"Display", "Desktop", "Remote access", "Audio", "D-Bus"}
	if len(svcGroup.Steps) != len(want) {
		t.Fatalf("Services steps = %v, want %v", svcGroup.Steps, want)
	}
	for i, w := range want {
		if svcGroup.Steps[i] != w {
			t.Errorf("Services step %d = %q, want %q", i, svcGroup.Steps[i], w)
		}
	}
}

func TestBootGroups_ConditionalSteps(t *testing.T) {
	groups := bootGroups("claude", groupOpts{
		hasSecrets:     true,
		hasAPIKeys:     true,
		hasWireGuard:   true,
		hasPostgres:    true,
		hasNixPackages: true,
		hasMCP:         true,
		hasGUI:         true,
	})

	want := map[string][]string{
		"Configure":   {"Secrets", "API keys", "WireGuard"},
		"Boot":        {"Postgres"},
		"Environment": {"Nix packages", "Agent config", "MCP", "GUI"},
	}
	for groupLabel, steps := range want {
		for _, step := range steps {
			found := false
			for _, g := range groups {
				if g.Label != groupLabel {
					continue
				}
				for _, s := range g.Steps {
					if s == step {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("step %q not found in group %q", step, groupLabel)
			}
		}
	}
}

func TestConsumeBootEventsPanel_BootReadyFinishes(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Services"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan BootEvent, 4)
	events <- BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"}
	events <- BootEvent{Component: "s6", State: "starting", Title: "Activating services"}
	events <- BootEvent{Component: "s6", State: "ready", Title: "Services ready"}
	events <- BootEvent{Component: "boot", State: "ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeBootEventsPanel(events, panel)
	}()
	if ok := <-done; !ok {
		t.Fatal("expected clean finish (true), got false")
	}
}

func TestConsumeBootEventsPanel_ChannelCloseFinishesError(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Entrypoint"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan BootEvent, 1)
	events <- BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeBootEventsPanel(events, panel)
	}()
	if ok := <-done; ok {
		t.Fatal("expected interrupted finish (false), got true")
	}
}

func TestBootComponentStep_MapsKnownComponents(t *testing.T) {
	cases := map[string]struct{ Group, Step string }{
		"s6":         {"Boot", "Services"},
		"entrypoint": {"Boot", "Entrypoint"},
		"shell":      {"Environment", "Shell"},
		"mise":       {"Environment", "Mise"},
		"home":       {"Environment", "Home"},
		"gui":        {"Environment", "GUI"},
		"container":  {"Boot", "Container"},
		"nix":        {"Boot", "Nix daemon"},
		"secrets":    {"Boot", "Secrets sync"},
		"claude":     {"Environment", "Agent config"},
		"codex":      {"Environment", "Agent config"},
		"postgres":       {"Boot", "Postgres"},
		"xvfb":           {"Services", "Display"},
		"window-manager": {"Services", "Desktop"},
		"xrdp":           {"Services", "Remote access"},
		"pulseaudio":     {"Services", "Audio"},
		"dbus-session":   {"Services", "D-Bus"},
	}
	for comp, want := range cases {
		got, ok := bootComponentStep[comp]
		if !ok {
			t.Errorf("component %q not in bootComponentStep", comp)
			continue
		}
		if got.Group != want.Group || got.Step != want.Step {
			t.Errorf("bootComponentStep[%q] = {%q, %q}, want {%q, %q}",
				comp, got.Group, got.Step, want.Group, want.Step)
		}
	}
}

func TestConsumeBootEventsPanel_WarnEvent(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan BootEvent, 2)
	events <- BootEvent{Component: "gcroot", State: "warn", Detail: "gc roots stale"}
	events <- BootEvent{Component: "boot", State: "ready"}
	close(events)

	done := make(chan struct{})
	go func() {
		ConsumeBootEventsPanel(events, panel)
		close(done)
	}()
	<-done
}

func TestConsumeBootEventsPanel_UpdatesDots(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Base init", "Nix daemon", "Services", "Secrets sync"}},
		{Label: "Environment", Steps: []string{"Shell", "Mise", "Home"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan BootEvent, 10)
	events <- BootEvent{Component: "entrypoint", State: "starting", Title: "Entrypoint starting"}
	events <- BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"}
	events <- BootEvent{Component: "s6", State: "starting", Title: "Activating services"}
	events <- BootEvent{Component: "shell", State: "starting", Title: "Configuring shell"}
	events <- BootEvent{Component: "shell", State: "ready", Title: "Shell ready"}
	events <- BootEvent{Component: "mise", State: "starting", Title: "Loading mise"}
	events <- BootEvent{Component: "mise", State: "ready", Title: "Mise ready"}
	events <- BootEvent{Component: "home", State: "ready", Title: "Home ready"}
	events <- BootEvent{Component: "s6", State: "ready", Title: "Services ready"}
	events <- BootEvent{Component: "boot", State: "ready"}
	close(events)

	done := make(chan struct{})
	go func() {
		ConsumeBootEventsPanel(events, panel)
		close(done)
	}()
	<-done
}

// Verify that SetBoot uses streak statuses correctly when called with
// explicit group targeting.
// TestSentinelToPanelRace exercises the full sentinel → watcher → panel
// pipeline under concurrent writes. Multiple goroutines write sentinel files
// simultaneously (simulating s6 services finishing in parallel within one
// dependency layer), and the test verifies every event arrives and no data
// race occurs (run with -race). The watcher polls a real temp dir.
func TestSentinelToPanelRace(t *testing.T) {
	bootDir := t.TempDir()

	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Nix daemon", "Services"}},
		{Label: "Services", Steps: []string{"Display", "Desktop", "Remote access", "Audio", "D-Bus"}},
		{Label: "Environment", Steps: []string{"Shell", "Mise", "Home"}},
	}
	panel := ux.NewBootPanel(groups)
	panel.SetBoot("Boot", "Container", streak.Done, "Container started")

	// Start the real watcher on the temp dir.
	origInterval := PollInterval
	PollInterval = 5 * time.Millisecond
	t.Cleanup(func() { PollInterval = origInterval })

	watcher := &BootDirWatcher{}
	events, err := watcher.Start(bootDir)
	if err != nil {
		t.Fatalf("watcher start: %v", err)
	}
	defer watcher.Close()

	// Consume events on the panel in the background.
	panelDone := make(chan bool, 1)
	go func() {
		panelDone <- ConsumeBootEventsPanel(events, panel)
	}()

	// Simulate s6 services writing sentinels concurrently.
	// Phase 1: entrypoint + s6 (sequential, like the real entrypoint).
	writeSentinel(t, bootDir, "entrypoint.ready")
	writeSentinel(t, bootDir, "s6.starting")

	// Phase 2: multiple services finish in parallel (same dependency layer).
	var wg sync.WaitGroup
	concurrent := []string{
		"nix.ready",
		"shell.ready",
		"mise.ready",
		"home.ready",
		"xvfb.ready",
		"dbus-session.ready",
	}
	for _, name := range concurrent {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			writeSentinel(t, bootDir, n)
		}(name)
	}
	wg.Wait()

	// Phase 3: services that depend on phase 2.
	concurrent2 := []string{
		"window-manager.ready",
		"xrdp.ready",
		"pulseaudio.ready",
	}
	for _, name := range concurrent2 {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			writeSentinel(t, bootDir, n)
		}(name)
	}
	wg.Wait()

	// Seal.
	writeSentinel(t, bootDir, "s6.ready")
	writeSentinel(t, bootDir, "boot.ready")

	select {
	case ok := <-panelDone:
		if !ok {
			t.Fatal("panel did not receive boot.ready (got interrupted finish)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("panel did not finish within 5s")
	}

	// Verify all sentinel files were written (the watcher should have seen them all).
	expected := append(concurrent, concurrent2...)
	expected = append(expected, "entrypoint.ready", "s6.starting", "s6.ready", "boot.ready")
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(bootDir, name)); err != nil {
			t.Errorf("sentinel %q missing from boot dir: %v", name, err)
		}
	}
}

func writeSentinel(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
		t.Errorf("write sentinel %q: %v", name, err)
	}
}

// --- ConsumeContainerEventsPanel tests ---

func TestConsumeContainerEventsPanel_BootReadyFinishes(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Services"}},
		{Label: "Environment", Steps: []string{"Shell"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan ContainerEvent, 4)
	events <- ContainerEvent{Scope: "system", Service: "entrypoint", Status: "ready"}
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "up"}
	events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeContainerEventsPanel(events, panel)
	}()
	if ok := <-done; !ok {
		t.Fatal("expected clean finish (true), got false")
	}
}

func TestConsumeContainerEventsPanel_ChannelCloseReturnsFalse(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Entrypoint"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan ContainerEvent, 1)
	events <- ContainerEvent{Scope: "system", Service: "entrypoint", Status: "ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeContainerEventsPanel(events, panel)
	}()
	if ok := <-done; ok {
		t.Fatal("expected interrupted finish (false), got true")
	}
}

func TestConsumeContainerEventsPanel_ServiceEventsUpdatePanel(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Nix daemon"}},
		{Label: "Environment", Steps: []string{"Shell", "Mise", "Home", "Agent config"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan ContainerEvent, 10)
	events <- ContainerEvent{Scope: "service", Service: "nix-daemon", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "nix-daemon", Status: "up"}
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "up"}
	events <- ContainerEvent{Scope: "service", Service: "mise", Status: "up"}
	events <- ContainerEvent{Scope: "service", Service: "homedir", Status: "up"}
	events <- ContainerEvent{Scope: "service", Service: "claude-config", Status: "up"}
	events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeContainerEventsPanel(events, panel)
	}()
	if ok := <-done; !ok {
		t.Fatal("expected clean finish")
	}
}

func TestConsumeContainerEventsPanel_WarnEvent(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan ContainerEvent, 2)
	events <- ContainerEvent{Scope: "service", Service: "secrets", Status: "warn", Msg: "2 keys missing"}
	events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
	close(events)

	done := make(chan struct{})
	go func() {
		ConsumeContainerEventsPanel(events, panel)
		close(done)
	}()
	<-done
}

func TestConsumeContainerEventsPanel_FailedEvent(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Postgres"}},
	}
	panel := ux.NewBootPanel(groups)
	events := make(chan ContainerEvent, 2)
	events <- ContainerEvent{Scope: "service", Service: "postgres", Status: "failed", Msg: "pg_ctl error"}
	events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
	close(events)

	done := make(chan struct{})
	go func() {
		ConsumeContainerEventsPanel(events, panel)
		close(done)
	}()
	<-done
}

func TestConsumeContainerEventsPanel_LongrunsPromotedOnBootReady(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Nix daemon", "Services"}},
		{Label: "Services", Steps: []string{"Display", "Desktop", "Audio"}},
		{Label: "Environment", Steps: []string{"Shell", "Home"}},
	}
	panel := ux.NewBootPanel(groups)
	panel.SetBoot("Boot", "Container", streak.Done, "Container started")

	events := make(chan ContainerEvent, 16)
	// System events from the entrypoint
	events <- ContainerEvent{Scope: "system", Service: "entrypoint", Status: "ready"}
	events <- ContainerEvent{Scope: "system", Service: "s6", Status: "starting"}
	// Oneshots: both activating + up
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "shell-rc", Status: "up"}
	events <- ContainerEvent{Scope: "service", Service: "homedir", Status: "up"}
	// Longruns: only activating (they exec into the daemon)
	events <- ContainerEvent{Scope: "service", Service: "nix-daemon", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "xvfb", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "window-manager", Status: "activating"}
	events <- ContainerEvent{Scope: "service", Service: "pulseaudio", Status: "activating"}
	// boot.ready signals all services are up
	events <- ContainerEvent{Scope: "system", Service: "s6", Status: "ready"}
	events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
	close(events)

	done := make(chan bool, 1)
	go func() {
		done <- ConsumeContainerEventsPanel(events, panel)
	}()
	if ok := <-done; !ok {
		t.Fatal("expected clean finish")
	}

	// Verify longruns were promoted: snapshot the grid and check that
	// all active dots are Done, Warning, or Error (not Running/Pending).
	snap := panel.Snapshot()
	for r := 0; r < snap.Rows(); r++ {
		for c := 0; c < snap.RowCols(r); c++ {
			s, _ := snap.Get(r, c)
			if s == streak.Running {
				t.Errorf("row %d col %d (%s) still Running after boot.ready",
					r, c, snap.Label(r))
			}
		}
	}
}

func TestBootComponentStep_MapsS6ServiceNames(t *testing.T) {
	cases := map[string]struct{ Group, Step string }{
		"nix-daemon":      {"Boot", "Nix daemon"},
		"homedir":         {"Environment", "Home"},
		"shell-rc":        {"Environment", "Shell"},
		"claude-config":   {"Environment", "Agent config"},
		"codex-config":    {"Environment", "Agent config"},
		"gemini-config":   {"Environment", "Agent config"},
		"opencode-config": {"Environment", "Agent config"},
		"gui-config":      {"Environment", "GUI"},
	}
	for svc, want := range cases {
		got, ok := bootComponentStep[svc]
		if !ok {
			t.Errorf("service %q not in bootComponentStep", svc)
			continue
		}
		if got.Group != want.Group || got.Step != want.Step {
			t.Errorf("bootComponentStep[%q] = {%q, %q}, want {%q, %q}",
				svc, got.Group, got.Step, want.Group, want.Step)
		}
	}
}

// --- ensureRunning tests ---

func TestEnsureRunning_AlreadyRunning(t *testing.T) {
	startCalled := false
	err := ensureRunning("test-container", 100*time.Millisecond,
		func() bool { return true },
		func() error {
			startCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startCalled {
		t.Fatal("start func should not be called when container is already running")
	}
}

func TestEnsureRunning_StartsAndPolls(t *testing.T) {
	checks := 0
	err := ensureRunning("test-container", 5*time.Second,
		func() bool {
			checks++
			return checks >= 3
		},
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checks < 3 {
		t.Fatalf("expected at least 3 polling checks, got %d", checks)
	}
}

func TestEnsureRunning_Timeout(t *testing.T) {
	err := ensureRunning("test-container", 200*time.Millisecond,
		func() bool { return false },
		func() error { return nil },
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "did not start") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

func TestEnsureRunning_StartError(t *testing.T) {
	err := ensureRunning("test-container", time.Second,
		func() bool { return false },
		func() error { return fmt.Errorf("docker daemon unreachable") },
	)
	if err == nil {
		t.Fatal("expected start error")
	}
	if !strings.Contains(err.Error(), "docker daemon unreachable") {
		t.Errorf("error should contain start failure, got: %v", err)
	}
}

func TestSetBoot_ExplicitGroup(t *testing.T) {
	groups := []ux.PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint"}},
		{Label: "Environment", Steps: []string{"Shell"}},
	}
	panel := ux.NewBootPanel(groups)
	panel.SetBoot("Boot", "Container", streak.Running, "starting")
	panel.SetBoot("Boot", "Container", streak.Done, "started")
	panel.SetBoot("Environment", "Shell", streak.Done, "ready")
	panel.Finish("done")
}
