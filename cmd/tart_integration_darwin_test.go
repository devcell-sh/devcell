//go:build darwin && arm64

package main

// Integration diagnostics for the `cell <agent> --os macos` (tart) path.
//
// Run on a Mac (Apple Silicon):
//
//	go test -v -run TestTartDarwinIntegration ./cmd/
//	task test:tart
//
// Every subtest writes its raw command output into
// test/results/<timestamp>-TestTartDarwinIntegration/<subtest>/ so a failure
// can be diagnosed from the artifacts alone — attach that directory when
// reporting a bug.
//
// The expensive VM stages (tart clone / boot / provision) are NOT run here;
// this covers the host-side stages that gate the auto-build, ending with the
// platform preflight that `cell build --engine=tart` runs first.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DimmKirr/devcell/internal/runner"
	"github.com/DimmKirr/devcell/internal/testutil"
	"github.com/DimmKirr/devcell/internal/version"
	"github.com/DimmKirr/devcell/internal/vm/tart"
)

func TestTartDarwinIntegration(t *testing.T) {
	flakeRef := runner.ResolveNixhomeRef(version.Version)

	t.Run("environment", func(t *testing.T) {
		resultsDir := testutil.TestResultsDir(t, nil)

		env := map[string]string{
			"goos":                 runtime.GOOS,
			"goarch":               runtime.GOARCH,
			"cell_version":         version.Version,
			"nixhome_flake_ref":    flakeRef,
			"DEVCELL_NIXHOME":      os.Getenv("DEVCELL_NIXHOME"),
			"DEVCELL_NIXHOME_PATH": os.Getenv("DEVCELL_NIXHOME_PATH"),
			"DEVCELL_CELL_NAME":    os.Getenv("DEVCELL_CELL_NAME"),
			"nix":                  captureVersion(t, resultsDir, "nix", "--version"),
			"tart":                 captureVersion(t, resultsDir, "tart", "--version"),
		}
		writeJSON(t, filepath.Join(resultsDir, "environment.json"), env)
		for k, v := range env {
			t.Logf("%s = %s", k, v)
		}
	})

	t.Run("flake-metadata", func(t *testing.T) {
		resultsDir := testutil.TestResultsDir(t, nil)
		requireBinary(t, "nix")

		out, err := runLogged(t, resultsDir, "flake-metadata.log",
			"nix", "flake", "metadata", flakeRef, "--json")
		if err != nil {
			t.Fatalf("nix flake metadata %s: %v\n%s", flakeRef, err, out)
		}
	})

	// Reproduces the first phase of the tart auto-build
	// (build_tart_darwin.go "Platform compatibility check"), which fails
	// when a nixhome module pulls in a Linux-only package (e.g. CELL: the
	// vm module's virtiofsd broke `cell claude --os macos`).
	t.Run("platform-preflight", func(t *testing.T) {
		resultsDir := testutil.TestResultsDir(t, nil)
		requireBinary(t, "nix")

		// Raw eval first — unlike the CLI (which trims to 5 error lines),
		// keep the complete trace for diagnosis.
		attr := fmt.Sprintf("%s#platformStrictCheck.aarch64-darwin", flakeRef)
		rawOut, rawErr := runLogged(t, resultsDir, "nix-eval-full.log",
			"nix", "eval", attr, "--json", "--show-trace")

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
		defer cancel()
		preflightErr := runner.PreflightPlatformCheck(ctx, flakeRef, "aarch64-darwin")

		writeJSON(t, filepath.Join(resultsDir, "run.json"), map[string]any{
			"flake_ref":       flakeRef,
			"attr":            attr,
			"raw_eval_error":  errString(rawErr),
			"preflight_error": errString(preflightErr),
		})

		if preflightErr != nil {
			t.Fatalf("platform preflight failed (full nix output in %s):\n%v\nraw eval tail:\n%s",
				resultsDir, preflightErr, tail(rawOut, 30))
		}
	})

	t.Run("host-nix-detection", func(t *testing.T) {
		resultsDir := testutil.TestResultsDir(t, nil)

		hostNix := tart.DetectHostNixStore()
		isStore := tart.IsNixStore("/nix")

		result := map[string]any{
			"detected":    hostNix,
			"is_nix_store": isStore,
		}

		if hostNix != "" {
			// Validate the store has actual content
			entries, err := os.ReadDir(filepath.Join(hostNix, "store"))
			if err != nil {
				t.Fatalf("detected host nix at %s but can't read store: %v", hostNix, err)
			}
			result["store_path_count"] = len(entries)
			t.Logf("host nix store: %d paths", len(entries))

			if len(entries) < 10 {
				t.Errorf("host nix store has only %d paths — expected a populated store", len(entries))
			}

			// Read content of a few store paths to verify they're accessible
			readable := 0
			var samplePaths []string
			for i, e := range entries {
				if i >= 5 {
					break
				}
				p := filepath.Join(hostNix, "store", e.Name())
				info, err := os.Stat(p)
				if err != nil {
					t.Errorf("can't stat store path %s: %v", e.Name(), err)
					continue
				}
				if info.IsDir() {
					// verify we can list contents inside a store path
					inner, err := os.ReadDir(p)
					if err != nil {
						t.Errorf("can't read store path dir %s: %v", e.Name(), err)
						continue
					}
					samplePaths = append(samplePaths, fmt.Sprintf("%s (%d entries)", e.Name(), len(inner)))
				} else {
					samplePaths = append(samplePaths, fmt.Sprintf("%s (%d bytes)", e.Name(), info.Size()))
				}
				readable++
			}
			result["sample_paths"] = samplePaths
			result["readable_count"] = readable
			t.Logf("verified %d store paths are readable", readable)
			if readable == 0 {
				t.Error("could not read any store path content")
			}

			// Find and read a known binary (nix itself should be in the store)
			nixBin, _ := exec.LookPath("nix")
			if nixBin != "" {
				resolved, err := filepath.EvalSymlinks(nixBin)
				if err == nil && strings.HasPrefix(resolved, "/nix/store/") {
					f, err := os.Open(resolved)
					if err != nil {
						t.Errorf("can't open nix binary at %s: %v", resolved, err)
					} else {
						header := make([]byte, 4)
						n, _ := f.Read(header)
						f.Close()
						result["nix_binary"] = resolved
						result["nix_binary_header_bytes"] = n
						t.Logf("nix binary at %s: read %d header bytes", resolved, n)
						if n < 4 {
							t.Errorf("could only read %d bytes from nix binary", n)
						}
					}
				}
			}

			// Verify DB exists, is non-empty, and has the SQLite magic header
			dbPath := filepath.Join(hostNix, "var", "nix", "db", "db.sqlite")
			dbInfo, err := os.Stat(dbPath)
			if err != nil {
				t.Fatalf("detected host nix but db.sqlite missing: %v", err)
			}
			result["db_size_bytes"] = dbInfo.Size()
			t.Logf("nix database: %d bytes", dbInfo.Size())

			if dbInfo.Size() < 1024 {
				t.Errorf("nix database unexpectedly small: %d bytes", dbInfo.Size())
			}

			// Read SQLite header magic to confirm it's a real database
			dbFile, err := os.Open(dbPath)
			if err != nil {
				t.Errorf("can't open db.sqlite: %v", err)
			} else {
				magic := make([]byte, 16)
				n, _ := dbFile.Read(magic)
				dbFile.Close()
				if n >= 15 && string(magic[:15]) == "SQLite format 3" {
					result["db_magic_ok"] = true
					t.Log("nix database: valid SQLite header")
				} else {
					t.Errorf("nix database has unexpected header: %q", string(magic[:n]))
				}
			}

			// Verify the substituter script references the right paths
			script := tart.GenerateHostNixSubstituterScript()
			for _, want := range []string{"hostnix", "host-nix-root", "db.sqlite", "local?root="} {
				if !strings.Contains(script, want) {
					t.Errorf("substituter script missing %q", want)
				}
			}
			result["substituter_script_ok"] = true

			// Verify diskutil shows the Nix Store volume (informational)
			diskOut, _ := runLogged(t, resultsDir, "diskutil-list.log", "diskutil", "list")
			result["has_nix_store_volume"] = strings.Contains(diskOut, "Nix Store")
		} else {
			t.Log("no host nix store detected — substituter caching will not be available")
			result["note"] = "host nix not installed; build will use 100GB sparse disk"
		}

		// Verify disk size constant
		if tart.NixVolumeSizeGB != 100 {
			t.Errorf("NixVolumeSizeGB = %d, want 100", tart.NixVolumeSizeGB)
		}
		result["nix_volume_size_gb"] = tart.NixVolumeSizeGB

		writeJSON(t, filepath.Join(resultsDir, "host-nix.json"), result)
	})
}

// captureVersion records `bin args...` output into <bin>-version.log and
// returns its first line ("<not found>" / "<failed>" instead of failing the
// test — the environment subtest is pure capture).
func captureVersion(t *testing.T, resultsDir, bin string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		return "<not found>"
	}
	out, err := runLogged(t, resultsDir, bin+"-version.log", bin, args...)
	if err != nil {
		return "<failed: " + err.Error() + ">"
	}
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return first
}

// runLogged runs a command, tees combined output into resultsDir/logName,
// and returns it. The command line itself is the log's first line.
func runLogged(t *testing.T, resultsDir, logName, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	content := fmt.Sprintf("$ %s %s\n%s", bin, strings.Join(args, " "), buf.String())
	if werr := os.WriteFile(filepath.Join(resultsDir, logName), []byte(content), 0o644); werr != nil {
		t.Fatalf("write %s: %v", logName, werr)
	}
	return buf.String(), err
}

func requireBinary(t *testing.T, bin string) {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s not in PATH", bin)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
