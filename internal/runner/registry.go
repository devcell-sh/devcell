package runner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
)

// EphemeralRegistry is a transient OCI registry backed by a filesystem blob
// store. It enables layer-level dedup when loading nix2container images into
// the Docker daemon: unchanged layers are served from disk cache instead of
// being re-copied every build.
type EphemeralRegistry struct {
	Port     int
	cacheDir string
	server   *http.Server
	listener net.Listener
}

// Start launches the registry on a random localhost port. cacheDir is created
// if it doesn't exist and used to persist blobs across invocations.
func (r *EphemeralRegistry) Start(cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("registry cache dir: %w", err)
	}
	r.cacheDir = cacheDir

	handler := registry.New(
		registry.WithBlobHandler(registry.NewDiskBlobHandler(cacheDir)),
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("registry listen: %w", err)
	}
	r.listener = ln
	r.Port = ln.Addr().(*net.TCPAddr).Port

	r.server = &http.Server{Handler: handler}
	go r.server.Serve(ln) //nolint:errcheck
	return nil
}

// Stop gracefully shuts down the registry with a 5s timeout.
func (r *EphemeralRegistry) Stop() error {
	if r.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.server.Shutdown(ctx)
}

// Addr returns the host:port string for use in docker:// and skopeo references.
func (r *EphemeralRegistry) Addr() string {
	return fmt.Sprintf("localhost:%d", r.Port)
}

// registryCacheDir returns the default blob cache path (~/.devcell/registry/).
func registryCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devcell", "registry")
}
