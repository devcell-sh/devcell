package tart

import (
	"path/filepath"
	"testing"
)

func TestNewCellSSHPaths_Layout(t *testing.T) {
	sp := NewCellSSHPaths("/Users/bob", "main")
	wantDir := filepath.Join("/Users/bob", ".devcell", "main", ".ssh")
	if sp.Dir != wantDir {
		t.Fatalf("Dir = %q, want %q", sp.Dir, wantDir)
	}
	if sp.PrivateKey != filepath.Join(wantDir, "id_ed25519") {
		t.Fatalf("PrivateKey = %q", sp.PrivateKey)
	}
	if sp.PublicKey != filepath.Join(wantDir, "id_ed25519.pub") {
		t.Fatalf("PublicKey = %q", sp.PublicKey)
	}
}

func TestCellHome(t *testing.T) {
	got := CellHome("/Users/bob", "main")
	want := filepath.Join("/Users/bob", ".devcell", "main")
	if got != want {
		t.Fatalf("CellHome = %q, want %q", got, want)
	}
}
