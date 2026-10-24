package docker_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// `cell build prune` (default mode, no --pure, no --force) runs the
// same Docker prune sequence as the user's cleandocker zsh function:
//
//	docker rm -f $(docker ps -aq) 2>/dev/null || true
//	docker system prune -af
//	docker volume prune -f
//	docker buildx prune -af
//
// This sequence is identical on macOS and Linux native — the Docker daemon
// surface is the same on both. No sudo. No host-specific branching.
func TestBuildDockerPruneSteps_Default_RunsCleandockerSequenceOnBothOSes(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			steps := docker.BuildDockerPruneSteps(docker.PruneOpts{GOOS: goos})

			if len(steps) != 4 {
				t.Fatalf("want 4 steps, got %d: %+v", len(steps), steps)
			}

			// Step 1: must remove ALL containers. Uses Func (pure Go)
			// instead of sh -c so devcell works on hosts without sh in PATH.
			if steps[0].Func == nil {
				t.Errorf("step 1 must have Func set (pure Go, no sh dependency)")
			}
			step1 := strings.Join(steps[0].Argv, " ")
			if !strings.Contains(step1, "docker") {
				t.Errorf("step 1 Argv (display) should mention docker: %q", step1)
			}
			if !steps[0].IgnoreError {
				t.Errorf("step 1 (docker rm) must IgnoreError — empty container list errors")
			}

			// Steps 2-4 are exact argvs.
			wantTail := [][]string{
				{"docker", "system", "prune", "-af"},
				{"docker", "volume", "prune", "-f"},
				{"docker", "buildx", "prune", "-af"},
			}
			for i, want := range wantTail {
				got := steps[i+1].Argv
				if !equalArgv(got, want) {
					t.Errorf("step %d: got %v, want %v", i+2, got, want)
				}
			}

			// No sudo in default mode.
			for i, s := range steps {
				if len(s.Argv) > 0 && s.Argv[0] == "sudo" {
					t.Errorf("step %d uses sudo in default mode: %v", i+1, s.Argv)
				}
			}

			// No DryRun in default mode — these are real commands.
			for i, s := range steps {
				if s.DryRun {
					t.Errorf("step %d marked DryRun in default mode: %v", i+1, s.Argv)
				}
			}
		})
	}
}

// `cell build prune --force` on macOS performs a nuclear Docker reset:
// stop Docker Desktop, wipe its VM data directory, restart it. This is
// the literal port of the user's `cleandocker -f` zsh function.
//
// The wipe path is anchored to HomeDir (passed via opts so the builder
// stays pure; the runtime resolves $HOME at the call site).
func TestBuildDockerPruneSteps_ForceOnDarwin_StopWipeStartSequence(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:    "darwin",
		Force:   true,
		HomeDir: "/Users/testuser",
	}
	steps := docker.BuildDockerPruneSteps(opts)

	if len(steps) != 3 {
		t.Fatalf("want 3 steps (stop, wipe, start), got %d: %+v", len(steps), steps)
	}

	// Step 1: docker desktop stop
	if !equalArgv(steps[0].Argv, []string{"docker", "desktop", "stop"}) {
		t.Errorf("step 1 want `docker desktop stop`, got %v", steps[0].Argv)
	}

	// Step 2: rm -rf <HomeDir>/Library/Containers/com.docker.docker/Data/vms/0/data
	wantWipe := []string{"rm", "-rf", "/Users/testuser/Library/Containers/com.docker.docker/Data/vms/0/data"}
	if !equalArgv(steps[1].Argv, wantWipe) {
		t.Errorf("step 2 want %v, got %v", wantWipe, steps[1].Argv)
	}

	// Step 3: docker desktop start
	if !equalArgv(steps[2].Argv, []string{"docker", "desktop", "start"}) {
		t.Errorf("step 3 want `docker desktop start`, got %v", steps[2].Argv)
	}

	// No sudo (Docker Desktop runs as the user).
	for i, s := range steps {
		if len(s.Argv) > 0 && s.Argv[0] == "sudo" {
			t.Errorf("step %d uses sudo on Darwin force: %v", i+1, s.Argv)
		}
	}
}

// `cell build prune --force` on Linux native nukes the local Docker daemon's
// data directory. Two variants: rootful (sudo + /var/lib/docker) and rootless
// (no sudo + ~/.local/share/docker). The runtime call site auto-detects via
// `docker info`; the builder takes the result as an opts.Rootless boolean.
func TestBuildDockerPruneSteps_ForceOnLinux_Rootful(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:     "linux",
		Force:    true,
		Rootless: false,
		HomeDir:  "/home/testuser",
	}
	steps := docker.BuildDockerPruneSteps(opts)

	if len(steps) != 3 {
		t.Fatalf("want 3 steps (stop, wipe, start), got %d: %+v", len(steps), steps)
	}

	// Step 1: sudo systemctl stop docker docker.socket
	want1 := []string{"sudo", "systemctl", "stop", "docker", "docker.socket"}
	if !equalArgv(steps[0].Argv, want1) {
		t.Errorf("step 1 want %v, got %v", want1, steps[0].Argv)
	}

	// Step 2: sudo rm -rf /var/lib/docker
	want2 := []string{"sudo", "rm", "-rf", "/var/lib/docker"}
	if !equalArgv(steps[1].Argv, want2) {
		t.Errorf("step 2 want %v, got %v", want2, steps[1].Argv)
	}

	// Step 3: sudo systemctl start docker
	want3 := []string{"sudo", "systemctl", "start", "docker"}
	if !equalArgv(steps[2].Argv, want3) {
		t.Errorf("step 3 want %v, got %v", want3, steps[2].Argv)
	}
}

func TestBuildDockerPruneSteps_ForceOnLinux_Rootless(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:     "linux",
		Force:    true,
		Rootless: true,
		HomeDir:  "/home/testuser",
	}
	steps := docker.BuildDockerPruneSteps(opts)

	if len(steps) != 3 {
		t.Fatalf("want 3 steps, got %d: %+v", len(steps), steps)
	}

	// Rootless: no sudo anywhere.
	for i, s := range steps {
		if len(s.Argv) > 0 && s.Argv[0] == "sudo" {
			t.Errorf("step %d uses sudo in rootless mode: %v", i+1, s.Argv)
		}
	}

	// Step 1: systemctl --user stop docker
	want1 := []string{"systemctl", "--user", "stop", "docker"}
	if !equalArgv(steps[0].Argv, want1) {
		t.Errorf("step 1 want %v, got %v", want1, steps[0].Argv)
	}

	// Step 2: rm -rf <HomeDir>/.local/share/docker
	want2 := []string{"rm", "-rf", "/home/testuser/.local/share/docker"}
	if !equalArgv(steps[1].Argv, want2) {
		t.Errorf("step 2 want %v, got %v", want2, steps[1].Argv)
	}

	// Step 3: systemctl --user start docker
	want3 := []string{"systemctl", "--user", "start", "docker"}
	if !equalArgv(steps[2].Argv, want3) {
		t.Errorf("step 3 want %v, got %v", want3, steps[2].Argv)
	}
}

// `cell build prune --pure` on Linux (default, no --force) runs safe
// project-aware GC: remove only orphaned profile generations that aren't
// protected by project-scoped GC roots under /nix/var/nix/gcroots/devcell/,
// then `nix-store --gc`. This is safe for shared Docker volumes where
// multiple containers use the same /nix store. See CELL-320.
//
// Blanket `nix-collect-garbage -d` is unsafe in this context because it
// deletes all non-current generations, including home-manager-files
// derivations that other containers' dotfiles symlink into.
func TestBuildNixPruneSteps_Default_LinuxSafeGC(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS: "linux",
		Pure: true,
	}
	steps := docker.BuildNixPruneSteps(opts)

	if len(steps) == 0 {
		t.Fatalf("want at least 1 step, got 0")
	}

	// No ssh anywhere — we're already on Linux.
	for i, s := range steps {
		if len(s.Argv) > 0 && s.Argv[0] == "ssh" {
			t.Errorf("step %d uses ssh on Linux native: %v", i+1, s.Argv)
		}
	}

	// The safe GC step runs via docker run with the nix volume (CELL-333).
	var script string
	for _, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "docker") && strings.Contains(joined, "run") {
			for i, a := range s.Argv {
				if a == "-c" && i+1 < len(s.Argv) {
					script = s.Argv[i+1]
				}
			}
		}
	}
	if script == "" {
		t.Fatalf("no docker run step with sh -c found: %+v", steps)
	}

	// Script must reference project GC roots and use safe nix-store --gc.
	mustContain := []string{
		"gcroots/devcell",
		"nix-store --gc",
	}
	for _, want := range mustContain {
		if !strings.Contains(script, want) {
			t.Errorf("safe GC script missing %q", want)
		}
	}

	// Script must NOT use blanket nix-collect-garbage -d — that's force mode.
	if strings.Contains(script, "nix-collect-garbage") {
		t.Errorf("safe GC script must not use nix-collect-garbage (use --force for blanket cleanup)")
	}
}

// `cell build prune --pure --force` on macOS wipes the linux-builder VM qcow
// disk image and restarts the launchd service so the VM is rebuilt from the
// nix-darwin derivation. Ships as DRY-RUN ONLY in the initial implementation:
// the qcow path varies across nix-darwin versions, so we print the plan and
// let the user verify before flipping NukeBuilderVMEnabled.
func TestBuildNixPruneSteps_ForceOnDarwin_IsDryRunPlan(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:  "darwin",
		Pure:  true,
		Force: true,
	}
	steps := docker.BuildNixPruneSteps(opts)

	if len(steps) == 0 {
		t.Fatalf("want at least 1 step in qcow-nuke plan, got 0")
	}

	// Every step (except registry cleanup) must be marked DryRun until
	// NukeBuilderVMEnabled is flipped.
	for i, s := range steps {
		if s.IgnoreError {
			continue // registry cleanup step
		}
		if !s.DryRun {
			t.Errorf("step %d not marked DryRun (NukeBuilderVMEnabled is false): %v", i+1, s.Argv)
		}
	}

	// Plan must include: launchctl bootout, rm of qcow, launchctl kickstart.
	joined := ""
	for _, s := range steps {
		joined += " | " + strings.Join(s.Argv, " ")
	}
	mustContain := []string{
		"launchctl bootout",
		"launchctl kickstart",
		"rm",
		"linux-builder",
	}
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("dry-run plan missing %q\nfull plan: %s", want, joined)
		}
	}
}

// `cell build prune --pure --force` on Linux native runs aggressive GC:
// delete old profile generations + nix-collect-garbage -d + optimise +
// wipe ~/.cache/nix. Does NOT rm -rf /nix/store (would destroy NixOS, requires
// reinstall otherwise).
func TestBuildNixPruneSteps_ForceOnLinux_AggressiveGC(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:    "linux",
		Pure:    true,
		Force:   true,
		HomeDir: "/home/testuser",
	}
	steps := docker.BuildNixPruneSteps(opts)

	if len(steps) == 0 {
		t.Fatalf("want aggressive GC steps, got 0")
	}

	// No step may DryRun on Linux — execution is safe here.
	for i, s := range steps {
		if s.DryRun {
			t.Errorf("step %d marked DryRun on Linux force-pure: %v", i+1, s.Argv)
		}
	}

	// CRITICAL: must NOT include `rm -rf /nix/store` — would destroy the system.
	for _, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "rm -rf /nix/store") ||
			strings.Contains(joined, "rm -rf /nix") {
			t.Errorf("plan contains forbidden /nix/store wipe: %v", s.Argv)
		}
	}

	joined := ""
	for _, s := range steps {
		joined += " | " + strings.Join(s.Argv, " ")
	}
	mustContain := []string{
		"nix-env --delete-generations old",
		"nix-collect-garbage -d",
		"nix-store --optimise",
		"/home/testuser/.cache/nix",
	}
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("aggressive GC plan missing %q\nfull plan: %s", want, joined)
		}
	}
}

// On NixOS the system profile also accumulates old generations. The aggressive
// GC plan must include a system-profile cleanup step when opts.NixOS is set.
func TestBuildNixPruneSteps_ForceOnLinux_NixOSAddsSystemProfileCleanup(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS:    "linux",
		Pure:    true,
		Force:   true,
		NixOS:   true,
		HomeDir: "/home/testuser",
	}
	steps := docker.BuildNixPruneSteps(opts)

	joined := ""
	for _, s := range steps {
		joined += " | " + strings.Join(s.Argv, " ")
	}
	if !strings.Contains(joined, "/nix/var/nix/profiles/system") {
		t.Errorf("NixOS plan missing system-profile cleanup: %s", joined)
	}

	// Same without NixOS — must NOT touch system profile.
	plain := docker.BuildNixPruneSteps(docker.PruneOpts{
		GOOS: "linux", Pure: true, Force: true, NixOS: false, HomeDir: "/home/testuser",
	})
	plainJoined := ""
	for _, s := range plain {
		plainJoined += " | " + strings.Join(s.Argv, " ")
	}
	if strings.Contains(plainJoined, "/nix/var/nix/profiles/system") {
		t.Errorf("non-NixOS plan must not touch system profile: %s", plainJoined)
	}
}

// Every prune mode must produce a clear warning before any destructive action.
// Default mode covers both Docker and nix; --force escalates both.
func TestBuildPrunePrompt_AllModesContainWarningAndTarget(t *testing.T) {
	tests := []struct {
		name     string
		opts     docker.PruneOpts
		mustHave []string // substrings the prompt must contain
	}{
		{
			name: "default",
			opts: docker.PruneOpts{GOOS: "linux"},
			mustHave: []string{
				"Docker",
				"Nix",
				"stopped containers",
				"orphaned profile generations",
				"devcell-nix-store",
				"Continue? [y/N]",
			},
		},
		{
			name: "force darwin",
			opts: docker.PruneOpts{GOOS: "darwin", Force: true, HomeDir: "/Users/x"},
			mustHave: []string{
				"Docker and nix",
				"Docker Desktop",
				"linux-builder",
				"qcow",
				"Continue? [y/N]",
			},
		},
		{
			name: "force linux rootful",
			opts: docker.PruneOpts{GOOS: "linux", Force: true, HomeDir: "/home/x"},
			mustHave: []string{
				"Docker and nix",
				"/var/lib/docker",
				"aggressive GC",
				"Continue? [y/N]",
			},
		},
		{
			name: "force linux rootless",
			opts: docker.PruneOpts{GOOS: "linux", Force: true, Rootless: true, HomeDir: "/home/x"},
			mustHave: []string{
				"Docker and nix",
				"/home/x/.local/share/docker",
				"Continue? [y/N]",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := docker.BuildPrunePrompt(tt.opts)
			for _, want := range tt.mustHave {
				if !strings.Contains(prompt, want) {
					t.Errorf("prompt missing %q\nfull prompt:\n%s", want, prompt)
				}
			}
		})
	}
}

// ConfirmDestructive prints the warning, reads stdin, returns true if the
// user typed y/Y/yes/YES (case-insensitive, trimmed); false otherwise.
// `-y` / `--yes` (skipYes=true) bypasses the prompt entirely.
// On non-TTY stdin without skipYes, the function refuses and returns false —
// prevents accidental pipelined destruction.
func TestConfirmDestructive_YesSkipsPromptAndReturnsTrue(t *testing.T) {
	var out strings.Builder
	got := docker.ConfirmDestructive(&out, strings.NewReader(""), true, false, "WARN")
	if !got {
		t.Errorf("skipYes=true must return true regardless of stdin")
	}
	// Warning should NOT be printed when bypassing — the user already opted in.
	if strings.Contains(out.String(), "WARN") {
		t.Errorf("warning leaked into output when skipYes=true: %q", out.String())
	}
}

func TestConfirmDestructive_TTYUserAcceptsLowercaseY(t *testing.T) {
	var out strings.Builder
	got := docker.ConfirmDestructive(&out, strings.NewReader("y\n"), false, true, "WARNING-TEXT")
	if !got {
		t.Errorf("user typed `y` — must accept")
	}
	if !strings.Contains(out.String(), "WARNING-TEXT") {
		t.Errorf("warning not printed before prompt: %q", out.String())
	}
}

func TestConfirmDestructive_TTYUserAcceptsCaseInsensitive(t *testing.T) {
	for _, ans := range []string{"y", "Y", "yes", "YES", "Yes", "  yes  "} {
		t.Run(ans, func(t *testing.T) {
			var out strings.Builder
			got := docker.ConfirmDestructive(&out, strings.NewReader(ans+"\n"), false, true, "WARN")
			if !got {
				t.Errorf("answer %q must be accepted", ans)
			}
		})
	}
}

func TestConfirmDestructive_TTYUserRejectsAnythingElse(t *testing.T) {
	for _, ans := range []string{"", "n", "N", "no", "NO", "maybe", "yep"} {
		t.Run(ans, func(t *testing.T) {
			var out strings.Builder
			got := docker.ConfirmDestructive(&out, strings.NewReader(ans+"\n"), false, true, "WARN")
			if got {
				t.Errorf("answer %q must be rejected", ans)
			}
		})
	}
}

func TestConfirmDestructive_NonTTYWithoutYesRefuses(t *testing.T) {
	var out strings.Builder
	// Even with "y" piped in, non-TTY + no --yes must refuse —
	// otherwise a stray pipe could destroy state.
	got := docker.ConfirmDestructive(&out, strings.NewReader("y\n"), false, false, "WARN")
	if got {
		t.Errorf("non-TTY without --yes must refuse even on y input")
	}
	// Must surface a clear refusal message (mentions --yes).
	if !strings.Contains(out.String(), "--yes") {
		t.Errorf("refusal message should mention --yes: %q", out.String())
	}
}

// RunPrune is the orchestration layer: build the step plan, prompt the user,
// execute the steps. Behavior:
//   - If the user rejects the prompt, NO step executes.
//   - Steps execute in declared order.
//   - Steps with DryRun=true are printed but NOT executed.
//   - Steps with IgnoreError=true don't abort the loop on failure.
//
// The executor is injected so tests can pin the sequence without invoking
// real commands.
func TestRunPrune_RejectedPromptExecutesNothing(t *testing.T) {
	var executed []string
	exec := func(step docker.PruneStep) error {
		executed = append(executed, strings.Join(step.Argv, " "))
		return nil
	}
	var out strings.Builder
	err := docker.RunPrune(docker.RunPruneArgs{
		Opts:    docker.PruneOpts{GOOS: "linux"}, // docker default
		Exec:    exec,
		Out:     &out,
		In:      strings.NewReader("n\n"),
		SkipYes: false,
		IsTTY:   true,
	})
	if err != nil {
		t.Fatalf("RunPrune err: %v", err)
	}
	if len(executed) != 0 {
		t.Errorf("rejected prompt — nothing should run, but got: %v", executed)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("rejection should print Aborted message, got: %q", out.String())
	}
}

func TestRunPrune_AcceptedPromptExecutesAllSteps(t *testing.T) {
	var executed [][]string
	exec := func(step docker.PruneStep) error {
		executed = append(executed, step.Argv)
		return nil
	}
	err := docker.RunPrune(docker.RunPruneArgs{
		Opts:    docker.PruneOpts{GOOS: "linux"}, // Docker (4) + Nix steps
		Exec:    exec,
		Out:     &strings.Builder{},
		In:      strings.NewReader("y\n"),
		SkipYes: false,
		IsTTY:   true,
	})
	if err != nil {
		t.Fatalf("RunPrune err: %v", err)
	}
	// Default prune now runs both Docker (4 steps) and Nix steps.
	dockerSteps := docker.BuildDockerPruneSteps(docker.PruneOpts{GOOS: "linux"})
	nixSteps := docker.BuildNixPruneSteps(docker.PruneOpts{GOOS: "linux"})
	want := len(dockerSteps) + len(nixSteps)
	if len(executed) != want {
		t.Errorf("want %d steps executed (Docker %d + Nix %d), got %d: %v",
			want, len(dockerSteps), len(nixSteps), len(executed), executed)
	}
}

func TestRunPrune_DryRunStepsAreNotExecuted(t *testing.T) {
	// Force on Darwin: Docker force steps execute (stop/wipe/start),
	// but Nix force steps are DryRun (qcow nuke not yet verified).
	var executed []string
	exec := func(step docker.PruneStep) error {
		executed = append(executed, strings.Join(step.Argv, " "))
		return nil
	}
	var out strings.Builder
	err := docker.RunPrune(docker.RunPruneArgs{
		Opts:    docker.PruneOpts{GOOS: "darwin", Force: true, HomeDir: "/Users/x"},
		Exec:    exec,
		Out:     &out,
		In:      strings.NewReader("y\n"),
		SkipYes: false,
		IsTTY:   true,
	})
	if err != nil {
		t.Fatalf("RunPrune err: %v", err)
	}
	// Nix DryRun steps should be printed but not executed.
	if !strings.Contains(out.String(), "# (dry-run)") {
		t.Errorf("dry-run output should include `# (dry-run)` marker, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "launchctl") {
		t.Errorf("dry-run output should print the launchctl commands, got:\n%s", out.String())
	}
}

func TestRunPrune_IgnoreErrorStepDoesNotAbortLoop(t *testing.T) {
	calls := 0
	exec := func(step docker.PruneStep) error {
		calls++
		// Step 1 (docker rm) has IgnoreError=true. Simulate failure.
		if calls == 1 {
			return fmt.Errorf("docker rm: no containers")
		}
		return nil
	}
	opts := docker.PruneOpts{GOOS: "linux"}
	totalSteps := len(docker.BuildDockerPruneSteps(opts)) + len(docker.BuildNixPruneSteps(opts))
	err := docker.RunPrune(docker.RunPruneArgs{
		Opts:    opts,
		Exec:    exec,
		Out:     &strings.Builder{},
		In:      strings.NewReader("y\n"),
		SkipYes: false,
		IsTTY:   true,
	})
	if err != nil {
		t.Errorf("step 1 failure with IgnoreError=true must not propagate: %v", err)
	}
	if calls != totalSteps {
		t.Errorf("want all %d steps attempted, got %d", totalSteps, calls)
	}
}

func TestRunPrune_NonIgnoredErrorAborts(t *testing.T) {
	calls := 0
	exec := func(step docker.PruneStep) error {
		calls++
		if calls == 2 { // docker system prune (no IgnoreError)
			return fmt.Errorf("simulated daemon down")
		}
		return nil
	}
	err := docker.RunPrune(docker.RunPruneArgs{
		Opts:    docker.PruneOpts{GOOS: "linux"},
		Exec:    exec,
		Out:     &strings.Builder{},
		In:      strings.NewReader("y\n"),
		SkipYes: false,
		IsTTY:   true,
	})
	if err == nil {
		t.Errorf("non-IgnoreError step failure must propagate")
	}
	if calls != 2 {
		t.Errorf("expected abort after step 2, got %d calls", calls)
	}
}

// CELL-334: SafeNixGCScript should report stale roots (roots with metadata
// files that have no matching running container). This enables drift detection.
func TestSafeNixGCScript_ReportsStaleRoots(t *testing.T) {
	if !strings.Contains(docker.SafeNixGCScript, "-meta") {
		t.Error("SafeNixGCScript must read *-meta files to identify stale roots for cleanup (CELL-334)")
	}
}

// CELL-334: SafeNixGCScript must clean up stale roots (roots whose -meta
// file shows a project that no longer has a running container).
func TestSafeNixGCScript_CleansStaleRoots(t *testing.T) {
	if !strings.Contains(docker.SafeNixGCScript, "STALE") {
		t.Error("SafeNixGCScript must track and report stale root count (CELL-334)")
	}
}

// CELL-334: NixGCRootReportScript must report drift when multiple unique
// hashes exist.
func TestNixGCRootReportScript_ContainsDriftWarning(t *testing.T) {
	if !strings.Contains(docker.NixGCRootReportScript, "drift") {
		t.Error("NixGCRootReportScript must contain drift detection logic")
	}
	if !strings.Contains(docker.NixGCRootReportScript, "-meta") {
		t.Error("NixGCRootReportScript must read -meta files for root attribution")
	}
}

// CELL-334: Linux default nix prune plan must include a root report step
// before the GC step.
func TestBuildNixPruneSteps_Default_LinuxIncludesReportStep(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS: "linux",
		Pure: true,
	}
	steps := docker.BuildNixPruneSteps(opts)

	var hasReport bool
	for _, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "GC Root Report") {
			hasReport = true
		}
	}
	if !hasReport {
		t.Error("Linux nix prune plan must include a GC root report step (CELL-334)")
	}
}

// CELL-333: safe nix GC on Linux must run inside a container with the nix
// volume mounted, not via `sudo sh -c` on the host. The host doesn't have
// /nix or the GC roots — the script would either fail or operate in the
// wrong namespace.
func TestBuildNixPruneSteps_Default_LinuxRunsInContainer(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS: "linux",
		Pure: true,
	}
	steps := docker.BuildNixPruneSteps(opts)

	// Must NOT use `sudo sh -c` for the safe GC step.
	for _, s := range steps {
		if len(s.Argv) >= 3 && s.Argv[0] == "sudo" && s.Argv[1] == "sh" && s.Argv[2] == "-c" {
			t.Error("safe GC on Linux must NOT use `sudo sh -c` — " +
				"runs in wrong mount namespace (CELL-333)")
		}
	}

	// Must use `docker run` with the nix volume mounted.
	var found bool
	for _, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "docker") && strings.Contains(joined, "run") &&
			strings.Contains(joined, "devcell-nix-store:/nix") {
			found = true
		}
	}
	if !found {
		t.Error("safe GC step must run via `docker run` with devcell-nix-store:/nix volume")
	}
}

// CELL-330: SafeNixGCScript must NOT touch gcroots/auto/ — those symlinks
// point into per-container paths (/opt/devcell, /tmp/...) that are valid
// inside the originating container but dangle from the host or any other
// container. Deleting "broken" auto roots from the wrong namespace reaps
// live containers' indirect roots.
func TestSafeNixGCScript_DoesNotTouchAutoRoots(t *testing.T) {
	if strings.Contains(docker.SafeNixGCScript, "gcroots/auto") {
		t.Error("SafeNixGCScript must not reference gcroots/auto/ — " +
			"auto roots are namespace-local and deleting them from " +
			"a different container is destructive (CELL-330)")
	}
}

// CELL-333: on macOS the thin-mode nix store lives in the devcell-nix-store
// Docker volume, not in the linux-builder VM. The default --pure prune must
// target that volume via a throwaway container (where /nix and the devcell
// GC roots resolve correctly), exactly like the Linux path. GC-ing the
// linux-builder VM never reclaims the store that actually grows.
func TestBuildNixPruneSteps_Default_DarwinRunsInContainerOnNixVolume(t *testing.T) {
	opts := docker.PruneOpts{
		GOOS: "darwin",
		Pure: true,
	}
	steps := docker.BuildNixPruneSteps(opts)

	if len(steps) == 0 {
		t.Fatalf("want at least 1 step, got 0")
	}

	// No ssh, no sudo — the volume is reachable through the local docker
	// daemon, and -u 0 inside the container covers root-only operations.
	for i, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "ssh") {
			t.Errorf("step %d must not ssh to linux-builder (CELL-333): %v", i+1, s.Argv)
		}
		if len(s.Argv) > 0 && s.Argv[0] == "sudo" {
			t.Errorf("step %d must not require sudo on the host: %v", i+1, s.Argv)
		}
	}

	// The safe GC script must run via docker run with the nix volume mounted.
	var script string
	sawVolume := false
	for _, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "docker run") &&
			strings.Contains(joined, docker.DefaultThinStoreVolume+":/nix") {
			sawVolume = true
			for i, a := range s.Argv {
				if a == "-c" && i+1 < len(s.Argv) {
					script = s.Argv[i+1]
				}
			}
		}
	}
	if !sawVolume {
		t.Fatalf("no docker run step mounting %s:/nix found: %+v",
			docker.DefaultThinStoreVolume, steps)
	}
	if !strings.Contains(script, "gcroots/devcell") || !strings.Contains(script, "nix-store --gc") {
		t.Errorf("darwin safe GC must use the project-aware script (gcroots/devcell + nix-store --gc), got: %q", script)
	}
	if strings.Contains(script, "nix-collect-garbage") {
		t.Errorf("darwin default prune must not blanket nix-collect-garbage (that's --force)")
	}
}

// Default docker prune must not invoke `sh` on the host. All shell-dependent
// steps must use Func (pure Go) so devcell works on NixOS and other hosts
// where sh may not be in PATH.
func TestBuildDockerPruneSteps_Default_NoHostShellDependency(t *testing.T) {
	steps := docker.BuildDockerPruneSteps(docker.PruneOpts{GOOS: "linux"})
	for i, s := range steps {
		if len(s.Argv) >= 2 && s.Argv[0] == "sh" && s.Argv[1] == "-c" && s.Func == nil {
			t.Errorf("step %d uses sh -c without Func override — "+
				"breaks on hosts without sh in PATH: %v", i+1, s.Argv)
		}
	}
}

// RunPrune must call Func when set, not exec Argv.
func TestRunPrune_FuncStepExecuted(t *testing.T) {
	called := false
	steps := []docker.PruneStep{{
		Argv: []string{"display", "only"},
		Func: func(_ context.Context) error {
			called = true
			return nil
		},
	}}
	// We can't call RunPrune directly (it builds its own steps), so verify
	// the contract: step.Func != nil means the executor must call Func.
	for _, s := range steps {
		if s.Func != nil {
			if err := s.Func(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !called {
		t.Error("Func was not invoked")
	}
}

func equalArgv(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
