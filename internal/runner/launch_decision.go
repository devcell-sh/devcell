package runner

import (
	"context"
	"fmt"
	"os/exec"
)

// LaunchAction is one image-acquisition step the launcher may run before
// exec'ing the container. DecideLaunchActionsPure orders them into a
// fallback sequence; AcquireImage walks that sequence at runtime.
type LaunchAction int

const (
	// ActionUseLocal: image is present locally; exec immediately.
	ActionUseLocal LaunchAction = iota

	// ActionDryRun: no image work; render argv only.
	ActionDryRun

	// ActionPullPure: pull the pure (nix2container) registry tag for the
	// active stack and retag locally as UserImageTagPure().
	ActionPullPure

	// ActionPullImpure: pull the impure (Dockerfile-built) registry tag and
	// retag locally so the next LocalExists check finds it. Used as the
	// second fallback when ActionPullPure fails — the impure tag carries
	// the same effective contents but is reachable by hosts without nix.
	ActionPullImpure

	// ActionBuildPure: build via nix2container against the resolved nixhome
	// flake. Requires a usable host nix (and on macOS, a Linux remote
	// builder — see PreflightNixBuilder).
	ActionBuildPure

	// ActionBuildImpure: build via `docker build` against the scaffolded
	// Dockerfile. Nix runs inside the build, so the host does not need a
	// nix binary. Final fallback for nix-less hosts.
	ActionBuildImpure
)

// LaunchInputs are the inputs to DecideLaunchActionsPure.
type LaunchInputs struct {
	// DryRun is true when --dry-run is set. Highest precedence.
	DryRun bool

	// ExplicitBuild is true when --build is set. Forces a rebuild regardless
	// of local image presence.
	ExplicitBuild bool

	// LocalExists is true when the locally-tagged pure image
	// (UserImageTagPure()) is present in the Docker daemon.
	LocalExists bool

	// HasNix is true when the host can usefully run a pure nix build —
	// nix is on PATH AND (on macOS) a Linux remote builder is configured.
	// When false the decision skips ActionBuildPure and prefers
	// ActionBuildImpure so docker can complete the build instead.
	HasNix bool
}

// DecideLaunchActionsPure returns the ordered fallback sequence AcquireImage
// should walk. The first action that succeeds wins; on the last action's
// failure AcquireImage surfaces a chain error.
//
// Sequences:
//
//	LocalExists           → [UseLocal]
//	DryRun                → [DryRun]
//	ExplicitBuild+HasNix  → [BuildPure]
//	ExplicitBuild+no nix  → [BuildImpure]
//	cold start + HasNix   → [PullPure, PullImpure, BuildPure]
//	cold start + no nix   → [PullPure, PullImpure, BuildImpure]
func DecideLaunchActionsPure(in LaunchInputs) []LaunchAction {
	switch {
	case in.DryRun:
		return []LaunchAction{ActionDryRun}
	case in.ExplicitBuild:
		if in.HasNix {
			return []LaunchAction{ActionBuildPure}
		}
		return []LaunchAction{ActionBuildImpure}
	case in.LocalExists:
		return []LaunchAction{ActionUseLocal}
	default:
		if in.HasNix {
			return []LaunchAction{ActionPullPure, ActionPullImpure, ActionBuildPure}
		}
		return []LaunchAction{ActionPullPure, ActionPullImpure, ActionBuildImpure}
	}
}

// PullAndTagPure pulls the registry's pure image for <stack> and re-tags it
// as UserImageTagPure() so the local-exists check finds it on the next launch.
//
// Returns nil on success. Either step failing returns the underlying error
// — the launcher is expected to treat any error as "fall back to build".
func PullAndTagPure(ctx context.Context, stack string, verbose bool) error {
	remote := StackImageTagPure(stack)
	local := UserImageTagPure()
	if err := PullImage(ctx, remote, verbose); err != nil {
		return fmt.Errorf("pull %s: %w", remote, err)
	}
	if err := exec.CommandContext(ctx, "docker", "tag", remote, local).Run(); err != nil {
		return fmt.Errorf("tag %s as %s: %w", remote, local, err)
	}
	return nil
}

// PullAndTagImpure pulls the registry's impure (Dockerfile-built) image for
// <stack> and tags it under BOTH UserImageTag() (so an explicit --impure
// caller finds it) AND UserImageTagPure() (so the next default-path launch's
// LocalExists check is satisfied without re-pulling). Carrying the second
// tag is what lets the impure-pull truly serve as a fallback for the pure
// path on nix-less hosts.
func PullAndTagImpure(ctx context.Context, stack string, verbose bool) error {
	remote := StackImageTagImpure(stack)
	local := UserImageTag()
	pureLocal := UserImageTagPure()
	if err := PullImage(ctx, remote, verbose); err != nil {
		return fmt.Errorf("pull %s: %w", remote, err)
	}
	if err := exec.CommandContext(ctx, "docker", "tag", remote, local).Run(); err != nil {
		return fmt.Errorf("tag %s as %s: %w", remote, local, err)
	}
	if err := exec.CommandContext(ctx, "docker", "tag", remote, pureLocal).Run(); err != nil {
		return fmt.Errorf("tag %s as %s: %w", remote, pureLocal, err)
	}
	return nil
}
