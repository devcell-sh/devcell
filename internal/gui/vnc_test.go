package gui_test

import (
	"os"
	"testing"

	"github.com/DimmKirr/devcell/internal/gui"
)

func TestVNCUrl_Legacy(t *testing.T) {
	got := gui.VNCUrl("127.0.0.1", "350", "", "vnc")
	want := "vnc://:vnc@127.0.0.1:350"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestVNCUrl_ARDAuth(t *testing.T) {
	got := gui.VNCUrl("192.168.64.5", "5900", "dmitry", "admin")
	want := "vnc://dmitry:admin@192.168.64.5:5900"
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

func TestRoyalTSXVNCUrl_Legacy(t *testing.T) {
	got := gui.RoyalTSXVNCUrl("127.0.0.1", "350", "", "vnc")
	want := "rtsx://vnc://:vnc@127.0.0.1:350"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestRoyalTSXVNCUrl_ARDAuth(t *testing.T) {
	got := gui.RoyalTSXVNCUrl("192.168.64.5", "5900", "dmitry", "admin")
	want := "rtsx://vnc://dmitry:admin@192.168.64.5:5900"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
