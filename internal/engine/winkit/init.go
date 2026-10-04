//go:build darwin || linux

package winkit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/ux"
)

// keyDir is where a cell's VM SSH keypair lives.
//
// ~/.devcell/<cell>/.ssh: per cell and engine-neutral. Naming the directory
// after qemu was an accident of which engine needed keys first.
//
// A cell that already has a key under the legacy qemu/ path keeps it. The
// public half is baked into a built template, so relocating the private half
// would leave a multi-hour template nothing can log into.
func keyDir(home, cellName string) string {
	legacy := filepath.Join(home, ".devcell", cellName, "qemu")
	if _, err := os.Stat(filepath.Join(legacy, "id_ed25519")); err == nil {
		return legacy
	}
	return filepath.Join(home, ".devcell", cellName, ".ssh")
}

// Init prepares directories and an SSH keypair and downloads the VirtIO
// drivers and Windows ARM64 ISO the build needs. It creates no VM; `cell
// build --engine winkit` does. opts.Stack overrides opts.Cell.Stack.
// opts.Force regenerates the keypair and downloads the VirtIO drivers again.
func (Engine) Init(ctx context.Context, opts engine.InitOpts) error {
	g, err := guestFor(opts.Cell.Guest)
	if err != nil {
		return err
	}
	c := opts.Cell.WithStack(opts.Stack)
	force := opts.Force
	sshDir := keyDir(c.HostHome, c.Name)
	tmplDir := templateDir(c.HostHome, g, c.Stack, c.Modules)
	instanceDir := InstanceDir(c.HostHome, c.Name)

	ux.Debugf("init winkit: cell=%s stack=%s guest=%s", c.Name, c.Stack, g.label())
	ux.Debugf("ssh dir: %s", sshDir)

	pr := &ux.PhaseRunner{}

	// --- Phase 1: Create directories ---
	if err := pr.PhaseDetailed("Preparing directories", func() (string, error) {
		for _, dir := range []string{sshDir, tmplDir, instanceDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("creating %s: %w", dir, err)
			}
		}
		return sshDir, nil
	}); err != nil {
		return err
	}

	// --- Phase 2: Generate SSH keypair ---
	privKeyPath := filepath.Join(sshDir, "id_ed25519")
	pubKeyPath := filepath.Join(sshDir, "id_ed25519.pub")
	if err := pr.PhaseDetailed("Generating SSH keypair", func() (string, error) {
		if !force {
			if _, err := os.Stat(privKeyPath); err == nil {
				ux.Debugf("SSH keypair exists, skipping (use --force to regenerate)")
				return privKeyPath, nil
			}
		}

		os.Remove(privKeyPath)
		os.Remove(pubKeyPath)
		cmd := exec.CommandContext(ctx, "ssh-keygen", "-t", "ed25519", "-f", privKeyPath, "-N", "", "-q")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("ssh-keygen: %w\n%s", err, out)
		}
		ux.Debugf("SSH keypair generated: %s", privKeyPath)

		// Collect existing ~/.ssh pub keys to add to authorized_keys
		pubKey, err := os.ReadFile(pubKeyPath)
		if err != nil {
			return "", fmt.Errorf("reading public key: %w", err)
		}
		allKeys := strings.TrimSpace(string(pubKey))

		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			existing := collectSSHPubKeys(filepath.Join(homeDir, ".ssh"))
			if existing != "" {
				allKeys = allKeys + "\n" + existing
				ux.Debugf("added existing ~/.ssh pub keys")
			}
		}

		authKeysPath := filepath.Join(sshDir, "authorized_keys")
		if err := os.WriteFile(authKeysPath, []byte(allKeys+"\n"), 0644); err != nil {
			return "", fmt.Errorf("writing authorized_keys: %w", err)
		}

		return privKeyPath, nil
	}); err != nil {
		return err
	}

	// --- Phase 3: Download VirtIO drivers ---
	if err := pr.PhaseDetailed("Downloading VirtIO drivers", func() (string, error) {
		obs := &phaseObserver{logf: ux.Debugf, runner: pr}
		path, err := DownloadVirtioDrivers(ctx, c.HostHome, force, obs)
		if err != nil {
			return "", err
		}
		return path, nil
	}); err != nil {
		return err
	}

	// --- Phase 4: Download Windows ARM64 ISO ---
	if err := pr.PhaseDetailed("Downloading Windows ARM64 ISO", func() (string, error) {
		obs := &phaseObserver{logf: ux.Debugf, runner: pr}
		path, err := DownloadWindowsISO(ctx, c.HostHome, "en-us", obs)
		if err != nil {
			return "", err
		}
		return path, nil
	}); err != nil {
		return err
	}

	pr.Seal("winkit artifacts ready")
	fmt.Println("  Run: cell build --engine winkit")
	return nil
}

// collectSSHPubKeys reads all *.pub files from sshDir.
func collectSSHPubKeys(sshDir string) string {
	matches, err := filepath.Glob(filepath.Join(sshDir, "*.pub"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	var keys []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		line := strings.TrimSpace(string(data))
		if line != "" {
			keys = append(keys, line)
		}
	}
	return strings.Join(keys, "\n")
}

// phaseObserver adapts Observer to ux.Debugf + PhaseRunner spinner updates.
type phaseObserver struct {
	logf   func(string, ...any)
	runner *ux.PhaseRunner
}

func (o *phaseObserver) Logf(format string, args ...any) {
	o.logf(format, args...)
}
func (o *phaseObserver) Progress(_ float64, msg string) {
	if o.runner != nil {
		o.runner.UpdateText(msg)
	}
}
