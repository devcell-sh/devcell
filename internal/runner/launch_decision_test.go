package runner_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/runner"
)

// DecideLaunchActionsPure returns the ordered fallback sequence the launcher
// walks to acquire the pure image. Each row exercises one branch of the
// decision tree.
//
// Sequences:
//
//	LocalExists=true             → [UseLocal]
//	DryRun=true                  → [DryRun]
//	ExplicitBuild + HasNix       → [BuildPure]
//	ExplicitBuild + no nix       → [BuildImpure]
//	cold start + HasNix          → [PullPure, PullImpure, BuildPure]
//	cold start + no nix          → [PullPure, PullImpure, BuildImpure]
//
// Staleness is intentionally NOT consulted: pure images are content-addressed,
// so a local tag is byte-identical to what a rebuild would produce from the
// same flake.lock.

func TestDecideLaunchActionsPure(t *testing.T) {
	tests := []struct {
		name string
		in   runner.LaunchInputs
		want []runner.LaunchAction
	}{
		{
			name: "local exists → use local (single action)",
			in:   runner.LaunchInputs{LocalExists: true, HasNix: true},
			want: []runner.LaunchAction{runner.ActionUseLocal},
		},
		{
			name: "dry-run trumps everything",
			in:   runner.LaunchInputs{DryRun: true, ExplicitBuild: true, LocalExists: false, HasNix: true},
			want: []runner.LaunchAction{runner.ActionDryRun},
		},
		{
			name: "--build with host nix → pure build only",
			in:   runner.LaunchInputs{ExplicitBuild: true, HasNix: true},
			want: []runner.LaunchAction{runner.ActionBuildPure},
		},
		{
			name: "--build without host nix → impure build (docker build, nix runs inside)",
			in:   runner.LaunchInputs{ExplicitBuild: true, HasNix: false},
			want: []runner.LaunchAction{runner.ActionBuildImpure},
		},
		{
			name: "cold start with host nix → pull pure → pull impure → build pure",
			in:   runner.LaunchInputs{HasNix: true},
			want: []runner.LaunchAction{runner.ActionPullPure, runner.ActionPullImpure, runner.ActionBuildPure},
		},
		{
			name: "cold start without host nix → pull pure → pull impure → build impure",
			in:   runner.LaunchInputs{HasNix: false},
			want: []runner.LaunchAction{runner.ActionPullPure, runner.ActionPullImpure, runner.ActionBuildImpure},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.DecideLaunchActionsPure(tt.in)
			if !equalActions(got, tt.want) {
				t.Errorf("DecideLaunchActionsPure(%+v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func equalActions(a, b []runner.LaunchAction) bool {
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
