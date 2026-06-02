package runner

import (
	"context"
	"errors"
	"fmt"
)

// AcquireDeps bundles the side-effectful work AcquireImage needs to satisfy
// each effective LaunchAction in the sequence returned by
// DecideLaunchActionsPure. UseLocal and DryRun are no-ops at this layer and
// are handled internally — callers only supply the four closures that may
// fail.
type AcquireDeps struct {
	Inputs      LaunchInputs
	PullPure    func(context.Context) error
	PullImpure  func(context.Context) error
	BuildPure   func(context.Context) error
	BuildImpure func(context.Context) error
}

// AcquireImage walks the action sequence, invoking the matching closure for
// each effective action and stopping at the first one that returns nil.
// If every action fails the returned error joins all attempts so the user
// sees the full chain.
func AcquireImage(ctx context.Context, d AcquireDeps) error {
	actions := DecideLaunchActionsPure(d.Inputs)
	var errs []error
	for _, a := range actions {
		fn := d.fnFor(a)
		if fn == nil {
			// UseLocal / DryRun fall through here — no work to do, success.
			return nil
		}
		if err := fn(ctx); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Errorf("%s: %w", actionName(a), err))
		}
	}
	return fmt.Errorf("image acquisition failed after %d attempt(s): %w",
		len(actions), errors.Join(errs...))
}

func (d AcquireDeps) fnFor(a LaunchAction) func(context.Context) error {
	switch a {
	case ActionUseLocal, ActionDryRun:
		return nil
	case ActionPullPure:
		return d.PullPure
	case ActionPullImpure:
		return d.PullImpure
	case ActionBuildPure:
		return d.BuildPure
	case ActionBuildImpure:
		return d.BuildImpure
	}
	return nil
}

func actionName(a LaunchAction) string {
	switch a {
	case ActionUseLocal:
		return "use-local"
	case ActionDryRun:
		return "dry-run"
	case ActionPullPure:
		return "pull-pure"
	case ActionPullImpure:
		return "pull-impure"
	case ActionBuildPure:
		return "build-pure"
	case ActionBuildImpure:
		return "build-impure"
	}
	return "unknown"
}
