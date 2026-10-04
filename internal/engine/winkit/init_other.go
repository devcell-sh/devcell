//go:build !(darwin || linux)

package winkit

import (
	"context"
	"fmt"
	"runtime"

	"github.com/DimmKirr/devcell/internal/engine"
)

// Init is unavailable off macOS and Linux: go-winkit builds need either.
func (Engine) Init(context.Context, engine.InitOpts) error {
	return fmt.Errorf("cell init --engine winkit requires macOS on Apple Silicon (current: %s/%s)", runtime.GOOS, runtime.GOARCH)
}
