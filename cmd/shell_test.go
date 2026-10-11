package main_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

func TestShellExecArgv_DefaultZsh(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-myproject-0",
		Binary:        "zsh",
		TTY:           true,
	})
	want := []string{"docker", "exec", "-it", "devcell-myproject-0", "zsh"}
	if len(argv) != len(want) {
		t.Fatalf("got %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestShellExecArgv_CustomCommand(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-myproject-0",
		Binary:        "bash",
		Args:          []string{"-c", "echo hello"},
		TTY:           false,
	})
	want := []string{"docker", "exec", "devcell-myproject-0", "bash", "-c", "echo hello"}
	if len(argv) != len(want) {
		t.Fatalf("got %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}
