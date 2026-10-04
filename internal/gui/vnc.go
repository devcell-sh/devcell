package gui

import (
	"os"
	"path/filepath"
)

// vncPasswdBytes is the DES-encrypted VNC password for "vnc".
// Pre-computed using the standard VNC fixed key, so vncpasswd is not needed at runtime.
var vncPasswdBytes = []byte{0x91, 0xbc, 0x75, 0xc1, 0x8d, 0x3d, 0x85, 0xa7}

// VNCPasswdFile returns the path to a VNC password file containing the
// encrypted password "vnc". Creates the file on first call.
func VNCPasswdFile() string {
	p := filepath.Join(os.TempDir(), "devcell-vnc-passwd")
	if _, err := os.Stat(p); err != nil {
		os.WriteFile(p, vncPasswdBytes, 0600)
	}
	return p
}

// VNCUrl returns a VNC URL for macOS Screen Sharing.
func VNCUrl(port string) string {
	return "vnc://:vnc@127.0.0.1:" + port
}

// RoyalTSXVNCUrl returns a Royal TSX URI for a VNC connection.
func RoyalTSXVNCUrl(port string) string {
	return "rtsx://vnc://:vnc@127.0.0.1:" + port
}
