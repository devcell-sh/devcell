package ux_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/ux"
)

func TestFormatSecretsPhase_AllResolved(t *testing.T) {
	got, warn := ux.FormatSecretsPhase(7, 0)
	if warn {
		t.Error("zero failures must not warn")
	}
	want := "7 resolved"
	if plain(got) != want {
		t.Errorf("want %q, got %q", want, plain(got))
	}
}

func TestFormatSecretsPhase_WithFailures(t *testing.T) {
	got, warn := ux.FormatSecretsPhase(5, 2)
	if !warn {
		t.Error("partial failures must warn")
	}
	if !strings.Contains(got, "2 failed") {
		t.Errorf("detail must mention failed count; got %q", got)
	}
	if !strings.Contains(got, "5 resolved") {
		t.Errorf("detail must mention resolved count; got %q", got)
	}
}

func TestFormatSecretsPhase_NoLabelPrefix(t *testing.T) {
	for _, tc := range []struct{ count, failed int }{{7, 0}, {5, 2}} {
		got, _ := ux.FormatSecretsPhase(tc.count, tc.failed)
		for _, banned := range []string{"Loading", "Loaded", "secrets"} {
			if strings.Contains(plain(got), banned) {
				t.Errorf("detail must not include label fragment %q; got %q", banned, got)
			}
		}
	}
}

func TestFormatSecretsPhase_ZeroFailedOmitsClause(t *testing.T) {
	got, _ := ux.FormatSecretsPhase(3, 0)
	if strings.Contains(plain(got), "failed") {
		t.Errorf("zero failures must not show 'failed' clause; got %q", got)
	}
}

func TestFormatSecretsPhase_OmitsElapsed(t *testing.T) {
	got, _ := ux.FormatSecretsPhase(3, 0)
	for _, suffix := range []string{"ms", "µs", "ns"} {
		if strings.Contains(plain(got), suffix) {
			t.Errorf("must not embed elapsed time; got %q contains %q", got, suffix)
		}
	}
}

func TestFormatSecretsPhase_WarnShortTag(t *testing.T) {
	got, warn := ux.FormatSecretsPhase(5, 2)
	if !warn {
		t.Error("partial failures must warn")
	}
	tag, _, ok := strings.Cut(got, " — ")
	if !ok {
		t.Fatalf("detail must contain ' — ' separator for tag extraction; got %q", got)
	}
	if tag != "2 failed" {
		t.Errorf("tag = %q, want %q", tag, "2 failed")
	}
}
