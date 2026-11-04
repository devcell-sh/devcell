package cfg_test

import (
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
)

func TestValidateStealthCrossArch_NativeX86OnX86(t *testing.T) {
	s := cfg.StealthSection{Arch: "x86"}
	if err := cfg.ValidateStealthCrossArch(s, "x86_64", "docker"); err != nil {
		t.Errorf("x86 on x86_64 should be valid: %v", err)
	}
}

func TestValidateStealthCrossArch_NativeArmOnArm(t *testing.T) {
	s := cfg.StealthSection{Arch: "arm"}
	if err := cfg.ValidateStealthCrossArch(s, "aarch64", "colima"); err != nil {
		t.Errorf("arm on aarch64 should be valid: %v", err)
	}
}

func TestValidateStealthCrossArch_X86OnArmWithColima(t *testing.T) {
	s := cfg.StealthSection{Arch: "x86"}
	if err := cfg.ValidateStealthCrossArch(s, "aarch64", "colima"); err != nil {
		t.Errorf("x86 on aarch64+colima should be valid (Rosetta): %v", err)
	}
}

func TestValidateStealthCrossArch_X86OnArmWithoutColima(t *testing.T) {
	s := cfg.StealthSection{Arch: "x86"}
	err := cfg.ValidateStealthCrossArch(s, "aarch64", "docker-desktop")
	if err == nil {
		t.Error("x86 on aarch64+docker-desktop should return error")
	}
}

func TestValidateStealthCrossArch_X86OnArmGenericDocker(t *testing.T) {
	s := cfg.StealthSection{Arch: "x86"}
	err := cfg.ValidateStealthCrossArch(s, "aarch64", "docker")
	if err == nil {
		t.Error("x86 on aarch64+generic docker should return error")
	}
}

func TestValidateStealthCrossArch_ArmOnX86(t *testing.T) {
	s := cfg.StealthSection{Arch: "arm"}
	err := cfg.ValidateStealthCrossArch(s, "x86_64", "docker")
	if err == nil {
		t.Error("arm on x86_64 should return error (not supported)")
	}
}

func TestValidateStealthCrossArch_DefaultArchOnArm(t *testing.T) {
	s := cfg.StealthSection{}
	_ = cfg.ValidateStealthCrossArch(s, "aarch64", "colima")
}

func TestValidateStealthCrossArch_ErrorMentionsColima(t *testing.T) {
	s := cfg.StealthSection{Arch: "x86"}
	err := cfg.ValidateStealthCrossArch(s, "aarch64", "docker-desktop")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"Rosetta", "Colima", "docker-desktop"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q; got: %s", want, msg)
		}
	}
}
