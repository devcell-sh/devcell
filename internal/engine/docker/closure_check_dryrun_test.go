package docker

import (
	"context"
	"testing"
)

// noopPhaser satisfies ux.Phaser without rendering anything: the closure
// check never opens a phase row, so the fake only has to exist.
type noopPhaser struct{}

func (noopPhaser) Phase(_ string, fn func() error) error { return fn() }
func (noopPhaser) PhaseDetailed(_ string, fn func() (string, error)) error {
	_, err := fn()
	return err
}
func (noopPhaser) PhaseDetailedWarn(_ string, fn func() (string, bool, error)) error {
	_, _, err := fn()
	return err
}
func (noopPhaser) PhaseDetailedRunning(_, _ string, fn func() (string, error)) error {
	_, err := fn()
	return err
}
func (noopPhaser) PhaseDetailedRunningWarn(_, _ string, fn func() (string, bool, error)) error {
	_, _, err := fn()
	return err
}
func (noopPhaser) UpdateText(string) {}
func (noopPhaser) Seal(string)       {}

// A dry run prints the docker argv and must not touch the daemon beyond
// read-only probes. Image acquisition already skips in dry-run mode, so on
// a host that has never built the thin image (a fresh CI runner, a new
// machine) the image does not exist when the closure check runs. Probing
// it with `docker run` then fails, which the check used to read as "the
// closure was garbage collected" and answer with a full rebuild, turning
// every `--dry-run` into an image build. The check must stand down in
// dry-run mode instead.
func TestClosureCheckPhase_DryRunNeverRebuilds(t *testing.T) {
	rebuilt := false
	err := closureCheckPhase(context.Background(), noopPhaser{}, true,
		"localhost/devcell-user:closure-check-dry-run-test-thin", true,
		func() error {
			rebuilt = true
			return nil
		})
	if err != nil {
		t.Fatalf("dry-run closure check returned error: %v", err)
	}
	if rebuilt {
		t.Fatal("dry-run closure check triggered a rebuild")
	}
}
