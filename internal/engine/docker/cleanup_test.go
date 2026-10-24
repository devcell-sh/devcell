package docker_test

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// CELL-334: `cell cleanup` + prune preflight share one primitive — map
// running devcell containers to the volume-resident closures they depend
// on. Retention policy (decided 2026-08-01): "in use" means a RUNNING
// container (docker ps), not any existing one. Stopped cells' closures are
// reapable; recovery is the CELL-38 hydration-gate rebuild path.

func fakeResolve(m map[string]string) func(container, path string) (string, error) {
	return func(container, path string) (string, error) {
		v, ok := m[container+" "+path]
		if !ok {
			return "", errors.New("no such path")
		}
		return v, nil
	}
}

func TestCollectLiveClosures_ResolvesProfileAndGenerationPerContainer(t *testing.T) {
	ps := func() ([]string, error) { return []string{"cell-a", "cell-b"}, nil }
	resolve := fakeResolve(map[string]string{
		"cell-a " + docker.ContainerProfileLink:    "/nix/store/aaa111-home-manager-path",
		"cell-a " + docker.ContainerGenerationLink: "/nix/store/bbb222-home-manager-generation",
		"cell-b " + docker.ContainerProfileLink:    "/nix/store/ccc333-home-manager-path",
		"cell-b " + docker.ContainerGenerationLink: "/nix/store/ddd444-home-manager-generation",
	})

	closures, failures, err := docker.CollectLiveClosures(ps, resolve, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Fatalf("want 0 failures, got %+v", failures)
	}
	if len(closures) != 2 {
		t.Fatalf("want 2 closures, got %d: %+v", len(closures), closures)
	}
	if closures[0].Container != "cell-a" ||
		closures[0].ProfilePath != "/nix/store/aaa111-home-manager-path" ||
		closures[0].GenerationPath != "/nix/store/bbb222-home-manager-generation" {
		t.Errorf("closure[0] wrong: %+v", closures[0])
	}
}

// A running container whose closure cannot be resolved is reported as a
// ClosureFailure, not a hard error — the caller decides whether to prompt
// the user to force-stop it.
func TestCollectLiveClosures_UnresolvableContainerReportedAsFailure(t *testing.T) {
	ps := func() ([]string, error) { return []string{"cell-broken"}, nil }
	resolve := fakeResolve(map[string]string{})

	closures, failures, err := docker.CollectLiveClosures(ps, resolve, nil)
	if err != nil {
		t.Fatalf("infrastructure error must be nil: %v", err)
	}
	if len(closures) != 0 {
		t.Errorf("want 0 closures, got %d", len(closures))
	}
	if len(failures) != 1 {
		t.Fatalf("want 1 failure, got %d", len(failures))
	}
	if failures[0].Container != "cell-broken" {
		t.Errorf("failure must name the container: %+v", failures[0])
	}
}

// When some containers resolve and others don't, closures and failures are
// returned together so the caller can prompt the user about the broken ones.
func TestCollectLiveClosures_PartialFailure_ReturnsBothClosuresAndFailures(t *testing.T) {
	ps := func() ([]string, error) { return []string{"cell-good", "cell-broken"}, nil }
	resolve := fakeResolve(map[string]string{
		"cell-good " + docker.ContainerProfileLink:    "/nix/store/aaa111-home-manager-path",
		"cell-good " + docker.ContainerGenerationLink: "/nix/store/bbb222-home-manager-generation",
	})

	closures, failures, err := docker.CollectLiveClosures(ps, resolve, nil)
	if err != nil {
		t.Fatalf("infrastructure error must be nil: %v", err)
	}
	if len(closures) != 1 || closures[0].Container != "cell-good" {
		t.Errorf("want 1 good closure for cell-good, got %+v", closures)
	}
	if len(failures) != 1 || failures[0].Container != "cell-broken" {
		t.Errorf("want 1 failure for cell-broken, got %+v", failures)
	}
}

func TestCollectLiveClosures_NoRunningContainersIsEmptyNotError(t *testing.T) {
	ps := func() ([]string, error) { return nil, nil }
	closures, failures, err := docker.CollectLiveClosures(ps, fakeResolve(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(closures) != 0 {
		t.Fatalf("want 0 closures, got %+v", closures)
	}
	if len(failures) != 0 {
		t.Fatalf("want 0 failures, got %+v", failures)
	}
}

func TestCollectLiveClosures_LogsDatapoints(t *testing.T) {
	ps := func() ([]string, error) { return []string{"cell-a"}, nil }
	resolve := fakeResolve(map[string]string{
		"cell-a " + docker.ContainerProfileLink:    "/nix/store/aaa111-home-manager-path",
		"cell-a " + docker.ContainerGenerationLink: "/nix/store/bbb222-home-manager-generation",
	})
	var lines []string
	logf := func(format string, args ...any) {
		lines = append(lines, format)
	}
	if _, _, err := docker.CollectLiveClosures(ps, resolve, logf); err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Error("CollectLiveClosures must log datapoints via logf for --debug visibility")
	}
}

// StampRootsScript makes the prune-gate invariant true by construction:
// instead of refusing when a running container has no GC root, stamp the
// (idempotent, hash-named) roots for every live closure before GC runs.
func TestStampRootsScript_StampsHashNamedRootsForEveryLiveClosure(t *testing.T) {
	closures := []docker.LiveClosure{
		{
			Container:      "cell-a",
			ProfilePath:    "/nix/store/aaa111-home-manager-path",
			GenerationPath: "/nix/store/bbb222-home-manager-generation",
		},
	}
	script := docker.StampRootsScript(closures)

	wants := []string{
		"mkdir -p /nix/var/nix/gcroots/devcell",
		`ln -sfT "/nix/store/aaa111-home-manager-path" /nix/var/nix/gcroots/devcell/aaa111-profile`,
		`ln -sfT "/nix/store/bbb222-home-manager-generation" /nix/var/nix/gcroots/devcell/bbb222-generation`,
	}
	for _, w := range wants {
		if !strings.Contains(script, w) {
			t.Errorf("stamp script missing %q\nscript:\n%s", w, script)
		}
	}
	if strings.Contains(script, "rm ") || strings.Contains(script, "nix-store") {
		t.Errorf("stamp script must only create roots, never delete or GC:\n%s", script)
	}
}

func TestStampRootsScript_EmptyClosuresIsEmpty(t *testing.T) {
	if s := docker.StampRootsScript(nil); s != "" {
		t.Errorf("no live closures → empty stamp script, got %q", s)
	}
}

// The reaper: remove devcell/ roots whose hash no RUNNING container's
// closure matches. Only touches /nix/var/nix/gcroots/devcell/ — never
// auto/ roots (namespace-local, CELL-330), never the store itself.
func TestBuildCleanupScript_KeepsLiveReapsStale(t *testing.T) {
	closures := []docker.LiveClosure{
		{
			Container:      "cell-a",
			ProfilePath:    "/nix/store/aaa111-home-manager-path",
			GenerationPath: "/nix/store/bbb222-home-manager-generation",
		},
	}
	script := docker.BuildCleanupScript(closures)

	if !strings.Contains(script, `LIVE=" aaa111 bbb222 "`) {
		t.Errorf("cleanup script must embed the live hash set, got:\n%s", script)
	}
	if !strings.Contains(script, "/nix/var/nix/gcroots/devcell/") {
		t.Errorf("cleanup script must scan gcroots/devcell/:\n%s", script)
	}
	if strings.Contains(script, "gcroots/auto") {
		t.Errorf("cleanup script must never touch auto/ roots (CELL-330):\n%s", script)
	}
	for _, forbidden := range []string{"nix-store", "nix-collect-garbage", "rm -rf /nix/store"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("cleanup script must only reap root symlinks, not GC (%q found):\n%s", forbidden, script)
		}
	}
}

// Running-only semantics: with nothing running, every root is reapable.
// The script must still be valid (empty LIVE set) — `cell cleanup` is
// explicit and confirmation-gated, so this is intended, not a foot-gun.
func TestBuildCleanupScript_NoLiveClosuresReapsAll(t *testing.T) {
	script := docker.BuildCleanupScript(nil)
	if !strings.Contains(script, `LIVE="  "`) && !strings.Contains(script, `LIVE=" "`) {
		t.Errorf("empty live set must produce an empty LIVE list:\n%s", script)
	}
}

func TestBuildCleanupSteps_RunsInContainerOnNixVolume(t *testing.T) {
	steps := docker.BuildCleanupSteps(nil)
	if len(steps) != 1 {
		t.Fatalf("want 1 step, got %d: %+v", len(steps), steps)
	}
	joined := strings.Join(steps[0].Argv, " ")
	if !strings.Contains(joined, "docker run") ||
		!strings.Contains(joined, docker.DefaultThinStoreVolume+":/nix") {
		t.Errorf("cleanup must run in a container mounting %s:/nix, got: %v",
			docker.DefaultThinStoreVolume, steps[0].Argv)
	}
}

// RunCleanup orchestrates: build the reaper plan, prompt, execute. Same
// confirmation semantics as RunPrune — rejection is a clean no-op, non-TTY
// without --yes refuses.
func TestRunCleanup_RejectedPromptExecutesNothing(t *testing.T) {
	executed := 0
	var out strings.Builder
	err := docker.RunCleanup(docker.RunCleanupArgs{
		Closures: nil,
		Exec:     func(docker.PruneStep) error { executed++; return nil },
		Out:      &out,
		In:       strings.NewReader("n\n"),
		IsTTY:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed != 0 {
		t.Errorf("rejected prompt must execute nothing, executed %d step(s)", executed)
	}
}

func TestRunCleanup_AcceptedPromptExecutesReaperStep(t *testing.T) {
	var got []docker.PruneStep
	var out strings.Builder
	err := docker.RunCleanup(docker.RunCleanupArgs{
		Closures: []docker.LiveClosure{
			{
				Container:      "cell-a",
				ProfilePath:    "/nix/store/aaa111-home-manager-path",
				GenerationPath: "/nix/store/bbb222-home-manager-generation",
			},
		},
		Exec:  func(s docker.PruneStep) error { got = append(got, s); return nil },
		Out:   &out,
		In:    strings.NewReader("y\n"),
		IsTTY: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 executed step, got %d", len(got))
	}
	joined := strings.Join(got[0].Argv, " ")
	if !strings.Contains(joined, docker.DefaultThinStoreVolume+":/nix") {
		t.Errorf("executed step must mount the nix volume: %v", got[0].Argv)
	}
}

// The prompt must tell the user the retention rule and how many containers
// are protected — this is the primary non-debug UX for cleanup.
func TestBuildCleanupPrompt_NamesRetentionRuleAndLiveCount(t *testing.T) {
	closures := []docker.LiveClosure{
		{Container: "cell-a", ProfilePath: "/nix/store/aaa111-p", GenerationPath: "/nix/store/bbb222-g"},
		{Container: "cell-b", ProfilePath: "/nix/store/aaa111-p", GenerationPath: "/nix/store/bbb222-g"},
	}
	prompt := docker.BuildCleanupPrompt(closures)
	for _, want := range []string{"running", "2 running container", "Continue? [y/N]"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("cleanup prompt missing %q\nprompt:\n%s", want, prompt)
		}
	}
}

func TestBuildCleanupPrompt_WarnsWhenNothingRunning(t *testing.T) {
	prompt := docker.BuildCleanupPrompt(nil)
	if !strings.Contains(prompt, "ALL") {
		t.Errorf("with nothing running the prompt must warn that ALL roots are reaped:\n%s", prompt)
	}
}

// CollectLiveClosures must resolve containers concurrently — with many
// running cells, serial docker exec calls compound into minutes of wall time.
func TestCollectLiveClosures_ResolvesContainersConcurrently(t *testing.T) {
	const N = 10
	var names []string
	for i := range N {
		names = append(names, fmt.Sprintf("cell-%d", i))
	}
	ps := func() ([]string, error) { return names, nil }

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	resolve := func(container, link string) (string, error) {
		n := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if n <= old || maxConcurrent.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		concurrent.Add(-1)
		if strings.HasSuffix(link, "profile") {
			return "/nix/store/aaa-home-manager-path", nil
		}
		return "/nix/store/bbb-home-manager-generation", nil
	}

	start := time.Now()
	closures, failures, err := docker.CollectLiveClosures(ps, resolve, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Fatalf("want 0 failures, got %+v", failures)
	}
	if len(closures) != N {
		t.Fatalf("want %d closures, got %d", N, len(closures))
	}
	if maxConcurrent.Load() < 2 {
		t.Errorf("resolve calls must run concurrently; max concurrent was %d", maxConcurrent.Load())
	}
	// Serial would take at least N * 2 * 10ms = 200ms. Parallel should be well under.
	if elapsed > 150*time.Millisecond {
		t.Errorf("resolution took %v — should be parallel, not serial", elapsed)
	}
}

// CELL-334 secondary: PROTECTED must be collected from BOTH *-profile and
// *-generation roots — generations carry home-manager-files, the very
// closure CELL-320 exists to protect.
func TestSafeNixGCScript_CollectsProtectedFromBothRootKinds(t *testing.T) {
	if !strings.Contains(docker.SafeNixGCScript,
		"/nix/var/nix/gcroots/devcell/*-profile /nix/var/nix/gcroots/devcell/*-generation") {
		t.Error("SafeNixGCScript must collect PROTECTED from both *-profile and *-generation globs (CELL-334)")
	}
}

// BuildStopBrokenPrompt must name the broken containers and ask the user
// whether to force-stop them so the operation can proceed.
func TestBuildStopBrokenPrompt_NamesContainersAndAsksToStop(t *testing.T) {
	failures := []docker.ClosureFailure{
		{Container: "cell-broken-1", Err: errors.New("exit status 127")},
		{Container: "cell-broken-2", Err: errors.New("readlink failed")},
	}
	prompt := docker.BuildStopBrokenPrompt(failures)
	for _, want := range []string{"cell-broken-1", "cell-broken-2", "stop", "Continue? [y/N]"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\nprompt:\n%s", want, prompt)
		}
	}
}

// The prune gate: when live closures are supplied, the plan must stamp
// roots for them (making "running implies rooted" true) before the safe GC
// step. Order matters — stamp, then GC.
func TestBuildNixPruneSteps_LiveClosuresInsertStampStepBeforeGC(t *testing.T) {
	closures := []docker.LiveClosure{
		{
			Container:      "cell-a",
			ProfilePath:    "/nix/store/aaa111-home-manager-path",
			GenerationPath: "/nix/store/bbb222-home-manager-generation",
		},
	}
	opts := docker.PruneOpts{GOOS: "linux", Pure: true, LiveClosures: closures}
	steps := docker.BuildNixPruneSteps(opts)

	stampIdx, gcIdx := -1, -1
	for i, s := range steps {
		joined := strings.Join(s.Argv, " ")
		if strings.Contains(joined, "aaa111-profile") {
			stampIdx = i
		}
		if strings.Contains(joined, "nix-store --gc") {
			gcIdx = i
		}
	}
	if stampIdx == -1 {
		t.Fatalf("no stamp step found in plan: %+v", steps)
	}
	if gcIdx == -1 {
		t.Fatalf("no safe GC step found in plan: %+v", steps)
	}
	if stampIdx >= gcIdx {
		t.Errorf("stamp step (%d) must run before safe GC (%d)", stampIdx, gcIdx)
	}
}
