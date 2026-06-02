package runner_test

import (
	"os"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/runner"
)

// Tag-variant contract after the 2026-05-15 flip (DIMM-202) + DIMM-213
// vocabulary rename (`debian` → `impure`):
//
//   - `UserImageTag()` — unchanged. Returns the bare local devcell-user:<stack>.
//     After the flip this is reached only via `--impure` (legacy Dockerfile
//     path); the new default reaches UserImageTagPure() via PickImageTag(false).
//   - `UserImageTagPure()` — returns devcell-user:<stack>-pure. The default
//     after the flip.
//   - `StackImageTagImpure(stack)` — registry-side tag for the multi-arch
//     Dockerfile build. `-impure` suffix matches the variant axis name.
//     `StackImageTagDebian` is retained as a deprecated alias.
//   - `StackImageTagPure(stack)` — registry-side tag for the pure variant
//     (unchanged).
//   - `PickImageTag(impure)`:
//       false → UserImageTagPure()   (local pure, new default)
//       true  → UserImageTag()       (bare local tag, legacy --impure path)

func TestUserImageTag_BareTagUnchanged(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "")()
	defer setStack("ultimate")()

	got := runner.UserImageTag()
	if got != "devcell-user:ultimate" {
		t.Errorf("UserImageTag = %q, want devcell-user:ultimate (unchanged across flip)", got)
	}
}

func TestUserImageTagPure_HasPureSuffix(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "")()
	defer setEnv("DEVCELL_USER_IMAGE_PURE", "")()
	defer setStack("ultimate")()

	got := runner.UserImageTagPure()
	if got != "devcell-user:ultimate-pure" {
		t.Errorf("UserImageTagPure = %q, want devcell-user:ultimate-pure", got)
	}
}

func TestStackImageTagImpure_RegistryImpureSuffix(t *testing.T) {
	got := runner.StackImageTagImpure("ultimate")
	if !strings.HasSuffix(got, "-ultimate-impure") {
		t.Errorf("impure stack tag = %q, want suffix -ultimate-impure", got)
	}
}

// Deprecated alias retained for one release — same return value as the
// canonical StackImageTagImpure.
func TestStackImageTagDebian_DeprecatedAliasReturnsImpureSuffix(t *testing.T) {
	got := runner.StackImageTagDebian("ultimate")
	if !strings.HasSuffix(got, "-ultimate-impure") {
		t.Errorf("debian alias tag = %q, want suffix -ultimate-impure (alias forwards to Impure)", got)
	}
}

func TestStackImageTagPure_RegistryPureSuffix(t *testing.T) {
	got := runner.StackImageTagPure("ultimate")
	if !strings.HasSuffix(got, "-ultimate-pure") {
		t.Errorf("pure stack tag = %q, want suffix -ultimate-pure", got)
	}
}

// PickImageTag — post DIMM-204 flip + DIMM-213 vocab rename: parameter
// renamed to `impure`. false (default) returns the pure tag, true returns
// the bare tag.
//
//	false (default)   → UserImageTagPure()
//	true  (--impure)  → UserImageTag()
func TestPickImageTag_DefaultIsPureTag(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "")()
	defer setEnv("DEVCELL_USER_IMAGE_PURE", "")()
	defer setStack("ultimate")()

	got := runner.PickImageTag(false)
	if got != "devcell-user:ultimate-pure" {
		t.Errorf("PickImageTag(false) = %q, want devcell-user:ultimate-pure (default after flip)", got)
	}
}

func TestPickImageTag_ImpureTrueReturnsBareTag(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "")()
	defer setStack("ultimate")()

	got := runner.PickImageTag(true)
	if got != "devcell-user:ultimate" {
		t.Errorf("PickImageTag(true) = %q, want devcell-user:ultimate (legacy bare tag, --impure path)", got)
	}
}

// Env overrides still apply per their respective tag.
func TestUserImageTag_EnvOverridesFullTag(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE", "my-registry/custom:tag")()
	if got := runner.UserImageTag(); got != "my-registry/custom:tag" {
		t.Errorf("env override ignored: got %q", got)
	}
}

func TestUserImageTagPure_EnvOverridesFullTag(t *testing.T) {
	defer setEnv("DEVCELL_USER_IMAGE_PURE", "my-registry/custom:tag")()
	if got := runner.UserImageTagPure(); got != "my-registry/custom:tag" {
		t.Errorf("env override ignored: got %q", got)
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
	prev := runner.Stack
	runner.Stack = s
	return func() { runner.Stack = prev }
}
