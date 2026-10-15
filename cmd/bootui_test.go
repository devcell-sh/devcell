package main_test

import (
	"errors"
	"os/exec"
	"testing"
)

// CELL-657: `devcell boot-ui` is the popup renderer. Without a reachable
// event socket it exits 2 quickly instead of hanging.
func TestBootUI_UnreachableSocketExits2(t *testing.T) {
	cmd := exec.Command(binaryPath, "boot-ui", t.TempDir()+"/none.sock")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("boot-ui with no socket: err = %v, want exit code 2", err)
	}
}
