//go:build !(darwin && arm64)

package tart

import (
	"context"
	"fmt"
	"runtime"

	"github.com/DimmKirr/devcell/internal/engine"
)

// Init needs darwin/arm64.
func (Engine) Init(context.Context, engine.InitOpts) error {
	return fmt.Errorf("cell init --engine=tart requires macOS on Apple Silicon (current: %s/%s)", runtime.GOOS, runtime.GOARCH)
}

// build needs darwin/arm64.
func build(context.Context, buildParams) error {
	return fmt.Errorf("cell build --engine=tart requires macOS on Apple Silicon (darwin/arm64)")
}
