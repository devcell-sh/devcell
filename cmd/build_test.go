package main_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

// After the 2026-05-15 flip (CELL-189), UserImageTag() is unchanged
// (bare devcell-user:<stack>); the pure variant is reached via
// UserImageTagPure() with the -pure suffix.
func TestUserImageTag_BareLocal_PureSuffixOnVariant(t *testing.T) {
	saved := os.Getenv("DEVCELL_USER_IMAGE")
	defer os.Setenv("DEVCELL_USER_IMAGE", saved)
	os.Unsetenv("DEVCELL_USER_IMAGE")
	savedPure := os.Getenv("DEVCELL_USER_IMAGE_PURE")
	defer os.Setenv("DEVCELL_USER_IMAGE_PURE", savedPure)
	os.Unsetenv("DEVCELL_USER_IMAGE_PURE")

	savedStack := docker.Stack
	defer func() { docker.Stack = savedStack }()
	docker.Stack = "ultimate"

	bare := docker.UserImageTag()
	pure := docker.UserImageTagPure()

	if bare != "devcell-user:ultimate" {
		t.Errorf("UserImageTag = %q, want devcell-user:ultimate (bare, unchanged)", bare)
	}
	if pure != "devcell-user:ultimate-pure" {
		t.Errorf("UserImageTagPure = %q, want devcell-user:ultimate-pure", pure)
	}
	if pure != bare+"-pure" {
		t.Errorf("UserImageTagPure must equal UserImageTag + '-pure': pure=%q bare=%q", pure, bare)
	}
}

// PickImageTag — post-flip direction (CELL-183) + CELL-165 vocab:
//
//	false (default) → pure tag
//	true (--impure, alias --debian) → bare tag
func TestPickImageTag_FlippedDirection(t *testing.T) {
	saved := os.Getenv("DEVCELL_USER_IMAGE")
	defer os.Setenv("DEVCELL_USER_IMAGE", saved)
	os.Unsetenv("DEVCELL_USER_IMAGE")
	savedPure := os.Getenv("DEVCELL_USER_IMAGE_PURE")
	defer os.Setenv("DEVCELL_USER_IMAGE_PURE", savedPure)
	os.Unsetenv("DEVCELL_USER_IMAGE_PURE")

	savedStack := docker.Stack
	defer func() { docker.Stack = savedStack }()
	docker.Stack = "ultimate"

	pure := docker.PickImageTag(false)
	bare := docker.PickImageTag(true)

	if pure != "devcell-user:ultimate-pure" {
		t.Errorf("PickImageTag(false) = %q, want devcell-user:ultimate-pure (default)", pure)
	}
	if bare != "devcell-user:ultimate" {
		t.Errorf("PickImageTag(true) = %q, want devcell-user:ultimate (impure / --impure path)", bare)
	}
	if pure == bare {
		t.Errorf("pure and bare must differ: pure=%s bare=%s", pure, bare)
	}
}

func TestVet(t *testing.T) {
	cmd := exec.Command("go", "vet", "./...")
	cmd.Env = append(os.Environ(), "GOMODCACHE=/tmp/gomodcache", "GOPATH=/tmp/gopath")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet failed:\n%s", out)
	}
}

func TestBuildBinarySize(t *testing.T) {
	tmp, err := os.MkdirTemp("", "cell-dist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	binPath := tmp + "/cell"
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOMODCACHE=/tmp/gomodcache", "GOPATH=/tmp/gopath")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed:\n%s", out)
	}

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatal(err)
	}
	const maxSize = 50 * 1024 * 1024 // 50 MB
	if info.Size() > maxSize {
		t.Errorf("binary size %d exceeds 50MB limit", info.Size())
	}
}
