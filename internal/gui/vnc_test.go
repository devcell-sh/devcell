package gui_test

import (
	"os"
	"testing"

	"github.com/DimmKirr/devcell/internal/gui"
)

func TestVNCUrl(t *testing.T) {
	got := gui.VNCUrl("350")
	want := "vnc://:vnc@127.0.0.1:350"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestVNCPasswdFile(t *testing.T) {
	p := gui.VNCPasswdFile()
	if p == "" {
		t.Fatal("expected non-empty path")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read passwd file: %v", err)
	}
	if len(data) != 8 {
		t.Errorf("expected 8 bytes, got %d", len(data))
	}
}

func TestRoyalTSXVNCUrl(t *testing.T) {
	got := gui.RoyalTSXVNCUrl("350")
	want := "rtsx://vnc://:vnc@127.0.0.1:350"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
