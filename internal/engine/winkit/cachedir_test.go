package winkit

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Inside a cell, $HOME is itself a per-cell directory, so the cache renders as
// ~/.devcell/<cell>/.devcell/cache/qemu and every cell re-downloads the same
// 6 GB of immutable media. There is no way to reach the real host home from
// inside the container, so the cache location has to be pointable.
func TestCacheDir_HonoursAnExplicitOverride(t *testing.T) {
	shared := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", shared)

	require.Equal(t, shared, CacheDir("/home/anyone"))
	require.Equal(t, filepath.Join(shared, "virtio-win.iso"), VirtioISOPath("/home/anyone"))
}

// DEVCELL_QEMU_CACHE_DIR is the deprecated name of DEVCELL_WINKIT_CACHE_DIR:
// still honoured when the new name is unset.
func TestCacheDir_HonoursTheDeprecatedQemuName(t *testing.T) {
	shared := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	t.Setenv("DEVCELL_QEMU_CACHE_DIR", shared)

	require.Equal(t, shared, CacheDir("/home/anyone"))
}

func TestCacheDir_DefaultsUnderHome(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	t.Setenv("DEVCELL_QEMU_CACHE_DIR", "")

	require.Equal(t, filepath.Join("/home/x", ".devcell", "cache", "qemu"), CacheDir("/home/x"))
}
