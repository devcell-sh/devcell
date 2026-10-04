//go:build !(darwin || linux)

package winkit

import (
	"context"
	"fmt"
	"runtime"

	"github.com/DimmKirr/devcell/internal/engine"
)

// Build is unavailable off macOS and Linux: go-winkit builds need either.
func (Engine) Build(context.Context, engine.BuildOpts) error {
	return fmt.Errorf("cell build --engine winkit requires macOS or Linux (current: %s/%s)", runtime.GOOS, runtime.GOARCH)
}
