package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/runner"
)

// AcquireImage walks runner.DecideLaunchActionsPure's sequence at runtime:
// it tries each effective action in order, returns nil at the first success,
// and on the final action's failure surfaces a chain error so the user sees
// every fallback that was attempted.
//
// Tests inject fake closures for each effective action via runner.AcquireDeps
// so we can exercise the orchestration logic without docker or nix.

type recordingDeps struct {
	calls   []string
	results map[string]error
}

func (r *recordingDeps) record(name string) error {
	r.calls = append(r.calls, name)
	if err, ok := r.results[name]; ok {
		return err
	}
	return nil
}

func newRecordingDeps(in runner.LaunchInputs, results map[string]error) (*recordingDeps, runner.AcquireDeps) {
	rec := &recordingDeps{results: results}
	return rec, runner.AcquireDeps{
		Inputs:      in,
		PullPure:    func(context.Context) error { return rec.record("pull-pure") },
		PullImpure:  func(context.Context) error { return rec.record("pull-impure") },
		BuildPure:   func(context.Context) error { return rec.record("build-pure") },
		BuildImpure: func(context.Context) error { return rec.record("build-impure") },
	}
}

func TestAcquireImage_LocalExists_NoClosuresInvoked(t *testing.T) {
	rec, deps := newRecordingDeps(runner.LaunchInputs{LocalExists: true, HasNix: true}, nil)
	if err := runner.AcquireImage(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("expected no closure invocations for use-local, got: %v", rec.calls)
	}
}

func TestAcquireImage_DryRun_NoClosuresInvoked(t *testing.T) {
	rec, deps := newRecordingDeps(runner.LaunchInputs{DryRun: true, HasNix: true}, nil)
	if err := runner.AcquireImage(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("expected no closure invocations for dry-run, got: %v", rec.calls)
	}
}

func TestAcquireImage_ColdStart_NixHost_PullPureSucceeds_StopsAtFirstAction(t *testing.T) {
	rec, deps := newRecordingDeps(runner.LaunchInputs{HasNix: true}, nil)
	if err := runner.AcquireImage(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"pull-pure"}
	if !equalStrings(rec.calls, want) {
		t.Errorf("calls = %v, want %v", rec.calls, want)
	}
}

func TestAcquireImage_ColdStart_PurePullFails_FallsBackToImpurePull(t *testing.T) {
	rec, deps := newRecordingDeps(runner.LaunchInputs{HasNix: true}, map[string]error{
		"pull-pure": errors.New("registry: pure tag missing"),
	})
	if err := runner.AcquireImage(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"pull-pure", "pull-impure"}
	if !equalStrings(rec.calls, want) {
		t.Errorf("calls = %v, want %v", rec.calls, want)
	}
}

func TestAcquireImage_ColdStart_NoNix_BothPullsFail_FallsBackToImpureBuild(t *testing.T) {
	rec, deps := newRecordingDeps(runner.LaunchInputs{HasNix: false}, map[string]error{
		"pull-pure":   errors.New("registry unreachable"),
		"pull-impure": errors.New("registry unreachable"),
	})
	if err := runner.AcquireImage(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"pull-pure", "pull-impure", "build-impure"}
	if !equalStrings(rec.calls, want) {
		t.Errorf("calls = %v, want %v", rec.calls, want)
	}
}

func TestAcquireImage_AllFailures_ReturnsChainError(t *testing.T) {
	_, deps := newRecordingDeps(runner.LaunchInputs{HasNix: true}, map[string]error{
		"pull-pure":   errors.New("pure-pull-failed-msg"),
		"pull-impure": errors.New("impure-pull-failed-msg"),
		"build-pure":  errors.New("nix-build-failed-msg"),
	})
	err := runner.AcquireImage(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{"pure-pull-failed-msg", "impure-pull-failed-msg", "nix-build-failed-msg"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q so user sees the full attempt chain", err.Error(), want)
		}
	}
}

func TestAcquireImage_BuildFailureSurfacedToCaller(t *testing.T) {
	// Lock in fix #1 from the self-review: BuildPure must actually run inside
	// the closure. If it fails, AcquireImage returns the error — it does not
	// silently succeed and defer the failure to a later, out-of-chain block.
	_, deps := newRecordingDeps(runner.LaunchInputs{ExplicitBuild: true, HasNix: true}, map[string]error{
		"build-pure": errors.New("nix-build-blew-up"),
	})
	err := runner.AcquireImage(context.Background(), deps)
	if err == nil {
		t.Fatal("expected build error, got nil")
	}
	if !strings.Contains(err.Error(), "nix-build-blew-up") {
		t.Errorf("error should surface the build failure verbatim; got: %v", err)
	}
}

func equalStrings(a, b []string) bool {
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
