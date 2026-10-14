package docker

import (
	streak "github.com/dimmkirr/go-streak-chart"

	"github.com/devcell-sh/devcell/internal/ux"
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
	if opts.hasProxy {
		prepare.Steps = append(prepare.Steps, "Proxy")
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
	if opts.hasKVMProbe {
		configure.Steps = append(configure.Steps, "KVM")
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
	hasKVMProbe    bool
	hasProxy       bool
	hasPostgres    bool
	hasNixPackages bool
	hasMCP         bool
	hasGUI         bool
}

// titles maps "<component>.<state>" to a human title for boot panel rows.
var titles = map[string]string{
	"container.ready":   "Container started",
	"entrypoint.ready":  "Entrypoint ready",
	"s6.starting":       "Activating session services",
	"s6.ready":          "Session services ready",
	"nix.starting":      "Configuring nix",
	"nix.ready":         "Nix ready",
	"shell.starting":    "Configuring shell",
	"shell.ready":       "Shell ready",
	"nix-lib.starting":  "Wiring nix library paths",
	"nix-lib.ready":     "Nix library paths ready",
	"mise.starting":     "Loading mise tools",
	"mise.ready":        "Mise ready",
	"home.starting":     "Setting up home directory",
	"home.ready":        "Home directory ready",
	"secrets.starting":  "Injecting secrets",
	"secrets.ready":     "Secrets ready",
	"chromium.starting": "Releasing chromium locks",
	"chromium.ready":    "Chromium ready",
	"claude.starting":   "Initializing Claude",
	"claude.ready":      "Claude ready",
	"codex.starting":    "Initializing Codex",
	"codex.ready":       "Codex ready",
	"gemini.starting":   "Initializing Gemini",
	"gemini.ready":      "Gemini ready",
	"opencode.starting": "Initializing OpenCode",
	"opencode.ready":    "OpenCode ready",
	"postgres.starting": "Starting PostgreSQL",
	"postgres.ready":    "PostgreSQL ready",
	"gui.starting":            "Starting GUI",
	"gui.ready":               "GUI ready",
	"xvfb.ready":              "Display server ready",
	"window-manager.ready":    "Window manager ready",
	"xrdp.ready":              "Remote desktop ready",
	"pulseaudio.ready":        "Audio server ready",
	"dbus-session.ready":      "D-Bus session ready",
	"boot.ready":              "",
}

func titleFor(component, state string) string {
	return titles[component+"."+state]
}

// bootComponentStep maps a boot-event component name to its (group, step)
// in the panel. Entries cover both short names (entrypoint, s6, shell, ...)
// and full s6 service names (shell-rc, nix-daemon, homedir, ...).
var bootComponentStep = map[string]struct{ Group, Step string }{
	"proxy":      {"Prepare", "Proxy"},
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
