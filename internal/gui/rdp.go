package gui

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// RDPUrl returns an RDP URL for the given port.
// Uses percent-encoded format required by macOS Sonoma+ Windows App:
//
//	rdp://full%20address=s%3A127.0.0.1%3A<port>
func RDPUrl(port string) string {
	return "rdp://full%20address=s%3A127.0.0.1%3A" + port
}

// RoyalTSXUrl returns a Royal TSX URI for the given port and credentials.
// Note: macOS Royal TSX does not support property_* query params in adhoc URIs.
// Retina must be enabled via Application → Default Settings → Remote Desktop → Display.
func RoyalTSXUrl(port, user, password string) string {
	return "rtsx://rdp://" + user + ":" + password + "@127.0.0.1:" + port
}

// HasRoyalTSX checks if Royal TSX is installed on macOS.
func HasRoyalTSX() bool {
	paths := []string{
		"/Applications/Royal TSX.app",
		filepath.Join(os.Getenv("HOME"), "Applications", "Royal TSX.app"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// ClientBinary holds the resolved FreeRDP binary path and name.
type ClientBinary struct {
	Path string // absolute path from LookPath
	Name string // binary name (e.g. "sdl-freerdp3")
}

// freerdpCandidates returns binary names in preference order for the given OS.
// macOS: prefer SDL variants (no X11 needed); Linux: prefer X11 variants.
func freerdpCandidates(goos string) []string {
	if goos == "darwin" {
		return []string{"sdl-freerdp3", "sdl-freerdp", "xfreerdp3", "xfreerdp"}
	}
	return []string{"xfreerdp3", "xfreerdp", "sdl-freerdp3", "sdl-freerdp"}
}

// FindClient finds the best available FreeRDP client binary for the current platform.
func FindClient() (ClientBinary, bool) {
	return FindClientWith(runtime.GOOS, exec.LookPath)
}

// FindClientWith is like FindClient but accepts explicit OS and lookPath for testing.
func FindClientWith(goos string, lookPath func(string) (string, error)) (ClientBinary, bool) {
	for _, name := range freerdpCandidates(goos) {
		if path, err := lookPath(name); err == nil {
			return ClientBinary{Path: path, Name: name}, true
		}
	}
	return ClientBinary{}, false
}

// InstallHint returns platform-specific install instructions for FreeRDP.
func InstallHint() string {
	return "xfreerdp not found — install FreeRDP and retry:\n\n" +
		"  macOS:   brew install freerdp\n" +
		"  nixpkgs: nix profile install nixpkgs#freerdp\n" +
		"  Debian:  sudo apt install freerdp3-x11\n" +
		"  Fedora:  sudo dnf install freerdp\n" +
		"  Arch:    sudo pacman -S freerdp\n"
}

// CertFingerprint reads the xrdp server cert from configDir/xrdp/cert.pem
// and returns its SHA256 fingerprint as a hex string (colon-separated).
// Returns empty string if the cert doesn't exist or can't be parsed.
func CertFingerprint(configDir string) string {
	data, err := os.ReadFile(filepath.Join(configDir, "xrdp", "cert.pem"))
	if err != nil {
		return ""
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return ""
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = hex.EncodeToString([]byte{b})
	}
	return strings.Join(parts, ":")
}

// CertFlag returns the FreeRDP /cert: flag. Uses fingerprint verification
// if the cert exists, otherwise falls back to /cert:ignore.
func CertFlag(configDir string) string {
	if fp := CertFingerprint(configDir); fp != "" {
		return "/cert:fingerprint:sha256:" + fp
	}
	return "/cert:ignore"
}
