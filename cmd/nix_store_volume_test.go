package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNixStoreVolume_Mounted(t *testing.T) {
	for _, args := range [][]string{
		{"claude", "--dry-run"},
		{"shell", "--dry-run"},
	} {
		t.Run(args[0], func(t *testing.T) {
			home := scaffoldedHome(t)
			cmd := exec.Command(binaryPath, args...)
			cmd.Dir = home
			cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s failed: %v\noutput: %s", strings.Join(args, " "), err, out)
			}
			if !strings.Contains(string(out), "devcell-nix-store:/nix") {
				t.Errorf("nix store volume should be mounted:\n%s", out)
			}
		})
	}
}
