package gui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/gui"
)

func TestRDPUrl(t *testing.T) {
	got := gui.RDPUrl("389")
	want := "rdp://full%20address=s%3A127.0.0.1%3A389"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestRoyalTSXUrl(t *testing.T) {
	got := gui.RoyalTSXUrl("389", "dmitry", "rdp")
	want := "rtsx://rdp://dmitry:rdp@127.0.0.1:389"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// mockLookPath returns a lookPath that "finds" only the given binaries.
func mockLookPath(available ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, b := range available {
		set[b] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}
}

func TestFindClient_DarwinPrefersSDL(t *testing.T) {
	// Both sdl and x11 available: macOS should prefer sdl-freerdp3
	c, ok := gui.FindClientWith("darwin", mockLookPath("xfreerdp", "sdl-freerdp3"))
	if !ok {
		t.Fatal("expected to find client")
	}
	if c.Name != "sdl-freerdp3" {
		t.Errorf("darwin should prefer sdl-freerdp3, got %q", c.Name)
	}
}

func TestFindClient_LinuxPrefersX11(t *testing.T) {
	// Both sdl and x11 available: Linux should prefer xfreerdp3
	c, ok := gui.FindClientWith("linux", mockLookPath("sdl-freerdp3", "xfreerdp3"))
	if !ok {
		t.Fatal("expected to find client")
	}
	if c.Name != "xfreerdp3" {
		t.Errorf("linux should prefer xfreerdp3, got %q", c.Name)
	}
}

func TestFindClient_FallsBackToV2(t *testing.T) {
	// Only v2 available
	c, ok := gui.FindClientWith("darwin", mockLookPath("sdl-freerdp"))
	if !ok {
		t.Fatal("expected to find client")
	}
	if c.Name != "sdl-freerdp" {
		t.Errorf("should fall back to sdl-freerdp, got %q", c.Name)
	}
}

func TestFindClient_NoneAvailable(t *testing.T) {
	_, ok := gui.FindClientWith("darwin", mockLookPath())
	if ok {
		t.Error("expected no client found")
	}
}

func TestFindClient_LinuxFallsToSDL(t *testing.T) {
	// Only SDL available on Linux: should still find it
	c, ok := gui.FindClientWith("linux", mockLookPath("sdl-freerdp3"))
	if !ok {
		t.Fatal("expected to find client")
	}
	if c.Name != "sdl-freerdp3" {
		t.Errorf("linux should fall back to sdl-freerdp3, got %q", c.Name)
	}
}

func TestCertFingerprint_ValidCert(t *testing.T) {
	// Generate a self-signed cert in a temp dir
	dir := t.TempDir()
	xrdpDir := filepath.Join(dir, "xrdp")
	os.MkdirAll(xrdpDir, 0700)
	out, err := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
		"-keyout", filepath.Join(xrdpDir, "key.pem"),
		"-out", filepath.Join(xrdpDir, "cert.pem"),
		"-days", "1", "-subj", "/CN=test").CombinedOutput()
	if err != nil {
		t.Skipf("openssl not available: %v\n%s", err, out)
	}
	fp := gui.CertFingerprint(dir)
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	// SHA256 fingerprint = 32 bytes = 32 hex pairs with colons
	parts := strings.Split(fp, ":")
	if len(parts) != 32 {
		t.Errorf("expected 32 colon-separated hex bytes, got %d: %s", len(parts), fp)
	}
}

func TestCertFingerprint_MissingCert(t *testing.T) {
	fp := gui.CertFingerprint(t.TempDir())
	if fp != "" {
		t.Errorf("expected empty fingerprint for missing cert, got %q", fp)
	}
}

func TestCertFlag_WithCert(t *testing.T) {
	dir := t.TempDir()
	xrdpDir := filepath.Join(dir, "xrdp")
	os.MkdirAll(xrdpDir, 0700)
	out, err := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
		"-keyout", filepath.Join(xrdpDir, "key.pem"),
		"-out", filepath.Join(xrdpDir, "cert.pem"),
		"-days", "1", "-subj", "/CN=test").CombinedOutput()
	if err != nil {
		t.Skipf("openssl not available: %v\n%s", err, out)
	}
	flag := gui.CertFlag(dir)
	if !strings.HasPrefix(flag, "/cert:fingerprint:sha256:") {
		t.Errorf("expected fingerprint flag, got %q", flag)
	}
}

func TestCertFlag_WithoutCert(t *testing.T) {
	flag := gui.CertFlag(t.TempDir())
	if flag != "/cert:ignore" {
		t.Errorf("expected /cert:ignore fallback, got %q", flag)
	}
}
