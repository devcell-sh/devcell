package cell

import (
	"strings"
	"testing"
)

func TestDetectTimezone_ReturnsNonEmpty(t *testing.T) {
	tz := DetectTimezone()
	if tz == "" {
		t.Fatal("DetectTimezone() returned empty string; should return at least \"UTC\"")
	}
}

func TestDetectTimezone_ValidIANA(t *testing.T) {
	tz := DetectTimezone()
	// IANA names contain a "/" (e.g. "America/New_York", "Europe/Prague"),
	// except for the fallback "UTC" which is also a valid IANA zone.
	if tz != "UTC" && !strings.Contains(tz, "/") {
		t.Errorf("DetectTimezone()=%q: expected IANA name containing \"/\" or \"UTC\"", tz)
	}
}

func TestDetectTimezone_FallbackUTC(t *testing.T) {
	// Verify the fallback path works. We can't easily unset /etc/localtime
	// in a test, but we can verify DetectTimezone never panics and returns
	// a non-empty result even on minimal systems.
	tz := DetectTimezone()
	if tz == "" {
		t.Fatal("DetectTimezone() must never return empty")
	}
	t.Logf("detected timezone: %s", tz)
}
