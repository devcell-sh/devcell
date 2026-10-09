package docker

import (
	streak "github.com/dimmkirr/go-streak-chart"

	"github.com/DimmKirr/devcell/internal/ux"
)

// bootGroups returns the 4-group panel layout. Each group is a row; each
// step within a group is a dot (column). Conditional steps (Prompt, API
// keys, WireGuard, Postgres, Agent config, MCP, GUI) are included only
// when relevant to the cell configuration.
func bootGroups(binary string, opts groupOpts) []ux.PanelGroup {
	// Group 1: Prepare — host infrastructure before the image runs.
	prepare := ux.PanelGroup{Label: "Prepare", Steps: []string{
		"Docker",
		"Network",
		"Orphan cleanup",
		"Nix store",
		"Backup",
	}}
	if opts.needsBuild {
		prepare.Steps = append(prepare.Steps, "Image build")
	}
	prepare.Steps = append(prepare.Steps, "Image pin")

	// Group 2: Configure — project-specific settings injected into the container.
	configure := ux.PanelGroup{Label: "Configure", Steps: []string{}}
	if opts.hasPackages {
		configure.Steps = append(configure.Steps, "Packages")
	}
	if opts.hasGitLookup {
		configure.Steps = append(configure.Steps, "Git")
	}
	if binary == "claude" || binary == "codex" {
		configure.Steps = append(configure.Steps, "Prompt")
	}
	if opts.hasSecrets {
		configure.Steps = append(configure.Steps, "Secrets")
	}
	if opts.hasAPIKeys {
		configure.Steps = append(configure.Steps, "API keys")
	}
	if opts.hasWireGuard {
		configure.Steps = append(configure.Steps, "WireGuard")
	}

	// Group 3: Boot — container startup and s6 service activation.
	boot := ux.PanelGroup{Label: "Boot", Steps: []string{
		"Container",
		"Entrypoint",
		"Nix daemon",
		"Services",
	}}
	if opts.hasSecrets {
		boot.Steps = append(boot.Steps, "Secrets sync")
	}
	if opts.hasPostgres {
		boot.Steps = append(boot.Steps, "Postgres")
	}

	// Group 4 (conditional): Services — individual s6 desktop services.
	var services *ux.PanelGroup
	if opts.hasGUI {
		services = &ux.PanelGroup{Label: "Services", Steps: []string{
			"Display",
			"Desktop",
			"Remote access",
			"Audio",
			"D-Bus",
		}}
	}

	// Group 5: Environment — dev tools and agent readiness.
	env := ux.PanelGroup{Label: "Environment", Steps: []string{
		"Shell",
		"Mise",
		"Home",
	}}
	if opts.hasNixPackages {
		env.Steps = append(env.Steps, "Nix packages")
	}
	if binary == "claude" || binary == "codex" || binary == "gemini" || binary == "opencode" {
		env.Steps = append(env.Steps, "Agent config")
	}
	if opts.hasMCP {
		env.Steps = append(env.Steps, "MCP")
	}
	if opts.hasGUI {
		env.Steps = append(env.Steps, "GUI")
	}

	groups := []ux.PanelGroup{prepare, configure, boot}
	if services != nil {
		groups = append(groups, *services)
	}
	groups = append(groups, env)
	return groups
}

// groupOpts controls which conditional steps appear.
type groupOpts struct {
	needsBuild     bool
	hasPackages    bool
	hasGitLookup   bool
	hasSecrets     bool
	hasAPIKeys     bool
	hasWireGuard   bool
	hasPostgres    bool
	hasNixPackages bool
	hasMCP         bool
	hasGUI         bool
}

// bootComponentStep maps a boot-event component name to its (group, step)
// in the panel. The group label is the row; the step name is the column.
// Entries cover both sentinel-file short names (entrypoint, s6, shell, ...)
// and full s6 service names (shell-rc, nix-daemon, homedir, ...) so the
// same map works for both BootDirWatcher and ContainerLogWatcher events.
var bootComponentStep = map[string]struct{ Group, Step string }{
	"container":  {"Boot", "Container"},
	"entrypoint": {"Boot", "Entrypoint"},
	"s6":         {"Boot", "Services"},
	"nix":        {"Boot", "Nix daemon"},
	"shell":      {"Environment", "Shell"},
	"mise":       {"Environment", "Mise"},
	"home":       {"Environment", "Home"},
	"secrets":    {"Boot", "Secrets sync"},
	"claude":     {"Environment", "Agent config"},
	"codex":      {"Environment", "Agent config"},
	"gemini":     {"Environment", "Agent config"},
	"opencode":   {"Environment", "Agent config"},
	"postgres":      {"Boot", "Postgres"},
	"gui":           {"Environment", "GUI"},
	"mcp-toggle":    {"Environment", "MCP"},
	"xvfb":          {"Services", "Display"},
	"window-manager":{"Services", "Desktop"},
	"xrdp":          {"Services", "Remote access"},
	"pulseaudio":    {"Services", "Audio"},
	"dbus-session":  {"Services", "D-Bus"},
	// s6 service names that differ from the short sentinel names above.
	"nix-daemon":      {"Boot", "Nix daemon"},
	"homedir":         {"Environment", "Home"},
	"shell-rc":        {"Environment", "Shell"},
	"claude-config":   {"Environment", "Agent config"},
	"codex-config":    {"Environment", "Agent config"},
	"gemini-config":   {"Environment", "Agent config"},
	"opencode-config": {"Environment", "Agent config"},
	"gui-config":      {"Environment", "GUI"},
}

// ConsumeBootEventsPanel reads BootDirWatcher events and updates the
// streak-chart panel. Returns true when boot.ready arrived (clean
// finish), false when the channel closed without it (crash/interrupt).
func ConsumeBootEventsPanel(events <-chan BootEvent, panel *ux.BootPanel) bool {
	for ev := range events {
		if ev.Component == "boot" && ev.State == "ready" {
			panel.PromoteRunning()
			panel.Finish("Cell ready")
			return true
		}

		if ev.State == "warn" {
			msg := ev.Detail
			if msg == "" {
				msg = ev.Title
			}
			if msg == "" {
				msg = ev.Component + " warned"
			}
			panel.WarnBoot(msg)
			continue
		}

		loc, known := bootComponentStep[ev.Component]
		if !known {
			continue
		}

		title := ev.Title
		if title == "" {
			title = ev.Component + " " + ev.State
		}

		switch ev.State {
		case "starting":
			panel.SetBoot(loc.Group, loc.Step, streak.Running, title)
		case "ready":
			panel.SetBoot(loc.Group, loc.Step, streak.Done, title)
		}
	}
	panel.FinishError("boot interrupted")
	return false
}

// ConsumeContainerEventsPanel reads JSONL events from a ContainerLogWatcher
// and updates the boot panel. Handles both system events (from _notify in
// the entrypoint) and service events (from devcell-event in s6 services).
// Returns true when boot.ready arrived (clean finish), false on channel close.
func ConsumeContainerEventsPanel(events <-chan ContainerEvent, panel *ux.BootPanel) bool {
	for ev := range events {
		if ev.Service == "boot" && ev.Status == "ready" {
			// Longruns emit "activating" at run-script start but exec
			// into the daemon, so they never emit "up". s6-rc has
			// returned successfully at this point: promote any Running
			// dots to Done before finishing.
			panel.PromoteRunning()
			panel.Finish("Cell ready")
			return true
		}

		if ev.Status == "warn" {
			msg := ev.Msg
			if msg == "" {
				msg = ev.Service + " warned"
			}
			panel.WarnBoot(msg)
			continue
		}

		if ev.Status == "failed" {
			loc, known := bootComponentStep[ev.Service]
			if known {
				msg := ev.Msg
				if msg == "" {
					msg = ev.Service + " failed"
				}
				panel.SetBoot(loc.Group, loc.Step, streak.Error, msg)
			}
			continue
		}

		loc, known := bootComponentStep[ev.Service]
		if !known {
			continue
		}

		title := ev.Msg
		if title == "" {
			title = titleFor(ev.Service, ev.Status)
		}
		if title == "" {
			title = ev.Service + " " + ev.Status
		}

		switch ev.Status {
		case "starting", "activating":
			panel.SetBoot(loc.Group, loc.Step, streak.Running, title)
		case "ready", "up":
			panel.SetBoot(loc.Group, loc.Step, streak.Done, title)
		}
	}
	panel.FinishError("boot interrupted")
	return false
}
