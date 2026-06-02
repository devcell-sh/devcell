package runner

import (
	"fmt"
	"os"
	"runtime"
)

// PreflightNixBuilder reports whether the host can usefully invoke a pure
// nix build (`nix build path:#packages.aarch64-linux.<...>-pure-image`).
//
// Returns nil on hosts that can build; returns an actionable error otherwise.
// On a non-Linux host that lacks a Linux remote builder (or extra-platforms),
// the returned error explains exactly which probe path failed and how to fix
// it (nix-darwin linux-builder, container build, or DEVCELL_PURE_SKIP_PREFLIGHT
// bypass).
//
// Callers can run this independently (before invoking BuildImagePure) to
// decide whether the host can usefully attempt a pure build at all.
func PreflightNixBuilder(stack string) error {
	return PreflightNixBuilderFromProbe(CheckNixLinuxBuilder(), stack)
}

// PreflightNixBuilderFromProbe is the testable seam — tests inject a
// LinuxBuilderProbe directly so the macOS-failure branch can be exercised
// on any host (including the Linux CI runner).
func PreflightNixBuilderFromProbe(probe LinuxBuilderProbe, stack string) error {
	// On Linux hosts the probe always reports OK with Source="linux-host"
	// (see CheckNixLinuxBuilder). DEVCELL_PURE_SKIP_PREFLIGHT also short-
	// circuits to OK with Source="env". Either way: nil.
	if probe.OK {
		return nil
	}
	return fmt.Errorf(`pure build: host is macOS (%s) but devcell images target aarch64-linux.
nix cannot cross-compile to Linux natively without a Linux remote builder.

── What we checked ─────────────────────────────────────────────────
  Probe source: %s
  Command:      %s
  builders:        %s
  extra-platforms: %s
  machines file:   %s
  machines lines:  %s%s

A working linux-builder shows up as either:
  - extra-platforms containing aarch64-linux (or x86_64-linux), OR
  - a 'builders = @/etc/nix/machines' line where that file lists a
    Linux platform on a remote builder.

── How to verify yourself ──────────────────────────────────────────
  nix config show | grep -iE 'builders|extra-platforms'
  launchctl list | grep linux-builder       # nix-darwin LaunchDaemon
  nix store ping --store ssh-ng://linux-builder

── Fixes ───────────────────────────────────────────────────────────
  1. nix-darwin (recommended):
       nix.linux-builder.enable = true;
       nix.settings.trusted-users = [ "@admin" "%s" ];
       Then: sudo darwin-rebuild switch --flake <path>
     After switch, nix config show MUST include aarch64-linux in
     extra-platforms — if it doesn't, the rebuild silently failed.

  2. Build inside a Linux devcell container, push to a registry:
       docker exec -it <linux-container> task image:build:pure STACK=%s

  3. Bypass this check entirely (e.g. CI on Linux misdetected as Darwin):
       DEVCELL_PURE_SKIP_PREFLIGHT=1 cell claude --pure`,
		runtime.GOARCH,
		probe.Source,
		orFallback(probe.ConfigCmd, "(not run)"),
		orFallback(probe.BuildersLine, "(not set)"),
		orFallback(probe.ExtraPlatforms, "(not set)"),
		orFallback(probe.MachinesFile, "(not referenced)"),
		orFallback(probe.MachinesLines, "(not read)"),
		probeErrSuffix(probe),
		os.Getenv("USER"),
		stack)
}
