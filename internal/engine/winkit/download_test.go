package winkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/media"
	"github.com/devcell-sh/go-winkit/media/isokit"
	"github.com/devcell-sh/go-winkit/media/uupdump"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateISO_RejectsNonISO(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "fake.iso")
	require.NoError(t, os.WriteFile(path, make([]byte, 0x9000), 0644))

	err := ValidateISO(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a recognised disc image")
}

func TestValidateISO_RejectsSmallFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tiny.iso")
	require.NoError(t, os.WriteFile(path, []byte("tiny"), 0644))

	err := ValidateISO(path)
	assert.Error(t, err)
}

func TestValidateISO_AcceptsValidMagic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "valid.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "CD001")
	require.NoError(t, os.WriteFile(path, data, 0644))

	assert.NoError(t, ValidateISO(path))
}

func TestValidateISO_AcceptsUDF(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "udf.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "BEA01")
	require.NoError(t, os.WriteFile(path, data, 0644))

	assert.NoError(t, ValidateISO(path))
}

func TestResolveWindowsISO_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "win.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "CD001")
	require.NoError(t, os.WriteFile(isoPath, data, 0644))

	result, err := ResolveWindowsISO(isoPath, "/some/toml/path.iso", "")
	require.NoError(t, err)
	assert.Equal(t, isoPath, result)
}

func TestResolveWindowsISO_FallsBackToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "win.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "CD001")
	require.NoError(t, os.WriteFile(isoPath, data, 0644))

	result, err := ResolveWindowsISO("", isoPath, "")
	require.NoError(t, err)
	assert.Equal(t, isoPath, result)
}

func TestResolveWindowsISO_FallsBackToCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	cached := filepath.Join(t.TempDir(), "mct.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "CD001")
	require.NoError(t, os.WriteFile(cached, data, 0644))

	fakeFetchers{
		windows: func(ctx context.Context, opts media.FetchOptions) (media.FetchResult, error) {
			require.Error(t, ctx.Err(), "cell build must look the cache up without downloading")
			assert.Equal(t, CacheDir(home), opts.CacheDir)
			return media.FetchResult{Path: cached, Source: media.SourceMCT}, nil
		},
		full: uncachedFullMedia(t),
	}.install(t)

	result, err := ResolveWindowsISO("", "", home)
	require.NoError(t, err)
	assert.Equal(t, cached, result)
}

// cell build must never start the multi-gigabyte download cell init owns:
// every fetcher the cache lookup reaches runs under a canceled context, and
// nothing is written to the cache.
func TestResolveWindowsISO_CacheLookupNeverDownloads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		windows: func(ctx context.Context, _ media.FetchOptions) (media.FetchResult, error) {
			require.Error(t, ctx.Err())
			return media.FetchResult{}, ctx.Err()
		},
		full: func(ctx context.Context, _ uupdump.FetchConfig) (string, error) {
			require.Error(t, ctx.Err())
			return "", ctx.Err()
		},
	}.install(t)

	_, err := ResolveWindowsISO("", "", home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cell init --engine winkit")
	assert.NoDirExists(t, fullMediaDir(CacheDir(home)))
}

// Boot-only media has no sources/install.wim, which the PE+WSL1 build
// extracts, so a cached boot-only ISO is not a usable cache hit.
func TestResolveWindowsISO_IgnoresBootOnlyMedia(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	bootOnly := filepath.Join(t.TempDir(), "boot-only.iso")
	data := make([]byte, 0x9000)
	copy(data[0x8001:], "CD001")
	require.NoError(t, os.WriteFile(bootOnly, data, 0644))

	fakeFetchers{
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{Path: bootOnly, Source: media.SourceUUPDump, BootOnly: true}, nil
		},
		full: uncachedFullMedia(t),
	}.install(t)

	_, err := ResolveWindowsISO("", "", home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cell init --engine winkit")
}

// The tests below run the real winkit fetchers against a seeded cache. The
// canceled context keeps them off the network, and they catch winkit
// renaming a cache file out from under the lookup.

func TestResolveWindowsISO_FindsCachedMCTMedia(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	cached := filepath.Join(CacheDir(home), "windows-11-mct-arm64-en-us.iso")
	writeBootableISO(t, cached)

	result, err := ResolveWindowsISO("", "", home)
	require.NoError(t, err)
	assert.Equal(t, cached, result)
}

func TestResolveWindowsISO_FindsAssembledCompleteMedia(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	cached := filepath.Join(fullMediaDir(CacheDir(home)), uupdumpISOName(t))
	writeBootableISO(t, cached)

	result, err := ResolveWindowsISO("", "", home)
	require.NoError(t, err)
	assert.Equal(t, cached, result)
}

// winkit's boot-only UUP dump ISO and devcell's complete one share a file
// name, so the boot-only one must never be picked up as complete media.
func TestResolveWindowsISO_NeverServesWinkitBootOnlyMedia(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	writeBootableISO(t, filepath.Join(CacheDir(home), uupdumpISOName(t)))

	_, err := ResolveWindowsISO("", "", home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cell init --engine winkit")
}

func TestResolveWindowsISO_MissingReturnsErrorWithURL(t *testing.T) {
	_, err := ResolveWindowsISO("", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), WindowsISODownloadURL)
	assert.Contains(t, err.Error(), "cell init --engine winkit")
}

func TestResolveWindowsISO_FileNotFound(t *testing.T) {
	_, err := ResolveWindowsISO("/nonexistent/win.iso", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWindowsISODownloadURL_IsSet(t *testing.T) {
	assert.NotEmpty(t, WindowsISODownloadURL)
	assert.Contains(t, WindowsISODownloadURL, "microsoft.com")
}

func TestCacheDir(t *testing.T) {
	dir := CacheDir("/home/user")
	assert.Contains(t, dir, ".devcell/cache/qemu")
}

func TestDownloadWindowsISO_ServesCompleteMediaFromWinkit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	obs := &recordingObserver{}
	fakeFetchers{
		windows: func(_ context.Context, opts media.FetchOptions) (media.FetchResult, error) {
			assert.Equal(t, CacheDir(home), opts.CacheDir)
			assert.Equal(t, "de-de", opts.Spec.Language)
			require.NotNil(t, opts.LogFunc)
			require.NotNil(t, opts.OnProgress)
			opts.LogFunc("from winkit %d", 1)
			opts.OnProgress("x.esd", 50<<20, 100<<20)
			return media.FetchResult{Path: "/cache/mct.iso", Source: media.SourceMCT}, nil
		},
		full: uncachedFullMedia(t),
	}.install(t)

	path, err := DownloadWindowsISO(context.Background(), home, "de-de", obs)
	require.NoError(t, err)
	assert.Equal(t, "/cache/mct.iso", path)
	assert.Contains(t, obs.logs, "from winkit 1")
	require.NotEmpty(t, obs.fractions)
	assert.InDelta(t, 0.5, obs.fractions[len(obs.fractions)-1], 0.001)
}

// winkit's UUP dump fallback is boot-only, and the PE+WSL1 build needs
// sources/install.wim, so boot-only media sends devcell to its own
// complete-media UUP dump assembly.
func TestDownloadWindowsISO_BootOnlyFallsBackToCompleteMedia(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{
				Path: "/cache/boot-only.iso", Source: media.SourceUUPDump, BootOnly: true,
				FallbackErr: errors.New("mct catalog down"),
			}, nil
		},
		full: func(ctx context.Context, cfg uupdump.FetchConfig) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			assert.False(t, cfg.BootOnly)
			assert.Equal(t, fullMediaDir(CacheDir(home)), cfg.CacheDir)
			assert.Equal(t, "en-us", cfg.Spec.Language)
			return "/cache/full-media/complete.iso", nil
		},
	}.install(t)

	path, err := DownloadWindowsISO(context.Background(), home, "en-us", NopObserver{})
	require.NoError(t, err)
	assert.Equal(t, "/cache/full-media/complete.iso", path)
}

func TestDownloadWindowsISO_WinkitFailureFallsBackToCompleteMedia(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{}, errors.New("both winkit lanes failed")
		},
		full: func(ctx context.Context, _ uupdump.FetchConfig) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "/cache/full-media/complete.iso", nil
		},
	}.install(t)

	path, err := DownloadWindowsISO(context.Background(), t.TempDir(), "en-us", NopObserver{})
	require.NoError(t, err)
	assert.Equal(t, "/cache/full-media/complete.iso", path)
}

func TestDownloadWindowsISO_ReportsEveryFailure(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{BootOnly: true, FallbackErr: errors.New("mct catalog down")}, nil
		},
		full: func(ctx context.Context, _ uupdump.FetchConfig) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", errors.New("uup dump down")
		},
	}.install(t)

	_, err := DownloadWindowsISO(context.Background(), t.TempDir(), "en-us", NopObserver{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mct catalog down")
	assert.Contains(t, err.Error(), "uup dump down")
}

// Complete media an earlier fallback assembled is reused before the MCT
// lane gets a chance to download a second copy.
func TestDownloadWindowsISO_ReusesAssembledCompleteMedia(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		full: func(ctx context.Context, _ uupdump.FetchConfig) (string, error) {
			require.Error(t, ctx.Err(), "the reuse check must not download")
			return "/cache/full-media/complete.iso", nil
		},
	}.install(t)

	path, err := DownloadWindowsISO(context.Background(), t.TempDir(), "en-us", NopObserver{})
	require.NoError(t, err)
	assert.Equal(t, "/cache/full-media/complete.iso", path)
}

// winkit's boot-only lane has already downloaded the build's ESDs by the
// time the fallback runs; the fallback must reuse them, not fetch ~4 GB
// again.
func TestDownloadWindowsISO_FallbackReusesDownloadedESDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	esd := filepath.Join(CacheDir(home), "uupdump-download", "26100.1", "professional_en-us.esd")
	require.NoError(t, os.MkdirAll(filepath.Dir(esd), 0o755))
	require.NoError(t, os.WriteFile(esd, []byte("esd"), 0o644))

	fakeFetchers{
		windows: func(context.Context, media.FetchOptions) (media.FetchResult, error) {
			return media.FetchResult{BootOnly: true}, nil
		},
		full: func(ctx context.Context, cfg uupdump.FetchConfig) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			got, err := os.ReadFile(filepath.Join(cfg.CacheDir, "uupdump-download", "26100.1", "professional_en-us.esd"))
			require.NoError(t, err)
			assert.Equal(t, "esd", string(got))
			return "/cache/full-media/complete.iso", nil
		},
	}.install(t)

	_, err := DownloadWindowsISO(context.Background(), home, "en-us", NopObserver{})
	require.NoError(t, err)
}

func TestDownloadVirtioDrivers_FetchesIntoVirtioISOPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	obs := &recordingObserver{}
	fakeFetchers{
		virtio: func(_ context.Context, opts media.FetchOptions) (string, error) {
			assert.Equal(t, CacheDir(home), opts.CacheDir)
			require.NotNil(t, opts.LogFunc)
			require.NotNil(t, opts.OnProgress)
			opts.OnProgress("virtio-win.iso", 25<<20, 100<<20)
			return filepath.Join(opts.CacheDir, "virtio-win.iso"), nil
		},
	}.install(t)

	path, err := DownloadVirtioDrivers(context.Background(), home, false, obs)
	require.NoError(t, err)
	assert.Equal(t, VirtioISOPath(home), path)
	require.NotEmpty(t, obs.fractions)
	assert.InDelta(t, 0.25, obs.fractions[len(obs.fractions)-1], 0.001)
}

func TestDownloadVirtioDrivers_NoCacheDiscardsTheCachedISO(t *testing.T) {
	for _, noCache := range []bool{false, true} {
		t.Run(fmt.Sprintf("noCache=%v", noCache), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
			cached := VirtioISOPath(home)
			require.NoError(t, os.MkdirAll(filepath.Dir(cached), 0o755))
			require.NoError(t, os.WriteFile(cached, []byte("stale"), 0o644))

			fakeFetchers{
				virtio: func(context.Context, media.FetchOptions) (string, error) {
					_, err := os.Stat(cached)
					assert.Equal(t, noCache, os.IsNotExist(err))
					return cached, nil
				},
			}.install(t)

			_, err := DownloadVirtioDrivers(context.Background(), home, noCache, NopObserver{})
			require.NoError(t, err)
		})
	}
}

func TestDownloadVirtioDrivers_WrapsFetchErrors(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	fakeFetchers{
		virtio: func(context.Context, media.FetchOptions) (string, error) {
			return "", errors.New("fedorapeople down")
		},
	}.install(t)

	_, err := DownloadVirtioDrivers(context.Background(), t.TempDir(), false, NopObserver{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VirtIO")
	assert.Contains(t, err.Error(), "fedorapeople down")
}

// fakeFetchers replaces winkit's network-bound fetchers for one test. A nil
// field fails the test if that fetcher runs.
type fakeFetchers struct {
	windows func(context.Context, media.FetchOptions) (media.FetchResult, error)
	full    func(context.Context, uupdump.FetchConfig) (string, error)
	virtio  func(context.Context, media.FetchOptions) (string, error)
}

func (f fakeFetchers) install(t *testing.T) {
	t.Helper()
	origWindows, origFull, origVirtio := fetchWindowsMedia, fetchFullMedia, fetchVirtioMedia
	t.Cleanup(func() {
		fetchWindowsMedia, fetchFullMedia, fetchVirtioMedia = origWindows, origFull, origVirtio
	})
	fetchWindowsMedia = func(ctx context.Context, opts media.FetchOptions) (media.FetchResult, error) {
		if f.windows == nil {
			t.Fatal("unexpected media.FetchWindowsISO call")
		}
		return f.windows(ctx, opts)
	}
	fetchFullMedia = func(ctx context.Context, cfg uupdump.FetchConfig) (string, error) {
		if f.full == nil {
			t.Fatal("unexpected complete-media UUP dump call")
		}
		return f.full(ctx, cfg)
	}
	fetchVirtioMedia = func(ctx context.Context, opts media.FetchOptions) (string, error) {
		if f.virtio == nil {
			t.Fatal("unexpected media.FetchVirtioISO call")
		}
		return f.virtio(ctx, opts)
	}
}

// uncachedFullMedia is a complete-media fetcher with nothing cached: the
// cache-only reuse check misses, and a real fetch fails the test.
func uncachedFullMedia(t *testing.T) func(context.Context, uupdump.FetchConfig) (string, error) {
	return func(ctx context.Context, _ uupdump.FetchConfig) (string, error) {
		if ctx.Err() == nil {
			t.Fatal("complete-media UUP dump fallback must not run")
		}
		return "", ctx.Err()
	}
}

type recordingObserver struct {
	logs      []string
	fractions []float64
}

func (o *recordingObserver) Logf(format string, args ...any) {
	o.logs = append(o.logs, fmt.Sprintf(format, args...))
}

func (o *recordingObserver) Progress(fraction float64, _ string) {
	o.fractions = append(o.fractions, fraction)
}

// writeBootableISO plants an ISO that passes winkit's cache check, which
// accepts only firmware-bootable media (El Torito EFI catalog).
func writeBootableISO(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	img := make([]byte, 64*2048)
	for sector, magic := range map[int]string{16: "BEA01", 17: "NSR02", 18: "TEA01"} {
		copy(img[sector*2048+1:], magic)
		img[sector*2048+6] = 0x01
	}
	require.NoError(t, os.WriteFile(path, img, 0o644))
	require.NoError(t, isokit.AddElToritoEFIBoot(path, []byte("boot-image")))
}

// uupdumpISOName is the file name both UUP dump lanes give the default
// en-us ISO.
func uupdumpISOName(t *testing.T) string {
	t.Helper()
	r, err := media.Spec{Language: "en-us"}.Resolve()
	require.NoError(t, err)
	return r.ISOName()
}
