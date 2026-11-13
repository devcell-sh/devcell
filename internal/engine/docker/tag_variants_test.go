package docker_test

import (
	"os"
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// Tag-variant contract:
//
//   - `UserImageTag()` — returns the local devcell-user:<stack> tag.

func TestUserImageTag_BareTagUnchanged(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "")()
	defer setEnv("DEVCELL_WORKSPACE", "")()
	defer setStack("ultimate")()

	got := docker.UserImageTag()
	if got != "localhost/devcell-user:ultimate" {
		t.Errorf("UserImageTag = %q, want localhost/devcell-user:ultimate", got)
	}
}

func TestUserImageTag_EnvOverridesFullTag(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "my-registry/custom:tag")()
	if got := docker.UserImageTag(); got != "my-registry/custom:tag" {
		t.Errorf("env override ignored: got %q", got)
	}
}

// ── Custom build tag override (--image flag) ─────────────────────────────────

func TestResolveBuildTag_EmptyCustomReturnsDerived(t *testing.T) {
	if got := docker.ResolveBuildTag("", "localhost/devcell-user:ultimate"); got != "localhost/devcell-user:ultimate" {
		t.Errorf("empty custom: got %q, want derived", got)
	}
}

func TestResolveBuildTag_CustomOverrides(t *testing.T) {
	if got := docker.ResolveBuildTag("myorg/devcell-test:dev", "localhost/devcell-user:ultimate"); got != "myorg/devcell-test:dev" {
		t.Errorf("custom override ignored: got %q", got)
	}
}

func TestResolveBuildTag_CustomTrimsWhitespace(t *testing.T) {
	if got := docker.ResolveBuildTag("  myorg/foo:bar  ", "default"); got != "myorg/foo:bar" {
		t.Errorf("whitespace not trimmed: got %q", got)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func setEnv(k, v string) func() {
	prev := os.Getenv(k)
	if v == "" {
		os.Unsetenv(k)
	} else {
		os.Setenv(k, v)
	}
	return func() {
		if prev == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, prev)
		}
	}
}

func setStack(s string) func() {
	prev := docker.Stack
	docker.Stack = s
	return func() { docker.Stack = prev }
}
