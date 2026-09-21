//go:build darwin && arm64 && !cgo

package tart

import "image"

// FindTextOnScreen without cgo cannot reach the Apple Vision framework
// (vnc_ocr_darwin.go imports "C" and is excluded when CGO_ENABLED=0, e.g.
// cross-compiles from Linux). Reports "not found" like the non-Darwin stub.
func FindTextOnScreen(_ *image.RGBA, _ string) (image.Rectangle, bool) {
	return image.Rectangle{}, false
}
