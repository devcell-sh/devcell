//go:build darwin || linux

package winkit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/engine"
)

// Keys lived at ~/.devcell/<cell>/qemu/, but .ssh is where anyone looks first.
// The engine name in the path was an accident of which engine happened to
// need keys first.
func TestQemuKeyDir_PrefersDotSSH(t *testing.T) {
	home := t.TempDir()

	dir := keyDir(home, "DIMM")

	if want := filepath.Join(home, ".devcell", "DIMM", ".ssh"); dir != want {
		t.Errorf("new cells must use %s, got %s", want, dir)
	}
}

// A template already built has the old key baked into the guest. Moving the
// path must not orphan it — that would silently turn a 3-hour template into
// one nothing can log into.
func TestQemuKeyDir_KeepsUsingTheLegacyPathWhenAKeyIsAlreadyThere(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".devcell", "DIMM", "qemu")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "id_ed25519"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if dir := keyDir(home, "DIMM"); dir != legacy {
		t.Errorf("an existing key must keep its path, got %s", dir)
	}
}

// Two cells on the same stack with different module sets must not share a
// template dir, or they share one boot image: the first build wins and
// the second either reuses an image missing its modules or, with --force,
// destroys the first. StackTag exists precisely to keep them apart.
func TestQemuTemplatePaths_SeparateTemplatesPerModuleSet(t *testing.T) {
	home := t.TempDir()

	bare := templateDir(home, guestPE, "base", nil)
	withMods := templateDir(home, guestPE, "base", []string{"docker", "node"})

	if bare == withMods {
		t.Fatalf("module sets must not share a template dir: both resolved to %s", bare)
	}
	if imagePath(home, guestPE, "base", nil) == imagePath(home, guestPE, "base", []string{"docker", "node"}) {
		t.Error("module sets must not share a boot image")
	}
}

// Module order is not meaningful, so it must not fork the template.
func TestQemuTemplatePaths_ModuleOrderDoesNotMatter(t *testing.T) {
	home := t.TempDir()

	if a, b := templateDir(home, guestPE, "base", []string{"node", "docker"}),
		templateDir(home, guestPE, "base", []string{"docker", "node"}); a != b {
		t.Errorf("the same modules in a different order must be one template: %s vs %s", a, b)
	}
}

// InitOpts.Force regenerates the cell's SSH keypair and downloads the VirtIO
// drivers again; without it, both are kept.
func TestInit_ForceRegeneratesKeysAndDrivers(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("force=%v", force), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			c := engine.Cell{Name: "main", HostHome: home, Stack: "base"}

			key := filepath.Join(keyDir(home, c.Name), "id_ed25519")
			require.NoError(t, os.MkdirAll(filepath.Dir(key), 0o700))
			require.NoError(t, os.WriteFile(key, []byte("old key"), 0o600))
			stubInitMedia(t, home, func() {
				_, err := os.Stat(VirtioISOPath(home))
				assert.Equal(t, force, os.IsNotExist(err), "cached VirtIO ISO discarded")
			})

			require.NoError(t, Engine{}.Init(context.Background(), engine.InitOpts{Cell: c, Force: force}))

			got, err := os.ReadFile(key)
			require.NoError(t, err)
			assert.Equal(t, force, string(got) != "old key", "keypair regenerated")
		})
	}
}

func TestInit_CreatesTheGuestsTemplateDir(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubInitMedia(t, home, func() {})

	c := engine.Cell{Name: "main", HostHome: home, Stack: "base", Guest: engine.WindowsFull}
	require.NoError(t, Engine{}.Init(context.Background(), engine.InitOpts{Cell: c}))

	assert.DirExists(t, templateDir(home, guestFull, "base", nil))
	assert.DirExists(t, InstanceDir(home, "main"))
}

// --stack prepares that stack's template dir instead of the cell's
// resolved one.
func TestInit_StackOverridesCellStack(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubInitMedia(t, home, func() {})

	c := engine.Cell{Name: "main", HostHome: home, Stack: "base"}
	require.NoError(t, Engine{}.Init(context.Background(), engine.InitOpts{Cell: c, Stack: "go"}))

	assert.DirExists(t, templateDir(home, guestPE, "go", nil))
	assert.NoDirExists(t, templateDir(home, guestPE, "base", nil))
}

// stubInitMedia serves Init's downloads from a planted cache: a cached
// VirtIO ISO, and complete Windows media from winkit's MCT lane. onVirtio
// runs when Init fetches the VirtIO drivers.
func stubInitMedia(t *testing.T, home string, onVirtio func()) {
	t.Helper()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	virtio := VirtioISOPath(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(virtio), 0o755))
	require.NoError(t, os.WriteFile(virtio, []byte("cached"), 0o644))
	fakeFetchers{
		virtio: func(context.Context, media.FetchOptions) (string, error) {
			onVirtio()
			return virtio, nil
		},
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{Path: filepath.Join(CacheDir(home), "win.iso")}, nil
		},
		full: uncachedFullMedia(t),
	}.install(t)
}
