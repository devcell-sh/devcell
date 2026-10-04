package winkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devcell-sh/go-winkit/cache"
	"github.com/devcell-sh/go-winkit/media"
	"github.com/devcell-sh/go-winkit/media/uupdump"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// WindowsISODownloadURL is the Microsoft page for downloading Windows 11 ARM64 ISO.
// Kept for the manual-download fallback message in ResolveWindowsISO.
const WindowsISODownloadURL = "https://www.microsoft.com/en-us/software-download/windows11arm64"

// Seams for tests: winkit's network-bound fetchers.
var (
	fetchWindowsMedia = media.FetchWindowsISO
	fetchFullMedia    = uupdump.FetchWindowsISO
	fetchVirtioMedia  = media.FetchVirtioISO
)

// DownloadWindowsISO returns a cached Windows 11 ARM64 install ISO with
// complete media, downloading and assembling it when missing.
//
// winkit's media.FetchWindowsISO serves the MCT catalog lane, which yields
// complete media, and its cache. Its UUP dump fallback (taken when the MCT
// catalog fails) builds boot-only media with no sources/install.wim, and the
// PE+WSL1 build extracts install.wim from the ISO, so boot-only media cannot
// feed it. Until winkit's fallback can assemble complete media, devcell runs
// UUP dump itself with BootOnly off whenever winkit returns boot-only media or
// fails. That ISO lives in fullMediaDir because both UUP dump lanes give their
// ISO the same file name and winkit's cache check cannot tell them apart.
func DownloadWindowsISO(ctx context.Context, home, language string, obs Observer) (string, error) {
	cacheDir := CacheDir(home)
	spec := media.Spec{Language: language}
	progress := progressFunc(obs)
	full := uupdump.FetchConfig{
		CacheDir:   fullMediaDir(cacheDir),
		Spec:       spec,
		LogFunc:    obs.Logf,
		OnProgress: progress,
	}

	// Reuse complete media an earlier fallback assembled before the MCT lane
	// gets a chance to download a second copy.
	probe := full
	probe.LogFunc = nil
	if path, err := fetchFullMedia(cacheOnly(ctx), probe); err == nil {
		obs.Logf("Windows ISO cache hit: %s", path)
		return path, nil
	}

	res, err := fetchWindowsMedia(ctx, media.FetchOptions{
		CacheDir:   cacheDir,
		Spec:       spec,
		LogFunc:    obs.Logf,
		OnProgress: progress,
	})
	if err == nil && !res.BootOnly {
		return res.Path, nil
	}
	cause := err
	if cause == nil {
		cause = res.FallbackErr
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("downloading Windows ISO: %w", errors.Join(cause, ctxErr))
	}

	obs.Logf("winkit has no complete Windows media (%v): assembling it from UUP dump for the PE+WSL1 build", cause)
	shareESDDownloads(cacheDir, full.CacheDir, obs)
	path, fullErr := fetchFullMedia(ctx, full)
	if fullErr != nil {
		return "", fmt.Errorf("downloading Windows ISO (MCT catalog and UUP dump both failed): %w",
			errors.Join(cause, fullErr))
	}
	return path, nil
}

// fullMediaDir holds the complete-media ISO devcell assembles from UUP dump,
// apart from the boot-only one winkit's fallback writes to cacheDir.
func fullMediaDir(cacheDir string) string {
	return filepath.Join(cacheDir, "full-media")
}

// shareESDDownloads points the complete-media lane's ESD download directory
// at the one winkit's boot-only lane has just filled, so the fallback reuses
// those ~4 GB instead of fetching them again (uupdump skips files that are
// already complete). "uupdump-download" is winkit's directory name; if it
// changes, the fallback still works and only downloads twice.
func shareESDDownloads(cacheDir, fullDir string, obs Observer) {
	link := filepath.Join(fullDir, "uupdump-download")
	if _, err := os.Lstat(link); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "uupdump-download"), 0o755); err != nil {
		obs.Logf("not sharing UUP dump downloads: %v", err)
		return
	}
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		obs.Logf("not sharing UUP dump downloads: %v", err)
		return
	}
	// Relative, so the link survives the cache being mounted elsewhere.
	if err := os.Symlink(filepath.Join("..", "uupdump-download"), link); err != nil {
		obs.Logf("not sharing UUP dump downloads: %v", err)
	}
}

// cacheOnly returns an already-canceled child of ctx. winkit's fetchers
// check their cache before any network I/O and every request they make
// honours ctx, so under this context a fetch returns cached media or fails
// without downloading. winkit has no public cache-only lookup; this stands
// in for one.
func cacheOnly(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	return ctx
}

// CacheDir returns the QEMU media cache directory.
//
// DEVCELL_WINKIT_CACHE_DIR points it somewhere shared. Inside a cell $HOME is
// itself a per-cell directory, so the default renders as
// ~/.devcell/<cell>/.devcell/cache/qemu and every cell re-downloads the same
// ~6 GB of immutable media. There is no way to reach the real host home from
// inside the container, so the location has to be pointable rather than
// inferred (CELL-386).
func CacheDir(home string) string {
	if dir := cfg.Getenv("DEVCELL_WINKIT_CACHE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".devcell", "cache", "qemu")
}

// VirtioISOPath returns the path to the cached VirtIO drivers ISO, where
// winkit's media.FetchVirtioISO puts it.
func VirtioISOPath(home string) string {
	return filepath.Join(CacheDir(home), cache.VirtIOISOName)
}

// DownloadVirtioDrivers returns the cached VirtIO drivers ISO at
// VirtioISOPath, downloading it via winkit when missing or unusable. noCache
// discards the cached ISO first to force a fresh download.
func DownloadVirtioDrivers(ctx context.Context, home string, noCache bool, obs Observer) (string, error) {
	if noCache {
		obs.Logf("discarding cached VirtIO drivers ISO")
		if err := os.Remove(VirtioISOPath(home)); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("discarding cached VirtIO drivers ISO: %w", err)
		}
	}
	path, err := fetchVirtioMedia(ctx, media.FetchOptions{
		CacheDir:   CacheDir(home),
		LogFunc:    obs.Logf,
		OnProgress: progressFunc(obs),
	})
	if err != nil {
		return "", fmt.Errorf("downloading VirtIO drivers: %w", err)
	}
	return path, nil
}

// ResolveWindowsISO resolves the Windows ARM64 ISO path.
// Priority: env DEVCELL_WINKIT_WINDOWS_ISO > config path > cached download > error.
//
// The cached download is looked up without touching the network: `cell build`
// must not start the multi-gigabyte download `cell init` owns.
func ResolveWindowsISO(envISO, configISO, home string) (string, error) {
	path := envISO
	if path == "" {
		path = configISO
	}
	if path == "" && home != "" {
		if cached, err := DownloadWindowsISO(cacheOnly(context.Background()), home, "en-us", NopObserver{}); err == nil {
			path = cached
		}
	}
	if path == "" {
		return "", fmt.Errorf("Windows ARM64 ISO not configured.\n\n"+
			"Run: cell init --engine winkit  (downloads automatically)\n"+
			"Or download from: %s\n"+
			"Then set: export DEVCELL_WINKIT_WINDOWS_ISO=/path/to/Win11_ARM64.iso\n"+
			"Or add to .devcell.toml:\n"+
			"  [cell]\n"+
			"  winkit_windows_iso = \"/path/to/Win11_ARM64.iso\"", WindowsISODownloadURL)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("Windows ISO not found at %s: %w", path, err)
	}
	if err := ValidateISO(path); err != nil {
		return "", fmt.Errorf("invalid ISO at %s: %w", path, err)
	}
	return path, nil
}

// ValidateISO checks that a file carries a recognised disc format by reading
// the volume descriptor at sector 16 (offset 0x8001). Both ISO 9660 (CD001)
// and UDF (BEA01/NSR02/NSR03) are accepted — Windows ARM64 ISOs built by UUP
// dump are pure UDF.
func ValidateISO(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 5)
	if _, err := f.ReadAt(magic, 0x8001); err != nil {
		return fmt.Errorf("cannot read ISO magic bytes: %w", err)
	}
	switch string(magic) {
	case "CD001", "BEA01", "NSR02", "NSR03":
		return nil
	default:
		return fmt.Errorf("not a recognised disc image (expected CD001 or UDF descriptor at offset 0x8001, got %q)", magic)
	}
}
