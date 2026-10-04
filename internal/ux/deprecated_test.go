package ux_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/DimmKirr/devcell/internal/ux"
)

func TestDeprecated_RendersSubjectAndHint(t *testing.T) {
	var buf bytes.Buffer
	ux.Deprecated(&buf, "--qemu-ssh-port", "use --winkit-ssh-port instead")
	want := " ⚠      --qemu-ssh-port is deprecated (use --winkit-ssh-port instead)\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeprecatedLine_SplitsCanonicalMessage(t *testing.T) {
	var buf bytes.Buffer
	ux.DeprecatedLine(&buf, `engine "qemu" is deprecated and will be removed in a future release: use engine = "winkit" instead`)
	want := " ⚠      engine \"qemu\" is deprecated (use engine = \"winkit\" instead)\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeprecatedLine_PassesThroughNonCanonical(t *testing.T) {
	var buf bytes.Buffer
	ux.DeprecatedLine(&buf, "something odd")
	if got := buf.String(); got != " ⚠      something odd\n" {
		t.Fatalf("got %q", got)
	}
}

func TestDeprecatedAt_AppendsSource(t *testing.T) {
	var buf bytes.Buffer
	ux.DeprecatedAt(&buf, "[[volumes]]", "use [cell] volumes instead", "devcell.toml")
	want := " ⚠      [[volumes]] is deprecated (use [cell] volumes instead) (devcell.toml)\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWarn_UsesStandardWarningIcon(t *testing.T) {
	out := captureStdout(func() { ux.Warn("nix config drift") })
	if !strings.HasPrefix(out, "\r ⚠") || !strings.Contains(out, "nix config drift") {
		t.Fatalf("got %q, want the ⚠ prefix row", out)
	}
}

func TestSuccessRow_TitleAndElapsed(t *testing.T) {
	out := captureStdout(func() { ux.SuccessRow("GUI ready", 1624*time.Millisecond) })
	if !strings.HasPrefix(out, "\r ✓") || !strings.Contains(out, "GUI ready 1.624s") {
		t.Fatalf("got %q", out)
	}
}
