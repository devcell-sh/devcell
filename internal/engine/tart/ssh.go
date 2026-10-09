package tart

import (
	"path/filepath"

	"github.com/DimmKirr/devcell/internal/cell"
)

// DarwinVMUser is the fixed nix-darwin/home-manager user inside the macOS VM
// (devcell-sh/home's darwinVMUser). Agent binaries are installed into its
// per-user profile, regardless of which session user runs the cell.
const DarwinVMUser = "devcell"

// nixProfileSource makes nix and the home-manager-installed agent binaries
// available in the exec shell. Sourcing nix-daemon.sh only yields nix itself;
// the session user (host $USER) is not DarwinVMUser, so its per-user profile
// and the nix-darwin system profile must be bridged onto PATH explicitly.
const nixProfileSource = `. /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh 2>/dev/null || . "$HOME/.nix-profile/etc/profile.d/nix.sh" 2>/dev/null || true; export PATH="/etc/profiles/per-user/` + DarwinVMUser + `/bin:/run/current-system/sw/bin:$PATH"`

// ExecSpec describes a command to run inside a tart VM via `tart exec`.
type ExecSpec struct {
	Binary     string   // binary to run (e.g. "zsh", "claude")
	Flags      []string // default flags for the binary
	UserArgs   []string // user-provided args
	EnvVars    []string // KEY=VAL pairs to set in the environment
	ProjectDir string   // host project path — basename fallback for cd ~/basename
	WorkDir    string   // absolute in-VM project path (ProjectPathInVM); wins over ProjectDir
	RunAsUser  string   // if set, wrap command with sudo -u <user> -i
}

// BuildExecCommand constructs a shell command string for `tart exec <vm> bash -l -c <cmd>`.
// Sources the nix daemon profile, cds into the project dir, sets env vars,
// and runs the binary.
//
// When RunAsUser is set, the entire command is wrapped with
// `sudo -u <user> -i bash -l -c '...'` so the session runs as the
// specified user (matching Docker's HOST_USER model).
func BuildExecCommand(spec ExecSpec) string {
	var tokens []string
	if len(spec.EnvVars) > 0 {
		tokens = append(tokens, "env")
		tokens = append(tokens, spec.EnvVars...)
	}
	tokens = append(tokens, spec.Binary)
	tokens = append(tokens, spec.Flags...)
	tokens = append(tokens, spec.UserArgs...)

	agentCmd := cell.ShellJoin(tokens)

	var cmd string
	switch {
	case spec.WorkDir != "":
		cmd = "cd " + cell.ShellQuote(spec.WorkDir) + " && " + agentCmd
	case spec.ProjectDir != "":
		basename := filepath.Base(spec.ProjectDir)
		cmd = "cd ~/" + cell.ShellQuote(basename) + " && " + agentCmd
	default:
		cmd = agentCmd
	}

	innerCmd := nixProfileSource + "; " + cmd

	if spec.RunAsUser != "" {
		return "sudo -u " + cell.ShellQuote(spec.RunAsUser) + " -i bash -l -c " + cell.ShellQuote(innerCmd)
	}
	return innerCmd
}
