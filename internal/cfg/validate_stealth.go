package cfg

import "fmt"

// ValidateStealthCrossArch checks whether a cross-architecture stealth config
// is viable. On arm64 hosts, stealth.arch = "x86" requires Rosetta translation
// (available on macOS with Colima --rosetta). Returns nil if compatible, or a
// user-friendly error explaining the incompatibility.
func ValidateStealthCrossArch(stealth StealthSection, hostArch, dockerRuntime string) error {
	resolved := stealth.ResolvedArch()

	native := (hostArch == "x86_64" && resolved == "x86") ||
		(hostArch == "aarch64" && resolved == "arm")
	if native {
		return nil
	}

	if hostArch == "x86_64" && resolved == "arm" {
		return fmt.Errorf(
			"[stealth] arch = %q on %s host is not supported: no arm64 Chromium available for x86_64",
			resolved, hostArch,
		)
	}

	if hostArch == "aarch64" && resolved == "x86" {
		if dockerRuntime == "colima" {
			return nil
		}
		return fmt.Errorf(
			"[stealth] arch = %q on %s host requires Rosetta translation.\n"+
				"  Rosetta is available with Colima (colima start --rosetta), but the current\n"+
				"  Docker runtime is %q. The x86_64 Chromium binary will not execute without it.",
			resolved, hostArch, dockerRuntime,
		)
	}

	return nil
}
