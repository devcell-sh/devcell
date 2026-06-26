package nixstore

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Push streams an uncompressed tar from r through to dstRef as a
// single OCI tar+gzip layer atop baseRef.
//
// Streaming guarantees:
//
//   - Bytes flow: r → gzip writer → registry chunked upload. No disk
//     buffer, no temp file. Peak memory is the gzip window (~32 KB).
//   - Digest is computed incrementally during the upload (via
//     `pkg/v1/stream.NewLayer`), then finalized in the PUT-with-digest
//     call that completes the OCI chunked upload — the registry
//     protocol doesn't require the digest upfront.
//   - The on-wire layer is SINGLE-gzipped. `stream.NewLayer` treats
//     its input as uncompressed and gzips exactly once, regardless of
//     what bytes the caller provides. The caller MUST provide raw
//     (uncompressed) tar — pre-gzipped input would round-trip through
//     a redundant gzip layer.
//
// Why this exists: `crane append --new_layer -` produced double-gzipped
// layers when fed `tar -czf -` because crane's stdin handler doesn't
// detect gzip magic; the CLI workaround was to stage a tarball file
// (which crane DOES detect). This function bypasses both surprises by
// using the underlying go-containerregistry library directly.
func Push(ctx context.Context, baseRef, dstRef string, r io.ReadCloser) error {
	dst, err := name.ParseReference(dstRef)
	if err != nil {
		return fmt.Errorf("parse dst %q: %w", dstRef, err)
	}
	base, err := name.ParseReference(baseRef)
	if err != nil {
		return fmt.Errorf("parse base %q: %w", baseRef, err)
	}

	baseImg, err := remote.Image(base,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	)
	if err != nil {
		return fmt.Errorf("fetch base %q: %w", baseRef, err)
	}

	// stream.NewLayer wraps r in a Layer whose Compressed() returns a
	// reader that lazily reads r, gzips on the fly, and writes to the
	// registry's chunked upload. The OCI media type is set explicitly
	// so the resulting layer is interoperable with OCI-aware clients.
	layer := stream.NewLayer(r, stream.WithMediaType(types.OCILayer))

	img, err := mutate.AppendLayers(baseImg, layer)
	if err != nil {
		return fmt.Errorf("append layer: %w", err)
	}

	if err := remote.Write(dst, img,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	); err != nil {
		return fmt.Errorf("push %q: %w", dstRef, err)
	}
	return nil
}
