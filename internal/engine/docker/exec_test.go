package docker_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

func TestBuildExecArgv_Basic(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-foo-0",
		Binary:        "zsh",
		TTY:           true,
	})
	want := []string{"docker", "exec", "-it", "devcell-foo-0", "zsh"}
	if len(argv) != len(want) {
		t.Fatalf("got %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestBuildExecArgv_WithArgs(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-foo-0",
		Binary:        "ls",
		Args:          []string{"-la", "/workspace"},
		TTY:           true,
	})
	want := []string{"docker", "exec", "-it", "devcell-foo-0", "ls", "-la", "/workspace"}
	if len(argv) != len(want) {
		t.Fatalf("got %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestBuildExecArgv_NoTTY(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-foo-0",
		Binary:        "zsh",
		TTY:           false,
	})
	for _, a := range argv {
		if a == "-it" {
			t.Error("-it should not be present when TTY is false")
		}
	}
	want := []string{"docker", "exec", "devcell-foo-0", "zsh"}
	if len(argv) != len(want) {
		t.Fatalf("got %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func TestBuildExecArgv_WithUser(t *testing.T) {
	argv := docker.BuildExecArgv(docker.ExecSpec{
		ContainerName: "devcell-foo-0",
		User:          "dmitry",
		Binary:        "claude",
		TTY:           true,
	})
	// Must contain -u dmitry before the container name
	found := false
	for i, a := range argv {
		if a == "-u" && i+1 < len(argv) && argv[i+1] == "dmitry" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("must contain -u dmitry, got %v", argv)
	}
}
