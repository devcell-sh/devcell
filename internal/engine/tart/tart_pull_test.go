package tart

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var mockTartBin string

func TestMain(m *testing.M) {
	tmpBin, err := os.MkdirTemp("", "mocktart-bin-*")
	if err != nil {
		log.Fatalf("creating temp dir: %v", err)
	}

	mockTartBin = filepath.Join(tmpBin, "tart")
	build := exec.Command("go", "build", "-o", mockTartBin, "./testdata/mocktart")
	if out, err := build.CombinedOutput(); err != nil {
		log.Fatalf("building mock tart:\n%s\n%v", out, err)
	}

	os.Setenv("PATH", tmpBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := m.Run()
	os.RemoveAll(tmpBin)
	os.Exit(code)
}

// --- TartClone ---

func TestTartClone(t *testing.T) {
	th := t.TempDir()
	t.Setenv("TART_HOME", th)

	err := TartClone(context.Background(), "ghcr.io/cirruslabs/macos-tahoe-base:latest", "test-vm")
	if err != nil {
		t.Fatalf("TartClone() error: %v", err)
	}

	vmDir := filepath.Join(th, "vms", "test-vm")
	for _, f := range []string{"config.json", "disk.img", "nvram.bin"} {
		if _, err := os.Stat(filepath.Join(vmDir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}

	cfgData, err := os.ReadFile(filepath.Join(vmDir, "config.json"))
	if err != nil {
		t.Fatalf("reading config.json: %v", err)
	}
	var cfg struct {
		OS       string `json:"os"`
		CPUCount int    `json:"cpuCount"`
	}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatalf("parsing config.json: %v", err)
	}
	if cfg.OS != "darwin" {
		t.Errorf("config OS = %q, want darwin", cfg.OS)
	}
	if cfg.CPUCount != 4 {
		t.Errorf("config CPUCount = %d, want 4", cfg.CPUCount)
	}
}

// --- TartGet ---

func TestTartGet(t *testing.T) {
	th := t.TempDir()
	t.Setenv("TART_HOME", th)

	if err := TartClone(context.Background(), "ghcr.io/test:latest", "get-vm"); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	info, err := TartGet(context.Background(), "get-vm")
	if err != nil {
		t.Fatalf("TartGet() error: %v", err)
	}
	if info.OS != "darwin" {
		t.Errorf("OS = %q, want darwin", info.OS)
	}
	if info.CPU != 4 {
		t.Errorf("CPU = %d, want 4", info.CPU)
	}
	if info.State != "stopped" {
		t.Errorf("State = %q, want stopped", info.State)
	}
}

// --- TartDelete ---

func TestTartDelete(t *testing.T) {
	th := t.TempDir()
	t.Setenv("TART_HOME", th)

	if err := TartClone(context.Background(), "ghcr.io/test:latest", "del-vm"); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	vmDir := filepath.Join(th, "vms", "del-vm")
	if _, err := os.Stat(vmDir); err != nil {
		t.Fatalf("VM dir should exist before delete: %v", err)
	}

	if err := TartDelete(context.Background(), "del-vm"); err != nil {
		t.Fatalf("TartDelete() error: %v", err)
	}

	if _, err := os.Stat(vmDir); !os.IsNotExist(err) {
		t.Errorf("VM dir should be gone after delete, got err: %v", err)
	}
}
